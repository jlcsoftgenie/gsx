import {
  CodeAction,
  CodeActionKind,
  Diagnostic,
  Range,
  TextEdit
} from 'vscode-languageserver/node';
import { TextDocument } from 'vscode-languageserver-textdocument';

export function provideCodeActions(
  document: TextDocument,
  diagnostics: readonly Diagnostic[],
  formattedText?: string
): CodeAction[] {
  const actions: CodeAction[] = [];
  for (const diagnostic of diagnostics) {
    const code = typeof diagnostic.code === 'string' ? diagnostic.code : '';
    switch (code) {
      case 'L009': {
        const edit = ensureAnchorRelNoopener(document, diagnostic.range.start);
        if (edit !== undefined) {
          actions.push(quickFix(document, diagnostic, 'Ensure rel="noopener noreferrer" on <a>', edit));
        }
        break;
      }
      case 'L001': {
        const edit = insertAttributeIntoTag(document, diagnostic.range.start, 'img', 'alt', ' alt=""');
        if (edit !== undefined) {
          actions.push(quickFix(document, diagnostic, 'Add alt="" to <img>', edit));
        }
        break;
      }
      case 'L002': {
        const edit = insertAttributeIntoTag(document, diagnostic.range.start, 'button', 'type', ' type="button"');
        if (edit !== undefined) {
          actions.push(quickFix(document, diagnostic, 'Add type="button" to <button>', edit));
        }
        break;
      }
      case 'L007': {
        if (formattedText !== undefined && formattedText !== document.getText()) {
          actions.push({
            title: 'Format GSX file',
            kind: CodeActionKind.SourceFixAll,
            diagnostics: [diagnostic],
            edit: {
              changes: {
                [document.uri]: [TextEdit.replace(fullDocumentRange(document), formattedText)]
              }
            },
            isPreferred: true
          });
        }
        break;
      }
      case 'L008': {
        const edit = addExplicitBooleanAttributeValue(document, diagnostic.range.start);
        if (edit !== undefined) {
          actions.push(quickFix(document, diagnostic, 'Set ARIA attribute to "true"', edit));
        }
        break;
      }
      default:
        break;
    }
  }
  return actions;
}

function quickFix(document: TextDocument, diagnostic: Diagnostic, title: string, edit: TextEdit): CodeAction {
  return {
    title,
    kind: CodeActionKind.QuickFix,
    diagnostics: [diagnostic],
    edit: {
      changes: {
        [document.uri]: [edit]
      }
    },
    isPreferred: true
  };
}

function insertAttributeIntoTag(
  document: TextDocument,
  start: Range['start'],
  tagName: string,
  attributeName: string,
  insertion: string
): TextEdit | undefined {
  const text = document.getText();
  const tagStart = elementStartOffset(text, document.offsetAt(start));
  if (tagStart === undefined) {
    return undefined;
  }
  if (!text.slice(tagStart).startsWith(`<${tagName}`)) {
    return undefined;
  }
  const tagEnd = findTagCloseOffset(text, tagStart);
  if (tagEnd === undefined) {
    return undefined;
  }
  const tagText = text.slice(tagStart, tagEnd + 1);
  if (new RegExp(`\\b${attributeName}\\b`).test(tagText)) {
    return undefined;
  }
  const insertOffset = insertionOffsetBeforeTagClose(text, tagEnd);
  return TextEdit.insert(document.positionAt(insertOffset), insertion);
}

function addExplicitBooleanAttributeValue(
  document: TextDocument,
  start: Range['start']
): TextEdit | undefined {
  const text = document.getText();
  const offset = document.offsetAt(start);
  let end = offset;
  while (end < text.length && /[A-Za-z0-9:._-]/.test(text[end])) {
    end++;
  }
  if (end === offset) {
    return undefined;
  }
  if (text.slice(offset, end).startsWith('aria-') === false) {
    return undefined;
  }
  let probe = end;
  while (probe < text.length && /\s/.test(text[probe])) {
    probe++;
  }
  if (text[probe] === '=') {
    return undefined;
  }
  return TextEdit.insert(document.positionAt(end), '="true"');
}

function ensureAnchorRelNoopener(
  document: TextDocument,
  start: Range['start']
): TextEdit | undefined {
  const text = document.getText();
  const tagStart = elementStartOffset(text, document.offsetAt(start));
  if (tagStart === undefined) {
    return undefined;
  }
  if (!text.slice(tagStart).startsWith('<a')) {
    return undefined;
  }
  const tagEnd = findTagCloseOffset(text, tagStart);
  if (tagEnd === undefined) {
    return undefined;
  }
  const tagText = text.slice(tagStart, tagEnd + 1);
  const relMatch = /\brel\s*=\s*(?:"([^"]*)"|'([^']*)')/.exec(tagText);
  if (relMatch === null) {
    const insertOffset = insertionOffsetBeforeTagClose(text, tagEnd);
    return TextEdit.insert(document.positionAt(insertOffset), ' rel="noopener noreferrer"');
  }
  const currentValue = relMatch[1] ?? relMatch[2] ?? '';
  const tokens = currentValue.split(/\s+/).filter(Boolean);
  if (!tokens.includes('noopener')) {
    tokens.push('noopener');
  }
  if (!tokens.includes('noreferrer')) {
    tokens.push('noreferrer');
  }
  const valueIndex = relMatch[1] !== undefined ? 1 : 2;
  const valueStart = tagStart + (relMatch.index ?? 0) + relMatch[0].indexOf(relMatch[valueIndex]);
  const valueEnd = valueStart + currentValue.length
  return TextEdit.replace(
    {
      start: document.positionAt(valueStart),
      end: document.positionAt(valueEnd)
    },
    tokens.join(' ')
  );
}

function elementStartOffset(text: string, offset: number): number | undefined {
  for (let index = offset; index >= 0; index--) {
    if (text[index] === '<') {
      return index;
    }
    if (text[index] === '\n') {
      break;
    }
  }
  return undefined;
}

function findTagCloseOffset(text: string, startOffset: number): number | undefined {
  let braceDepth = 0;
  let quote: '"' | '\'' | '`' | undefined;
  for (let index = startOffset; index < text.length; index++) {
    const ch = text[index];
    if (quote !== undefined) {
      if (quote !== '`' && ch === '\\') {
        index++;
        continue;
      }
      if (ch === quote) {
        quote = undefined;
      }
      continue;
    }
    if (ch === '"' || ch === '\'' || ch === '`') {
      quote = ch;
      continue;
    }
    if (ch === '{') {
      braceDepth++;
      continue;
    }
    if (ch === '}') {
      braceDepth = Math.max(0, braceDepth - 1);
      continue;
    }
    if (ch === '>' && braceDepth === 0) {
      return index;
    }
  }
  return undefined;
}

function insertionOffsetBeforeTagClose(text: string, tagCloseOffset: number): number {
  let index = tagCloseOffset - 1;
  while (index >= 0 && /\s/.test(text[index])) {
    index--;
  }
  if (index >= 0 && text[index] === '/') {
    return index;
  }
  return tagCloseOffset;
}

function fullDocumentRange(document: TextDocument): Range {
  const lines = document.getText().split(/\r?\n/);
  const lastLine = Math.max(0, lines.length - 1);
  return {
    start: { line: 0, character: 0 },
    end: { line: lastLine, character: (lines[lastLine] ?? '').length }
  };
}
