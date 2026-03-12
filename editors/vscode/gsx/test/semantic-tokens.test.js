const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { TextDocument } = require('../node_modules/vscode-languageserver-textdocument');
const { provideSemanticTokens, semanticTokenLegend } = require('../dist/semanticTokens.js');

function inlineDocument(relativePath, text) {
  const filePath = path.resolve(__dirname, '..', '..', '..', '..', relativePath);
  return TextDocument.create(pathToFileURL(filePath).toString(), 'gsx', 1, text);
}

function decodeTokens(document, semanticTokens) {
  const out = [];
  const data = semanticTokens.data;
  let line = 0;
  let character = 0;
  for (let index = 0; index < data.length; index += 5) {
    line += data[index];
    character = data[index] === 0 ? character + data[index + 1] : data[index + 1];
    const length = data[index + 2];
    const tokenType = semanticTokenLegend.tokenTypes[data[index + 3]];
    const tokenModifiers = semanticTokenLegend.tokenModifiers.filter((_, modifierIndex) =>
      (data[index + 4] & (1 << modifierIndex)) !== 0
    );
    const start = document.offsetAt({ line, character });
    const end = document.offsetAt({ line, character: character + length });
    out.push({
      line,
      character,
      length,
      text: document.getText().slice(start, end),
      tokenType,
      tokenModifiers
    });
  }
  return out;
}

test('semantic tokens highlight local declaration names and types', () => {
  const document = inlineDocument('examples/basic/__semantic_local_decls.gsx', `package main

component Page(users []User) {
  count := len(users)
  const emptyLabel = "No users"
  var first *User
}
`);
  const tokens = decodeTokens(document, provideSemanticTokens(document));

  const count = tokens.find((token) => token.text === 'count' && token.tokenType === 'variable' && token.tokenModifiers.includes('declaration'));
  assert.ok(count, 'expected declaration token for count');

  const emptyLabel = tokens.find((token) => token.text === 'emptyLabel' && token.tokenType === 'variable' && token.tokenModifiers.includes('declaration'));
  assert.ok(emptyLabel, 'expected declaration token for emptyLabel');

  const first = tokens.find((token) => token.text === 'first' && token.tokenType === 'variable' && token.tokenModifiers.includes('declaration'));
  assert.ok(first, 'expected declaration token for first');

  const userType = tokens.find((token) => token.text === 'User' && token.tokenType === 'type');
  assert.ok(userType, 'expected type token for User');
});
