const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { pathToFileURL } = require('node:url');
const { TextDocument } = require('../node_modules/vscode-languageserver-textdocument');
const {
  buildEmbeddedGoProbe,
  provideCompletionItems,
  provideHover
} = require('../dist/gsxFeatures.js');
const { shutdownGoplsClients } = require('../dist/goplsClient.js');

function documentFor(relativePath) {
  const filePath = path.resolve(__dirname, '..', '..', '..', '..', relativePath);
  const text = fs.readFileSync(filePath, 'utf8');
  return TextDocument.create(pathToFileURL(filePath).toString(), 'gsx', 1, text);
}

function offsetFor(text, needle, occurrence, offsetInNeedle) {
  let from = 0;
  let index = -1;
  for (let count = 0; count < occurrence; count++) {
    index = text.indexOf(needle, from);
    assert.notEqual(index, -1, `did not find ${needle}`);
    from = index + needle.length;
  }
  return index + offsetInNeedle;
}

function findGoplsCommand() {
  const candidates = [
    process.env.GSX_GOPLS_COMMAND,
    'gopls',
    `${process.env.HOME}/.local/bin/gopls`,
    `${process.env.HOME}/go/bin/gopls`
  ].filter(Boolean);
  for (const candidate of candidates) {
    const result = spawnSync(candidate, ['version'], { stdio: 'ignore' });
    if (result.status === 0) {
      return candidate;
    }
  }
  return undefined;
}

test('embedded Go probe includes params and inferred loop bindings', async () => {
  const document = documentFor('examples/admin/pages.gsx');
  const text = document.getText();
  const position = document.positionAt(offsetFor(text, 'metric={metric}', 1, 'metric={'.length));
  const probe = await buildEmbeddedGoProbe(document, position);
  assert.ok(probe);
  assert.match(probe.text, /^package main/m);
  assert.match(probe.text, /func __gsxProbe\(data DashboardData\)/);
  assert.match(probe.text, /var metric Metric/);
  assert.match(probe.text, /_ = metric/);
});

test('gopls-backed hover and completion work inside single-line GSX expressions when gopls is available', async (t) => {
  const goplsCommand = findGoplsCommand();
  if (goplsCommand === undefined) {
    t.skip('gopls not available');
  }
  t.after(async () => {
    await shutdownGoplsClients();
  });

  const document = documentFor('examples/admin/pages.gsx');
  const text = document.getText();

  const hoverPosition = document.positionAt(offsetFor(text, '{data.Title}', 1, '{data.'.length));
  const hover = await provideHover(document, hoverPosition, { goplsCommand });
  assert.ok(hover);

  const completionPosition = document.positionAt(offsetFor(text, '{data.Title}', 1, '{data.'.length));
  const items = await provideCompletionItems(document, completionPosition, { goplsCommand });
  assert.ok(items);
  assert.ok(items.some((item) => item.label === 'Title'));
});
