const { after, test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { pathToFileURL } = require('node:url');
const { TextDocument } = require('../node_modules/vscode-languageserver-textdocument');
const {
  buildEmbeddedGoProbe,
  provideDefinition,
  provideEmbeddedGoDiagnostics,
  provideCompletionItems,
  provideHover,
  provideReferences,
  prepareComponentRename,
  provideRenameEdits
} = require('../dist/gsxFeatures.js');
const { shutdownGoplsClients } = require('../dist/goplsClient.js');

after(async () => {
  await shutdownGoplsClients();
});

function documentFor(relativePath) {
  const filePath = path.resolve(__dirname, '..', '..', '..', '..', relativePath);
  const text = fs.readFileSync(filePath, 'utf8');
  return TextDocument.create(pathToFileURL(filePath).toString(), 'gsx', 1, text);
}

function inlineDocument(relativePath, text) {
  const filePath = path.resolve(__dirname, '..', '..', '..', '..', relativePath);
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

function requireGopls(t) {
  const goplsCommand = findGoplsCommand();
  if (goplsCommand !== undefined) {
    return goplsCommand;
  }
  t.skip('gopls not available');
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

test('embedded Go probe includes inferred local declaration bindings', async () => {
  const document = inlineDocument('examples/basic/__embedded_local_decls.gsx', `package main

component Page(users []User) {
  count := len(users)
  const emptyLabel = "No users"
  var first *User

  <section>
    <p>{first.Name}</p>
    <p>{count}</p>
    <p>{emptyLabel}</p>
  </section>
}
`);
  const text = document.getText();
  const position = document.positionAt(offsetFor(text, '{first.Name}', 1, '{first.'.length));
  const probe = await buildEmbeddedGoProbe(document, position);
  assert.ok(probe);
  assert.match(probe.text, /var count int/);
  assert.match(probe.text, /var emptyLabel string/);
  assert.match(probe.text, /var first \*User/);
  assert.match(probe.text, /_ = first.Name/);
});

test('gopls-backed hover and completion work inside single-line GSX expressions when gopls is available', async (t) => {
  const goplsCommand = requireGopls(t);
  if (goplsCommand === undefined) {
    return;
  }

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

test('embedded Go definition resolves local declarations back to the GSX file when gopls is available', async (t) => {
  const goplsCommand = requireGopls(t);
  if (goplsCommand === undefined) {
    return;
  }

  const document = inlineDocument('examples/basic/__embedded_definition.gsx', `package main

component Page(users []User) {
  count := len(users)
  <p>{count}</p>
}
`);
  const text = document.getText();
  const position = document.positionAt(offsetFor(text, '{count}', 1, '{co'.length));
  const links = await provideDefinition(document, position, { goplsCommand });
  assert.ok(Array.isArray(links));
  assert.equal(links[0].targetUri, document.uri);
  assert.equal(links[0].targetSelectionRange.start.line, 3);
});

test('embedded Go diagnostics map back to GSX lines when gopls is available', async (t) => {
  const goplsCommand = requireGopls(t);
  if (goplsCommand === undefined) {
    return;
  }

  const document = inlineDocument('examples/basic/__embedded_diagnostics.gsx', `package main

component Page(users []User) {
  count := len(users)
  <p>{missingName}</p>
}
`);
  const diagnostics = await provideEmbeddedGoDiagnostics(document, { goplsCommand });
  assert.ok(diagnostics.length > 0);
  assert.ok(diagnostics.some((diagnostic) => diagnostic.range.start.line === 4));
  assert.ok(diagnostics.some((diagnostic) => /undefined|undeclared/i.test(diagnostic.message)));
});

test('local declaration references and rename work through the embedded Go overlay when gopls is available', async (t) => {
  const goplsCommand = requireGopls(t);
  if (goplsCommand === undefined) {
    return;
  }

  const document = inlineDocument('examples/basic/__embedded_refs.gsx', `package main

component Page(users []User) {
  count := len(users)
  <p>{count}</p>
  if count > 0 {
    <span>{count}</span>
  }
}
`);
  const text = document.getText();
  const position = document.positionAt(offsetFor(text, '{count}', 1, '{co'.length));

  const refs = await provideReferences(document, position, true, { goplsCommand });
  assert.ok(refs);
  assert.ok(refs.length >= 3);
  assert.ok(refs.every((ref) => ref.uri === document.uri));

  const prepare = await prepareComponentRename(document, position, { goplsCommand });
  assert.ok(prepare);
  assert.equal(prepare.start.line, 4);

  const edit = await provideRenameEdits(document, position, 'totalCount', { goplsCommand });
  assert.ok(edit);
  const fileEdits = edit.changes[document.uri];
  assert.ok(fileEdits);
  assert.ok(fileEdits.length >= 3);
  assert.ok(fileEdits.every((item) => item.newText === 'totalCount'));
});
