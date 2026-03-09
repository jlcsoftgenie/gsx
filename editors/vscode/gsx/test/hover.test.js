const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { TextDocument } = require('../node_modules/vscode-languageserver-textdocument');
const { provideHover } = require('../dist/gsxFeatures.js');

function documentFor(relativePath) {
  const filePath = path.resolve(__dirname, '..', '..', '..', '..', relativePath);
  const text = fs.readFileSync(filePath, 'utf8');
  return TextDocument.create(pathToFileURL(filePath).toString(), 'gsx', 1, text);
}

async function hoverValue(relativePath, needle, occurrence, offsetInNeedle) {
  const document = documentFor(relativePath);
  const text = document.getText();
  let from = 0;
  let index = -1;
  for (let count = 0; count < occurrence; count++) {
    index = text.indexOf(needle, from);
    assert.notEqual(index, -1, `did not find ${needle} in ${relativePath}`);
    from = index + needle.length;
  }
  const position = document.positionAt(index + offsetInNeedle);
  const hover = await provideHover(document, position);
  assert.ok(hover, `expected hover for ${needle} in ${relativePath}`);
  return hover.contents.value;
}

test('hover shows component parameter types and local struct details', async () => {
  const paramHover = await hoverValue('examples/basic/pages.gsx', 'title string', 1, 'title'.indexOf('t'));
  assert.match(paramHover, /\*\*title\*\*/);
  assert.match(paramHover, /Type: `string`/);

  const typeHover = await hoverValue('examples/basic/pages.gsx', 'user User', 1, 'user '.length);
  assert.match(typeHover, /\*\*type User struct\*\*/);
  assert.match(typeHover, /`AvatarURL`: `string`/);
  assert.match(typeHover, /`Email`: `string`/);
});

test('hover shows component prop types, loop variables, and selector field types', async () => {
  const propHover = await hoverValue('examples/basic/pages.gsx', 'title={title}', 1, 0);
  assert.match(propHover, /Parameter of component `AppLayout`\./);
  assert.match(propHover, /Type: `string`/);

  const loopHover = await hoverValue('examples/admin/pages.gsx', 'for _, metric := range data.Metrics', 1, 'for _, '.length);
  assert.match(loopHover, /\*\*metric\*\*/);
  assert.match(loopHover, /Type: `Metric`/);

  const fieldHover = await hoverValue('examples/admin/pages.gsx', 'metric.Value', 1, 'metric.'.length);
  assert.match(fieldHover, /\*\*Value\*\*/);
  assert.match(fieldHover, /Field of `Metric`\./);
});
