const test = require('node:test');
const assert = require('node:assert/strict');
const { TextDocument } = require('../node_modules/vscode-languageserver-textdocument');
const { DiagnosticSeverity } = require('../node_modules/vscode-languageserver/node');
const { provideCodeActions } = require('../dist/codeActions.js');

function inlineDocument(text) {
  return TextDocument.create('file:///code-actions.gsx', 'gsx', 1, text);
}

test('code actions add missing img alt text and button type', () => {
  const document = inlineDocument('<img src={user.AvatarURL} />\n<button>Save</button>\n');
  const actions = provideCodeActions(document, [
    {
      code: 'L001',
      message: 'img element should include alt text',
      severity: DiagnosticSeverity.Warning,
      range: {
        start: { line: 0, character: 0 },
        end: { line: 0, character: 1 }
      }
    },
    {
      code: 'L002',
      message: 'button element should include type',
      severity: DiagnosticSeverity.Warning,
      range: {
        start: { line: 1, character: 0 },
        end: { line: 1, character: 1 }
      }
    }
  ]);

  assert.equal(actions.length, 2);
  assert.equal(actions[0].title, 'Add alt="" to <img>');
  assert.equal(actions[0].edit.changes[document.uri][0].newText, ' alt=""');
  assert.equal(actions[1].title, 'Add type="button" to <button>');
  assert.equal(actions[1].edit.changes[document.uri][0].newText, ' type="button"');
});

test('code actions can offer a format fix and aria boolean fix', () => {
  const document = inlineDocument('<div><button aria-hidden>Save</button></div>\n');
  const actions = provideCodeActions(
    document,
    [
      {
        code: 'L007',
        message: 'file is not formatted',
        severity: DiagnosticSeverity.Warning,
        range: {
          start: { line: 0, character: 0 },
          end: { line: 0, character: 1 }
        }
      },
      {
        code: 'L008',
        message: 'ARIA attribute "aria-hidden" should have an explicit value',
        severity: DiagnosticSeverity.Warning,
        range: {
          start: { line: 0, character: 13 },
          end: { line: 0, character: 24 }
        }
      }
    ],
    '<div>\n  <button aria-hidden="true">Save</button>\n</div>\n'
  );

  assert.equal(actions.length, 2);
  assert.equal(actions[0].title, 'Format GSX file');
  assert.equal(actions[0].kind, 'source.fixAll');
  assert.equal(actions[1].title, 'Set ARIA attribute to "true"');
  assert.equal(actions[1].edit.changes[document.uri][0].newText, '="true"');
});

test('code actions can add or extend rel on target blank anchors', () => {
  const document = inlineDocument('<a href="/docs" target="_blank">Docs</a>\n<a href="/docs" target="_blank" rel="nofollow">Docs</a>\n');
  const actions = provideCodeActions(document, [
    {
      code: 'L009',
      message: 'anchor with target="_blank" should include rel="noopener noreferrer"',
      severity: DiagnosticSeverity.Warning,
      range: {
        start: { line: 0, character: 0 },
        end: { line: 0, character: 1 }
      }
    },
    {
      code: 'L009',
      message: 'anchor with target="_blank" should include rel="noopener noreferrer"',
      severity: DiagnosticSeverity.Warning,
      range: {
        start: { line: 1, character: 0 },
        end: { line: 1, character: 1 }
      }
    }
  ]);

  assert.equal(actions.length, 2);
  assert.equal(actions[0].title, 'Ensure rel="noopener noreferrer" on <a>');
  assert.equal(actions[0].edit.changes[document.uri][0].newText, ' rel="noopener noreferrer"');
  assert.equal(actions[1].edit.changes[document.uri][0].newText, 'nofollow noopener noreferrer');
});
