import { SemanticTokens, SemanticTokensBuilder } from 'vscode-languageserver/node';
import { TextDocument } from 'vscode-languageserver-textdocument';

const TOKEN_TYPES = [
  'namespace',
  'type',
  'class',
  'parameter',
  'variable',
  'property',
  'function',
  'keyword',
  'string',
  'number',
  'operator'
] as const;

const TOKEN_MODIFIERS = ['declaration', 'defaultLibrary'] as const;

type TokenType = typeof TOKEN_TYPES[number];
type TokenModifier = typeof TOKEN_MODIFIERS[number];

interface SemanticToken {
  offset: number;
  length: number;
  type: TokenType;
  modifiers: readonly TokenModifier[];
  priority: number;
}

interface ComponentContext {
  start: number;
  end: number;
  paramNames: ReadonlySet<string>;
}

interface GoSegmentOptions {
  parameterNames?: ReadonlySet<string>;
}

const tokenTypeIndex = new Map<TokenType, number>(
  TOKEN_TYPES.map((type, index) => [type, index])
);

const tokenModifierIndex = new Map<TokenModifier, number>(
  TOKEN_MODIFIERS.map((modifier, index) => [modifier, index])
);

const KEYWORDS = new Set([
  'break', 'case', 'chan', 'component', 'const', 'continue', 'default', 'defer',
  'else', 'fallthrough', 'for', 'func', 'go', 'if', 'import', 'interface', 'map',
  'package', 'range', 'return', 'select', 'struct', 'switch', 'type', 'var'
]);

const BUILTIN_TYPES = new Set([
  'any', 'bool', 'byte', 'complex64', 'complex128', 'error', 'float32', 'float64',
  'int', 'int8', 'int16', 'int32', 'int64', 'rune', 'string', 'uint', 'uint8',
  'uint16', 'uint32', 'uint64', 'uintptr'
]);

const BUILTIN_FUNCTIONS = new Set([
  'append', 'cap', 'clear', 'close', 'complex', 'copy', 'delete', 'imag', 'len',
  'make', 'max', 'min', 'new', 'panic', 'print', 'println', 'real', 'recover'
]);

const BUILTIN_VALUES = new Set(['false', 'nil', 'true']);
const GSX_BUILTIN_TAGS = new Set(['fragment', 'raw', 'slot']);

export const semanticTokenLegend = {
  tokenTypes: [...TOKEN_TYPES],
  tokenModifiers: [...TOKEN_MODIFIERS]
};

export function provideSemanticTokens(document: TextDocument): SemanticTokens {
  const text = document.getText();
  const collector = new TokenCollector(document);
  const blockBraceOffsets = new Set<number>();

  tokenizePackageAndImports(text, collector);
  const componentContexts = tokenizeComponentDeclarations(text, collector, blockBraceOffsets);
  tokenizeControlFlowHeaders(text, collector, blockBraceOffsets, componentContexts);
  tokenizeComponentTags(text, collector);
  tokenizeEmbeddedExpressions(text, collector, blockBraceOffsets, componentContexts);

  return collector.build();
}

class TokenCollector {
  private readonly tokens: SemanticToken[] = [];

  constructor(private readonly document: TextDocument) {}

  add(offset: number, length: number, type: TokenType, modifiers: readonly TokenModifier[] = [], priority = 0): void {
    if (length <= 0) {
      return;
    }
    this.tokens.push({ offset, length, type, modifiers, priority });
  }

  build(): SemanticTokens {
    const builder = new SemanticTokensBuilder();
    const exactSeen = new Set<string>();
    const tokens = this.tokens.slice().sort((a, b) =>
      a.offset - b.offset ||
      a.length - b.length ||
      b.priority - a.priority ||
      tokenTypeValue(a.type) - tokenTypeValue(b.type)
    );
    let lastOffset = -1;
    let lastEnd = -1;
    for (const token of tokens) {
      const key = `${token.offset}:${token.length}:${token.type}:${modifierMask(token.modifiers)}`;
      if (exactSeen.has(key)) {
        continue;
      }
      if (token.offset < lastEnd && token.offset >= lastOffset) {
        continue;
      }
      exactSeen.add(key);
      const position = this.document.positionAt(token.offset);
      builder.push(
        position.line,
        position.character,
        token.length,
        tokenTypeValue(token.type),
        modifierMask(token.modifiers)
      );
      lastOffset = token.offset;
      lastEnd = token.offset + token.length;
    }
    return builder.build();
  }
}

function tokenizePackageAndImports(text: string, collector: TokenCollector): void {
  let inImportBlock = false;
  forEachLine(text, (line, lineStart) => {
    const trimmed = line.trim();
    if (trimmed === '' || trimmed.startsWith('//')) {
      return;
    }
    if (inImportBlock) {
      if (trimmed === ')') {
        inImportBlock = false;
        collector.add(lineStart + line.indexOf(')'), 1, 'operator');
        return;
      }
      tokenizeImportLine(line, lineStart, collector, false);
      return;
    }
    const packageMatch = /^(\s*)(package)\s+([A-Za-z_][A-Za-z0-9_]*)/.exec(line);
    if (packageMatch !== null) {
      collector.add(lineStart + packageMatch[1].length, packageMatch[2].length, 'keyword');
      collector.add(lineStart + packageMatch[1].length + packageMatch[2].length + 1, packageMatch[3].length, 'namespace');
      return;
    }
    const importBlockMatch = /^(\s*)(import)\s*\($/.exec(line);
    if (importBlockMatch !== null) {
      collector.add(lineStart + importBlockMatch[1].length, importBlockMatch[2].length, 'keyword');
      collector.add(lineStart + line.lastIndexOf('('), 1, 'operator');
      inImportBlock = true;
      return;
    }
    tokenizeImportLine(line, lineStart, collector, true);
  });
}

function tokenizeImportLine(line: string, lineStart: number, collector: TokenCollector, singleLine: boolean): void {
  const prefix = singleLine ? /^(\s*)(import)\s+/ : /^(\s*)/;
  const match = prefix.exec(line);
  if (match === null) {
    return;
  }
  let cursor = lineStart + match[0].length;
  if (singleLine) {
    collector.add(lineStart + match[1].length, 'import'.length, 'keyword');
  }
  const rest = line.slice(match[0].length);
  const importMatch = /^(?:(?<alias>[A-Za-z_][A-Za-z0-9_]*)\s+)?(?<path>"[^"]+")/.exec(rest);
  if (importMatch?.groups === undefined) {
    return;
  }
  if (importMatch.groups.alias !== undefined) {
    collector.add(cursor, importMatch.groups.alias.length, 'namespace');
    cursor += importMatch.groups.alias.length;
    cursor += rest.slice(importMatch.groups.alias.length).match(/^\s+/)?.[0].length ?? 0;
  }
  const quotedPath = importMatch.groups.path;
  const pathOffset = line.indexOf(quotedPath, match[0].length);
  collector.add(lineStart + pathOffset, quotedPath.length, 'string');
}

function tokenizeComponentDeclarations(
  text: string,
  collector: TokenCollector,
  blockBraceOffsets: Set<number>
): ComponentContext[] {
  const contexts: Array<{ start: number; paramNames: Set<string> }> = [];
  forEachLine(text, (line, lineStart) => {
    const match = /^(\s*)(component)\s+([A-Z][A-Za-z0-9_]*)\s*\((.*)\)\s*(\{)?\s*$/.exec(line);
    if (match === null) {
      return;
    }
    const keywordOffset = lineStart + match[1].length;
    collector.add(keywordOffset, match[2].length, 'keyword');
    const componentNameOffset = keywordOffset + match[2].length + 1;
    collector.add(componentNameOffset, match[3].length, 'class', ['declaration'], 2);
    const paramsStart = line.indexOf('(', componentNameOffset - lineStart) + 1;
    const paramsText = match[4];
    const paramNames = tokenizeComponentParams(paramsText, lineStart + paramsStart, collector);
    const braceIndex = line.lastIndexOf('{');
    if (braceIndex >= 0) {
      blockBraceOffsets.add(lineStart + braceIndex);
      collector.add(lineStart + braceIndex, 1, 'operator');
    }
    contexts.push({ start: lineStart + match[1].length, paramNames });
  });
  const finalized: ComponentContext[] = [];
  for (let i = 0; i < contexts.length; i++) {
    finalized.push({
      start: contexts[i].start,
      end: i+1 < contexts.length ? contexts[i+1].start : text.length,
      paramNames: contexts[i].paramNames
    });
  }
  return finalized;
}

function tokenizeComponentParams(paramsText: string, baseOffset: number, collector: TokenCollector): Set<string> {
  const paramNames = new Set<string>();
  for (const part of splitTopLevelRanges(paramsText, ',')) {
    const trimmedStart = part.text.search(/\S/);
    if (trimmedStart < 0) {
      continue;
    }
    const trimmedEnd = lastNonWhitespaceIndex(part.text);
    const segment = part.text.slice(trimmedStart, trimmedEnd + 1);
    const segmentOffset = baseOffset + part.start + trimmedStart;
    const nameMatch = /^([A-Za-z_][A-Za-z0-9_]*)\b/.exec(segment);
    if (nameMatch === null) {
      tokenizeGoSegment(segment, segmentOffset, collector);
      continue;
    }
    collector.add(segmentOffset, nameMatch[1].length, 'parameter', ['declaration'], 2);
    paramNames.add(nameMatch[1]);
    tokenizeGoSegment(segment.slice(nameMatch[1].length), segmentOffset + nameMatch[1].length, collector);
  }
  return paramNames;
}

function tokenizeControlFlowHeaders(
  text: string,
  collector: TokenCollector,
  blockBraceOffsets: Set<number>,
  componentContexts: readonly ComponentContext[]
): void {
  forEachLine(text, (line, lineStart) => {
    const trimmed = line.trim();
    if (trimmed === '' || trimmed.startsWith('<') || trimmed === '}') {
      return;
    }
    const context = componentContextForOffset(componentContexts, lineStart);
    const parameterNames = context?.paramNames;

    const elseIfMatch = /^(\s*)(else)\s+(if)\b(.*?)(\{)\s*$/.exec(line);
    if (elseIfMatch !== null) {
      const elseOffset = lineStart + elseIfMatch[1].length;
      collector.add(elseOffset, elseIfMatch[2].length, 'keyword');
      const ifOffset = elseOffset + elseIfMatch[2].length + 1;
      collector.add(ifOffset, elseIfMatch[3].length, 'keyword');
      const headerOffset = ifOffset + elseIfMatch[3].length;
      tokenizeGoSegment(elseIfMatch[4], headerOffset, collector, { parameterNames });
      const braceOffset = lineStart + line.lastIndexOf('{');
      blockBraceOffsets.add(braceOffset);
      collector.add(braceOffset, 1, 'operator');
      return;
    }

    const headerMatch = /^(\s*)(if|for)\b(.*?)(\{)\s*$/.exec(line);
    if (headerMatch !== null) {
      const keywordOffset = lineStart + headerMatch[1].length;
      collector.add(keywordOffset, headerMatch[2].length, 'keyword');
      tokenizeGoSegment(headerMatch[3], keywordOffset + headerMatch[2].length, collector, { parameterNames });
      const braceOffset = lineStart + line.lastIndexOf('{');
      blockBraceOffsets.add(braceOffset);
      collector.add(braceOffset, 1, 'operator');
      return;
    }

    const elseMatch = /^(\s*)(else)\s*(\{)\s*$/.exec(line);
    if (elseMatch !== null) {
      const keywordOffset = lineStart + elseMatch[1].length;
      collector.add(keywordOffset, elseMatch[2].length, 'keyword');
      const braceOffset = lineStart + line.lastIndexOf('{');
      blockBraceOffsets.add(braceOffset);
      collector.add(braceOffset, 1, 'operator');
      return;
    }

    const closeMatch = /^(\s*)(\})\s*$/.exec(line);
    if (closeMatch !== null) {
      collector.add(lineStart + closeMatch[1].length, 1, 'operator');
    }
  });
}

function tokenizeComponentTags(text: string, collector: TokenCollector): void {
  forEachLine(text, (line, lineStart) => {
    for (const match of line.matchAll(/<\/?((?:[A-Z][A-Za-z0-9_]*|[a-z_][A-Za-z0-9_]*\.[A-Z][A-Za-z0-9_]*|fragment|slot|raw))\b/g)) {
      const name = match[1];
      const full = match[0];
      const leading = full.startsWith('</') ? 2 : 1;
      const offset = lineStart + (match.index ?? 0) + leading;
      const type: TokenType = GSX_BUILTIN_TAGS.has(name) ? 'keyword' : 'class';
      collector.add(offset, name.length, type);
    }
  });
}

function tokenizeEmbeddedExpressions(
  text: string,
  collector: TokenCollector,
  blockBraceOffsets: ReadonlySet<number>,
  componentContexts: readonly ComponentContext[]
): void {
  let index = 0;
  while (index < text.length) {
    const brace = text.indexOf('{', index);
    if (brace < 0) {
      return;
    }
    if (!looksLikeEmbeddedExpressionStart(text, brace, blockBraceOffsets)) {
      index = brace + 1;
      continue;
    }
    const end = findMatchingBrace(text, brace, blockBraceOffsets);
    if (end < 0) {
      return;
    }
    collector.add(brace, 1, 'operator');
    collector.add(end, 1, 'operator');
    const context = componentContextForOffset(componentContexts, brace);
    tokenizeGoSegment(text.slice(brace + 1, end), brace + 1, collector, {
      parameterNames: context?.paramNames
    });
    index = end + 1;
  }
}

function tokenizeGoSegment(
  segment: string,
  baseOffset: number,
  collector: TokenCollector,
  options: GoSegmentOptions = {}
): void {
  let index = 0;
  let afterDot = false;
  while (index < segment.length) {
    const ch = segment[index];
    if (isWhitespace(ch)) {
      index++;
      continue;
    }
    if (ch === '/' && segment[index + 1] === '/') {
      return;
    }
    if (ch === '/' && segment[index + 1] === '*') {
      const end = segment.indexOf('*/', index + 2);
      if (end < 0) {
        return;
      }
      index = end + 2;
      continue;
    }
    if (ch === '"' || ch === '\'' || ch === '`') {
      const end = findStringEnd(segment, index, ch);
      const length = Math.max(1, end - index + 1);
      collector.add(baseOffset + index, length, 'string');
      index += length;
      afterDot = false;
      continue;
    }
    const numberMatch = /^(?:0x[0-9A-Fa-f]+|\d+(?:\.\d+)?)\b/.exec(segment.slice(index));
    if (numberMatch !== null) {
      collector.add(baseOffset + index, numberMatch[0].length, 'number');
      index += numberMatch[0].length;
      afterDot = false;
      continue;
    }
    const operator = readOperator(segment, index);
    if (operator !== undefined) {
      collector.add(baseOffset + index, operator.length, 'operator');
      index += operator.length;
      afterDot = operator === '.';
      continue;
    }
    const identMatch = /^[A-Za-z_][A-Za-z0-9_]*/.exec(segment.slice(index));
    if (identMatch !== null) {
      const ident = identMatch[0];
      const nextNonWhitespace = peekNonWhitespace(segment, index + ident.length);
      if (afterDot) {
        collector.add(baseOffset + index, ident.length, 'property');
      } else if (KEYWORDS.has(ident) || BUILTIN_VALUES.has(ident)) {
        collector.add(baseOffset + index, ident.length, 'keyword');
      } else if (BUILTIN_TYPES.has(ident) || startsUpper(ident)) {
        collector.add(baseOffset + index, ident.length, 'type');
      } else if (options.parameterNames?.has(ident) === true) {
        collector.add(baseOffset + index, ident.length, 'parameter');
      } else if (BUILTIN_FUNCTIONS.has(ident)) {
        collector.add(baseOffset + index, ident.length, 'function', ['defaultLibrary']);
      } else if (nextNonWhitespace === '(') {
        collector.add(baseOffset + index, ident.length, 'function');
      } else {
        collector.add(baseOffset + index, ident.length, 'variable');
      }
      index += ident.length;
      afterDot = false;
      continue;
    }
    index++;
    afterDot = false;
  }
}

function looksLikeEmbeddedExpressionStart(
  text: string,
  braceOffset: number,
  blockBraceOffsets: ReadonlySet<number>
): boolean {
  if (blockBraceOffsets.has(braceOffset)) {
    return false;
  }
  const lineStart = text.lastIndexOf('\n', braceOffset - 1) + 1;
  const before = text.slice(lineStart, braceOffset);
  const prev = before.trimEnd().slice(-1);
  if (prev === '' || prev === '=' || prev === '>' || prev === ',' || prev === ':' || prev === '(' || prev === '[') {
    return true;
  }
  const lastLt = before.lastIndexOf('<');
  const lastGt = before.lastIndexOf('>');
  return lastGt > lastLt;
}

function findMatchingBrace(text: string, startBrace: number, blockBraceOffsets: ReadonlySet<number>): number {
  let depth = 1;
  let index = startBrace + 1;
  while (index < text.length) {
    const ch = text[index];
    if (ch === '"' || ch === '\'' || ch === '`') {
      index = findStringEnd(text, index, ch) + 1;
      continue;
    }
    if (ch === '/' && text[index + 1] === '/') {
      const newline = text.indexOf('\n', index + 2);
      if (newline < 0) {
        return -1;
      }
      index = newline + 1;
      continue;
    }
    if (ch === '/' && text[index + 1] === '*') {
      const end = text.indexOf('*/', index + 2);
      if (end < 0) {
        return -1;
      }
      index = end + 2;
      continue;
    }
    if (ch === '{' && !blockBraceOffsets.has(index)) {
      depth++;
    } else if (ch === '}') {
      depth--;
      if (depth === 0) {
        return index;
      }
    }
    index++;
  }
  return -1;
}

function componentContextForOffset(
  contexts: readonly ComponentContext[],
  offset: number
): ComponentContext | undefined {
  for (let i = contexts.length - 1; i >= 0; i--) {
    const context = contexts[i];
    if (offset >= context.start && offset < context.end) {
      return context;
    }
  }
  return undefined;
}

function splitTopLevel(text: string, separator: ',' | ';'): string[] {
  return splitTopLevelRanges(text, separator).map((part) => part.text);
}

function splitTopLevelRanges(text: string, separator: ',' | ';'): Array<{ start: number; text: string }> {
  let start = 0;
  let parenDepth = 0;
  let bracketDepth = 0;
  let braceDepth = 0;
  let quote: '"' | '\'' | '`' | undefined;
  const ranges: Array<{ start: number; text: string }> = [];
  for (let index = 0; index < text.length; index++) {
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
    switch (ch) {
      case '(':
        parenDepth++;
        break;
      case ')':
        parenDepth = Math.max(0, parenDepth - 1);
        break;
      case '[':
        bracketDepth++;
        break;
      case ']':
        bracketDepth = Math.max(0, bracketDepth - 1);
        break;
      case '{':
        braceDepth++;
        break;
      case '}':
        braceDepth = Math.max(0, braceDepth - 1);
        break;
      default:
        break;
    }
    if (ch === separator && parenDepth === 0 && bracketDepth === 0 && braceDepth === 0) {
      ranges.push({ start, text: text.slice(start, index) });
      start = index + 1;
    }
  }
  ranges.push({ start, text: text.slice(start) });
  return ranges;
}

function forEachLine(text: string, fn: (line: string, startOffset: number) => void): void {
  let start = 0;
  for (let index = 0; index <= text.length; index++) {
    if (index === text.length || text[index] === '\n') {
      let end = index;
      if (end > start && text[end - 1] === '\r') {
        end--;
      }
      fn(text.slice(start, end), start);
      start = index + 1;
    }
  }
}

function readOperator(text: string, index: number): string | undefined {
  const operators = ['...', ':=', '==', '!=', '<=', '>=', '&&', '||', '<-', '.', '=', '+', '-', '*', '/', '%', '<', '>', '!', '&', '|', '^', ':', ',', '(', ')', '[', ']', '{', '}'];
  for (const operator of operators) {
    if (text.startsWith(operator, index)) {
      return operator;
    }
  }
  return undefined;
}

function findStringEnd(text: string, start: number, quote: '"' | '\'' | '`'): number {
  let index = start + 1;
  while (index < text.length) {
    const ch = text[index];
    if (quote !== '`' && ch === '\\') {
      index += 2;
      continue;
    }
    if (ch === quote) {
      return index;
    }
    index++;
  }
  return Math.max(start, text.length - 1);
}

function peekNonWhitespace(text: string, index: number): string | undefined {
  while (index < text.length) {
    if (!isWhitespace(text[index])) {
      return text[index];
    }
    index++;
  }
  return undefined;
}

function lastNonWhitespaceIndex(text: string): number {
  for (let index = text.length - 1; index >= 0; index--) {
    if (!isWhitespace(text[index])) {
      return index;
    }
  }
  return -1;
}

function isWhitespace(ch: string | undefined): boolean {
  return ch === ' ' || ch === '\t' || ch === '\r' || ch === '\n';
}

function startsUpper(text: string): boolean {
  return text.length > 0 && text[0] >= 'A' && text[0] <= 'Z';
}

function tokenTypeValue(type: TokenType): number {
  return tokenTypeIndex.get(type) ?? 0;
}

function modifierMask(modifiers: readonly TokenModifier[]): number {
  let mask = 0;
  for (const modifier of modifiers) {
    const bit = tokenModifierIndex.get(modifier);
    if (bit !== undefined) {
      mask |= 1 << bit;
    }
  }
  return mask;
}
