const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { TextDocument } = require('../node_modules/vscode-languageserver-textdocument');
const {
  prepareComponentRename,
  provideDefinition,
  provideDocumentSymbols,
  provideReferences,
  provideRenameEdits,
  provideSignatureHelp
} = require('../dist/gsxFeatures.js');

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

test('document symbols list GSX components in file order', () => {
  const document = documentFor('examples/admin/pages.gsx');
  const symbols = provideDocumentSymbols(document);
  assert.deepEqual(symbols.map((symbol) => symbol.name), ['MetricCard', 'DashboardLayout', 'DashboardPage']);
  assert.equal(symbols[0].detail, 'MetricCard(metric Metric)');
  assert.equal(symbols[1].selectionRange.start.line, 9);
  assert.deepEqual(symbols[1].children.map((child) => child.name), ['title', 'slot:head', 'slot']);
});

test('go to definition resolves local and imported component tags', async () => {
  const localDocument = documentFor('examples/basic/pages.gsx');
  const localText = localDocument.getText();
  const localOffset = offsetFor(localText, '<UserCard user={user} />', 1, 2);
  const localLinks = await provideDefinition(localDocument, localDocument.positionAt(localOffset));
  assert.ok(localLinks);
  assert.equal(localLinks[0].targetUri, localDocument.uri);
  assert.equal(localLinks[0].targetSelectionRange.start.line, 16);

  const importedDocument = documentFor('examples/crosspkg/pages.gsx');
  const importedText = importedDocument.getText();
  const importedOffset = offsetFor(importedText, '<shared.Panel title={title}>', 1, '<shared.'.length);
  const importedLinks = await provideDefinition(importedDocument, importedDocument.positionAt(importedOffset));
  assert.ok(importedLinks);
  assert.match(importedLinks[0].targetUri, /examples\/shared\/layouts\/layouts\.gsx$/);
  assert.equal(importedLinks[0].targetSelectionRange.start.line, 22);
});

test('signature help exposes component props and active parameter', async () => {
  const document = documentFor('examples/basic/pages.gsx');
  const text = document.getText();

  const titleOffset = offsetFor(text, 'title={title}', 1, 'title={'.length);
  const titleHelp = await provideSignatureHelp(document, document.positionAt(titleOffset));
  assert.ok(titleHelp);
  assert.equal(titleHelp.signatures[0].label, 'AppLayout(title string)');
  assert.equal(titleHelp.activeParameter, 0);

  const userOffset = offsetFor(text, '<UserCard user={user} />', 1, '<UserCard user={'.length);
  const userHelp = await provideSignatureHelp(document, document.positionAt(userOffset));
  assert.ok(userHelp);
  assert.equal(userHelp.signatures[0].label, 'UserCard(user User)');
  assert.equal(userHelp.activeParameter, 0);
  assert.equal(userHelp.signatures[0].parameters[0].label, 'user User');
});

test('find references returns declarations and usages across the module', async () => {
  const localDocument = documentFor('examples/basic/pages.gsx');
  const localText = localDocument.getText();
  const localOffset = offsetFor(localText, '<UserCard user={user} />', 1, '<User'.length);
  const localRefs = await provideReferences(localDocument, localDocument.positionAt(localOffset), true);
  assert.ok(localRefs);
  assert.equal(localRefs.length, 2);
  assert.equal(localRefs.filter((ref) => ref.uri === localDocument.uri).length, 2);

  const importedDocument = documentFor('examples/crosspkg/pages.gsx');
  const importedText = importedDocument.getText();
  const importedOffset = offsetFor(importedText, '<shared.Panel title={title}>', 1, '<shared.Panel'.length);
  const importedRefs = await provideReferences(importedDocument, importedDocument.positionAt(importedOffset), true);
  assert.ok(importedRefs);
  assert.ok(importedRefs.length >= 3);
  const importedUris = importedRefs.map((ref) => ref.uri).join('\n');
  assert.match(importedUris, /examples\/crosspkg\/pages\.gsx/);
  assert.match(importedUris, /examples\/shared\/layouts\/layouts\.gsx/);
  assert.match(importedUris, /examples\/webserver\/pages\.gsx/);
});

test('rename prepares a component range and rewrites declarations and usages', async () => {
  const document = documentFor('examples/basic/pages.gsx');
  const text = document.getText();
  const offset = offsetFor(text, '<UserCard user={user} />', 1, '<User'.length);
  const position = document.positionAt(offset);

  const prepare = await prepareComponentRename(document, position);
  assert.ok(prepare);
  assert.equal(prepare.start.line, 37);

  const edit = await provideRenameEdits(document, position, 'PersonCard');
  assert.ok(edit);
  const fileEdits = edit.changes[document.uri];
  assert.equal(fileEdits.length, 2);
  assert.equal(fileEdits[0].newText, 'PersonCard');
  assert.equal(fileEdits[1].newText, 'PersonCard');

  const invalid = await provideRenameEdits(document, position, 'personCard');
  assert.equal(invalid, null);
});

test('rename rewrites imported component declarations and cross-package usages', async () => {
  const document = documentFor('examples/crosspkg/pages.gsx');
  const text = document.getText();
  const position = document.positionAt(offsetFor(text, '<shared.Panel title={title}>', 1, '<shared.Panel'.length));
  const edit = await provideRenameEdits(document, position, 'Surface');
  assert.ok(edit);
  assert.ok(Object.keys(edit.changes).length >= 3);
  const allEdits = Object.entries(edit.changes).flatMap(([uri, edits]) => edits.map((entry) => ({ uri, entry })));
  assert.ok(allEdits.some(({ uri }) => /examples\/shared\/layouts\/layouts\.gsx$/.test(uri)));
  assert.ok(allEdits.some(({ uri }) => /examples\/crosspkg\/pages\.gsx$/.test(uri)));
  assert.ok(allEdits.some(({ uri }) => /examples\/webserver\/pages\.gsx$/.test(uri)));
  assert.ok(allEdits.every(({ entry }) => entry.newText === 'Surface'));
});

test('go to definition resolves local and imported slot names', async () => {
  const localDocument = documentFor('examples/layouts/pages.gsx');
  const localText = localDocument.getText();
  const localOffset = offsetFor(localText, 'slot="head"', 1, 'slot="'.length);
  const localLinks = await provideDefinition(localDocument, localDocument.positionAt(localOffset));
  assert.ok(localLinks);
  assert.equal(localLinks.length, 1);
  assert.equal(localLinks[0].targetUri, localDocument.uri);
  assert.equal(localLinks[0].targetSelectionRange.start.line, 26);

  const importedDocument = documentFor('examples/crosspkg/pages.gsx');
  const importedText = importedDocument.getText();
  const importedOffset = offsetFor(importedText, 'slot="head"', 1, 'slot="'.length);
  const importedLinks = await provideDefinition(importedDocument, importedDocument.positionAt(importedOffset));
  assert.ok(importedLinks);
  assert.equal(importedLinks.length, 1);
  assert.match(importedLinks[0].targetUri, /examples\/shared\/layouts\/layouts\.gsx$/);
  assert.equal(importedLinks[0].targetSelectionRange.start.line, 24);
});

test('find references returns slot declarations and usages scoped to the owning component', async () => {
  const localDocument = documentFor('examples/layouts/pages.gsx');
  const localText = localDocument.getText();
  const localOffset = offsetFor(localText, 'slot="head"', 1, 'slot="'.length);
  const localRefs = await provideReferences(localDocument, localDocument.positionAt(localOffset), true);
  assert.ok(localRefs);
  assert.equal(localRefs.length, 2);
  assert.ok(localRefs.every((ref) => ref.uri === localDocument.uri));

  const importedDocument = documentFor('examples/crosspkg/pages.gsx');
  const importedText = importedDocument.getText();
  const importedOffset = offsetFor(importedText, 'slot="head"', 1, 'slot="'.length);
  const importedRefs = await provideReferences(importedDocument, importedDocument.positionAt(importedOffset), true);
  assert.ok(importedRefs);
  assert.equal(importedRefs.length, 4);
  const importedUris = importedRefs.map((ref) => ref.uri).join('\n');
  assert.match(importedUris, /examples\/shared\/layouts\/layouts\.gsx/);
  assert.match(importedUris, /examples\/crosspkg\/pages\.gsx/);
  assert.match(importedUris, /examples\/webserver\/pages\.gsx/);
});

test('rename rewrites local and imported slot declarations and usages', async () => {
  const localDocument = documentFor('examples/layouts/pages.gsx');
  const localText = localDocument.getText();
  const localPosition = localDocument.positionAt(offsetFor(localText, 'slot="head"', 1, 'slot="'.length));
  const localPrepare = await prepareComponentRename(localDocument, localPosition);
  assert.ok(localPrepare);
  const localEdit = await provideRenameEdits(localDocument, localPosition, 'meta');
  assert.ok(localEdit);
  assert.equal(localEdit.changes[localDocument.uri].length, 2);
  assert.ok(localEdit.changes[localDocument.uri].every((entry) => entry.newText === 'meta'));

  const importedDocument = documentFor('examples/crosspkg/pages.gsx');
  const importedText = importedDocument.getText();
  const importedPosition = importedDocument.positionAt(offsetFor(importedText, 'slot="head"', 1, 'slot="'.length));
  const importedEdit = await provideRenameEdits(importedDocument, importedPosition, 'meta');
  assert.ok(importedEdit);
  assert.equal(Object.keys(importedEdit.changes).length, 3);
  const importedChanges = Object.entries(importedEdit.changes).flatMap(([uri, edits]) => edits.map((entry) => ({ uri, entry })));
  assert.ok(importedChanges.some(({ uri }) => /examples\/shared\/layouts\/layouts\.gsx$/.test(uri)));
  assert.ok(importedChanges.some(({ uri }) => /examples\/crosspkg\/pages\.gsx$/.test(uri)));
  assert.ok(importedChanges.some(({ uri }) => /examples\/webserver\/pages\.gsx$/.test(uri)));
  assert.ok(importedChanges.every(({ entry }) => entry.newText === 'meta'));

  const invalid = await provideRenameEdits(importedDocument, importedPosition, 'bad name');
  assert.equal(invalid, null);
});
