import * as fs from 'node:fs/promises';
import * as path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import {
  CompletionItem,
  CompletionItemKind,
  DocumentSymbol,
  Hover,
  InsertTextFormat,
  Location,
  LocationLink,
  MarkupKind,
  ParameterInformation,
  Position,
  Range,
  SignatureHelp,
  SignatureInformation,
  SymbolKind,
  TextEdit,
  WorkspaceEdit
} from 'vscode-languageserver/node';
import { TextDocument } from 'vscode-languageserver-textdocument';
import { goplsCompletion, goplsHover } from './goplsClient';

const HTML_TAGS = [
  'a', 'abbr', 'address', 'article', 'aside', 'audio', 'b', 'base', 'blockquote', 'body', 'br',
  'button', 'canvas', 'caption', 'code', 'col', 'colgroup', 'data', 'datalist', 'dd', 'del',
  'details', 'div', 'dl', 'dt', 'em', 'fieldset', 'figcaption', 'figure', 'footer', 'form', 'h1',
  'h2', 'h3', 'h4', 'h5', 'h6', 'head', 'header', 'hr', 'html', 'i', 'iframe', 'img', 'input',
  'label', 'legend', 'li', 'link', 'main', 'meta', 'nav', 'ol', 'option', 'p', 'pre', 'script',
  'section', 'select', 'small', 'source', 'span', 'strong', 'style', 'table', 'tbody', 'td',
  'textarea', 'tfoot', 'th', 'thead', 'title', 'tr', 'u', 'ul'
];

const GSX_TAGS = ['fragment', 'slot', 'raw'];

const COMMON_ATTRIBUTES = [
  'class', 'id', 'title', 'style', 'role', 'slot', 'hidden', 'tabindex', 'lang',
  'data-testid', 'data-state', 'aria-label', 'aria-hidden'
];

const TAG_ATTRIBUTES: Record<string, string[]> = {
  a: ['href', 'target', 'rel'],
  button: ['type', 'disabled', 'name', 'value'],
  form: ['method', 'action'],
  img: ['src', 'alt', 'width', 'height', 'loading'],
  input: ['type', 'name', 'value', 'placeholder', 'checked', 'disabled'],
  label: ['for'],
  link: ['rel', 'href'],
  meta: ['name', 'content', 'charset'],
  option: ['value', 'selected'],
  script: ['src', 'type', 'defer', 'async'],
  select: ['name', 'multiple', 'disabled'],
  slot: ['name'],
  raw: ['html'],
  textarea: ['name', 'rows', 'cols', 'placeholder', 'disabled']
};

const HOVER_DOCS: Record<string, string> = {
  package: 'Declares the Go package for the GSX file.',
  import: 'Imports a Go package or GSX component package for expressions and cross-package components.',
  component: 'Defines a typed GSX component that compiles to a Go render function.',
  if: 'Conditional rendering block. The condition is captured as Go code.',
  else: 'Fallback branch for a preceding `if` block.',
  for: 'Loop block. The header is captured as Go code and rendered repeatedly.',
  fragment: 'Groups child nodes without emitting a wrapping HTML element.',
  slot: 'Layout outlet. `<slot />` renders default children, and `<slot name="..." />` renders a named slot.',
  raw: 'Explicit raw HTML escape hatch. Requires `html={runtime.HTML(...)}`.',
  html: 'Attribute for `<raw>`. The value must be an expression that evaluates to trusted raw HTML.',
  class: 'Standard HTML class attribute.',
  id: 'Standard HTML id attribute.',
  slotattr: 'Routes direct child content into a named slot when calling a layout or component.'
};

const GO_BUILTIN_TYPE_DOCS: Record<string, string> = {
  any: 'Go predeclared alias for `interface{}`.',
  bool: 'Go predeclared boolean type.',
  byte: 'Go predeclared alias for `uint8`.',
  complex64: 'Go predeclared complex number type with `float32` components.',
  complex128: 'Go predeclared complex number type with `float64` components.',
  error: 'Go predeclared interface for error values.',
  float32: 'Go predeclared 32-bit floating-point type.',
  float64: 'Go predeclared 64-bit floating-point type.',
  int: 'Go predeclared signed integer type.',
  int8: 'Go predeclared 8-bit signed integer type.',
  int16: 'Go predeclared 16-bit signed integer type.',
  int32: 'Go predeclared 32-bit signed integer type.',
  int64: 'Go predeclared 64-bit signed integer type.',
  rune: 'Go predeclared alias for `int32`, conventionally used for Unicode code points.',
  string: 'Go predeclared UTF-8 string type.',
  uint: 'Go predeclared unsigned integer type.',
  uint8: 'Go predeclared 8-bit unsigned integer type.',
  uint16: 'Go predeclared 16-bit unsigned integer type.',
  uint32: 'Go predeclared 32-bit unsigned integer type.',
  uint64: 'Go predeclared 64-bit unsigned integer type.',
  uintptr: 'Go predeclared unsigned integer large enough to store pointer bits.'
};

interface TokenInfo {
  token: string;
  full: string;
  segmentIndex: number;
}

interface ComponentParam {
  name: string;
  type: string;
  nameStartCharacter: number;
  nameEndCharacter: number;
}

interface ComponentDefinition {
  name: string;
  signature: string;
  params: ComponentParam[];
  paramsByName: Map<string, string>;
  sourceUri?: string;
  startLine: number;
  startCharacter: number;
  endLine: number;
  endCharacter: number;
  nameStartCharacter: number;
  nameEndCharacter: number;
}

interface HoverBinding {
  kind: 'parameter' | 'loopVariable';
  type: string;
  owner: string;
}

type StructFieldIndex = Map<string, Map<string, string>>;

interface TagNameContext {
  tagName: string;
  nameStartOffset: number;
  nameEndOffset: number;
}

interface OpenComponentTagContext {
  tagName: string;
  nameStartOffset: number;
  nameEndOffset: number;
  attrsTailBeforeCursor: string;
}

interface TagAttributeOccurrence {
  name: string;
  startOffset: number;
  endOffset: number;
  spanEndOffset: number;
  value?: string;
  valueRange?: {
    startOffset: number;
    endOffset: number;
  };
}

interface ComponentTarget {
  importPath: string;
  name: string;
  originRange: Range;
}

interface SlotTarget {
  componentImportPath: string;
  componentName: string;
  slotName: string;
  originRange: Range;
}

interface SlotOccurrence {
  componentImportPath: string;
  componentName: string;
  slotName: string;
  range: Range;
  isDeclaration: boolean;
}

interface TagFrame {
  tagName: string;
  isComponent: boolean;
  componentImportPath?: string;
  componentName?: string;
}

interface GSXFileSnapshot {
  document: TextDocument;
  importPath: string;
  imports: Map<string, string>;
  definitions: Map<string, ComponentDefinition>;
}

interface FeatureOptions {
  goplsCommand?: string;
}

interface EmbeddedGoProbe {
  uri: string;
  text: string;
  position: Position;
}

export async function provideCompletionItems(
  document: TextDocument,
  position: Position,
  options: FeatureOptions = {}
): Promise<CompletionItem[] | undefined> {
  const goItems = await provideEmbeddedGoCompletionItems(document, position, options);
  if (goItems !== undefined && goItems.length > 0) {
    return goItems;
  }
  const offset = document.offsetAt(position);
  const windowStart = Math.max(0, offset - 2048);
  const before = document.getText({ start: document.positionAt(windowStart), end: position });
  const line = document.getText({
    start: { line: position.line, character: 0 },
    end: position
  });
  const items: CompletionItem[] = [];

  if (isTagNameContext(before)) {
    items.push(...tagItems(document));
    const importAlias = importedAliasContext(before);
    if (importAlias !== undefined) {
      return importedComponentItems(document, importAlias);
    }
    return items;
  }

  const tagContext = openTagContext(before);
  if (tagContext !== undefined && isAttributeContext(tagContext)) {
    return attributeItems(tagContext.tagName);
  }

  if (isTopLevelContext(line, before)) {
    items.push(...TOP_LEVEL_SNIPPETS);
  }
  if (isControlFlowContext(line, before)) {
    items.push(
      snippetItem('if', 'if ${1:condition} {\n  ${0}\n}', 'If block'),
      snippetItem('for', 'for ${1:_, item := range items} {\n  ${0}\n}', 'For-range block'),
      snippetItem('else', 'else {\n  ${0}\n}', 'Else block')
    );
  }

  return items.length > 0 ? items : undefined;
}

export async function provideHover(
  document: TextDocument,
  position: Position,
  options: FeatureOptions = {}
): Promise<Hover | null> {
  const goHover = await provideEmbeddedGoHover(document, position, options);
  if (goHover !== null) {
    return goHover;
  }
  const token = tokenInfoAt(document, position);
  if (token === undefined) {
    return null;
  }
  const doc = await hoverTextForPosition(document, position, token);
  if (doc === undefined) {
    return null;
  }
  return {
    contents: {
      kind: MarkupKind.Markdown,
      value: doc
    }
  };
}

export async function buildEmbeddedGoProbe(
  document: TextDocument,
  position: Position
): Promise<EmbeddedGoProbe | undefined> {
  const expression = embeddedExpressionAtPosition(document, position);
  if (expression === undefined) {
    return undefined;
  }
  const text = document.getText();
  const definitions = parseComponentDefinitions(text, document.uri);
  const component = componentAtLine(definitions, position.line);
  if (component === undefined) {
    return undefined;
  }
  const structs = await loadStructFieldIndex(document.uri);
  const bindings = bindingsAtLine(text, component, position.line, structs);
  const params = component.params.map((param) => `${param.name} ${param.type}`).join(', ');
  const overlay = await probeOverlayBase(document.uri);
  const prefix = (overlay?.text ?? headerBeforeFirstComponent(text)).trimEnd();
  const probeLines: string[] = [];
  probeLines.push(`func __gsxProbe(${params}) {`);
  for (const [name, binding] of bindings) {
    if (binding.kind !== 'loopVariable') {
      continue;
    }
    probeLines.push(`  var ${name} ${binding.type}`);
  }
  const exprPrefix = '  _ = ';
  const exprLineIndex = probeLines.length;
  probeLines.push(exprPrefix + expression.text);
  probeLines.push('}');
  const source = (prefix === '' ? '' : prefix + '\n\n') + probeLines.join('\n') + '\n';
  const prefixLineCount = prefix === '' ? 0 : prefix.split(/\r?\n/).length + 1;
  const line = prefixLineCount + exprLineIndex;
  const character = exprPrefix.length + expression.cursorOffset;
  return {
    uri: overlay?.uri ?? syntheticProbeURI(document.uri),
    text: source,
    position: { line, character }
  };
}

async function provideEmbeddedGoHover(
  document: TextDocument,
  position: Position,
  options: FeatureOptions
): Promise<Hover | null> {
  if (options.goplsCommand === undefined) {
    return null;
  }
  const probe = await buildEmbeddedGoProbe(document, position);
  if (probe === undefined) {
    return null;
  }
  const result = await goplsHover(options.goplsCommand, probe.uri, probe.text, probe.position);
  return (result ?? null) as Hover | null;
}

async function provideEmbeddedGoCompletionItems(
  document: TextDocument,
  position: Position,
  options: FeatureOptions
): Promise<CompletionItem[] | undefined> {
  if (options.goplsCommand === undefined) {
    return undefined;
  }
  const probe = await buildEmbeddedGoProbe(document, position);
  if (probe === undefined) {
    return undefined;
  }
  const result = await goplsCompletion(options.goplsCommand, probe.uri, probe.text, probe.position);
  const items = Array.isArray(result?.items) ? result.items : Array.isArray(result) ? result : [];
  if (items.length === 0) {
    return undefined;
  }
  const range = completionReplacementRange(document, position);
  return items.map((item: any) => ({
    label: item.label,
    kind: item.kind,
    detail: item.detail,
    documentation: item.documentation,
    sortText: item.sortText,
    filterText: item.filterText,
    insertTextFormat: item.insertTextFormat,
    textEdit: TextEdit.replace(range, item.textEdit?.newText ?? item.insertText ?? item.label),
    insertText: undefined,
    additionalTextEdits: undefined,
    commitCharacters: item.commitCharacters
  }));
}

export function provideDocumentSymbols(document: TextDocument): DocumentSymbol[] {
  const definitions = [...parseComponentDefinitions(document.getText(), document.uri).values()];
  return definitions.map((definition) => {
    const symbol = DocumentSymbol.create(
      definition.name,
      definition.signature,
      SymbolKind.Function,
      componentRange(definition),
      componentSelectionRange(definition)
    );
    symbol.children = [
      ...definition.params.map((param) => {
        const range = {
          start: { line: definition.startLine, character: param.nameStartCharacter },
          end: { line: definition.startLine, character: param.nameEndCharacter }
        };
        return DocumentSymbol.create(param.name, param.type, SymbolKind.Variable, range, range);
      }),
      ...slotOutletSymbols(document, definition)
    ];
    return symbol;
  });
}

export async function provideDefinition(
  document: TextDocument,
  position: Position
): Promise<LocationLink[] | null> {
  const slot = await resolveSlotTarget(document, position);
  if (slot !== undefined) {
    const declarations = await slotDeclarationOccurrences(document.uri, slot);
    if (declarations.length === 0) {
      return null;
    }
    return declarations.map((occurrence) => ({
      targetUri: occurrence.document.uri,
      targetRange: occurrence.range,
      targetSelectionRange: occurrence.range,
      originSelectionRange: slot.originRange
    }));
  }
  const tag = componentTagNameContext(document, position);
  if (tag === undefined || !isComponentReference(tag.tagName)) {
    return null;
  }
  const text = document.getText();
  const localDefinitions = parseComponentDefinitions(text, document.uri);
  const definition = localDefinitions.get(tag.tagName) ??
    await importedComponentDefinition(document.uri, text, tag.tagName);
  if (definition === undefined) {
    return null;
  }
  return [{
    targetUri: definition.sourceUri ?? document.uri,
    targetRange: componentRange(definition),
    targetSelectionRange: componentSelectionRange(definition),
    originSelectionRange: offsetRange(document, tag.nameStartOffset, tag.nameEndOffset)
  }];
}

export async function provideSignatureHelp(
  document: TextDocument,
  position: Position
): Promise<SignatureHelp | null> {
  const tag = openComponentTagContextAt(document, position);
  if (tag === undefined || !isComponentReference(tag.tagName)) {
    return null;
  }
  const text = document.getText();
  const localDefinitions = parseComponentDefinitions(text, document.uri);
  const definition = localDefinitions.get(tag.tagName) ??
    await importedComponentDefinition(document.uri, text, tag.tagName);
  if (definition === undefined) {
    return null;
  }
  const activeParameter = activeComponentParameter(definition, tag.attrsTailBeforeCursor, document.offsetAt(position), tag.nameEndOffset);
  const parameters = definition.params.map((param) =>
    ParameterInformation.create(
      `${param.name} ${param.type}`,
      `Type: ${param.type}`
    )
  );
  return {
    activeSignature: 0,
    activeParameter,
    signatures: [
      SignatureInformation.create(
        definition.signature,
        `component ${definition.name}`,
        ...parameters
      )
    ]
  };
}

export async function provideReferences(
  document: TextDocument,
  position: Position,
  includeDeclaration: boolean
): Promise<Location[] | null> {
  const slotTarget = await resolveSlotTarget(document, position);
  if (slotTarget !== undefined) {
    const occurrences = await slotOccurrences(document.uri, slotTarget);
    return occurrences
      .filter((occurrence) => includeDeclaration || !occurrence.isDeclaration)
      .map((occurrence) => ({
        uri: occurrence.document.uri,
        range: occurrence.range
      }));
  }
  const target = await resolveComponentTarget(document, position);
  if (target === undefined) {
    return null;
  }
  const occurrences = await componentOccurrences(document.uri, target);
  return occurrences
    .filter((occurrence) => includeDeclaration || !occurrence.isDeclaration)
    .map((occurrence) => ({
      uri: occurrence.document.uri,
      range: occurrence.range
    }));
}

export async function prepareComponentRename(
  document: TextDocument,
  position: Position
): Promise<Range | null> {
  const slotTarget = await resolveSlotTarget(document, position);
  if (slotTarget !== undefined) {
    return slotTarget.originRange;
  }
  const target = await resolveComponentTarget(document, position);
  return target?.originRange ?? null;
}

export async function provideRenameEdits(
  document: TextDocument,
  position: Position,
  newName: string
): Promise<WorkspaceEdit | null> {
  const slotTarget = await resolveSlotTarget(document, position);
  if (slotTarget !== undefined) {
    if (!/^[A-Za-z][A-Za-z0-9_-]*$/.test(newName)) {
      return null;
    }
    const occurrences = await slotOccurrences(document.uri, slotTarget);
    const changes: Record<string, TextEdit[]> = {};
    for (const occurrence of occurrences) {
      const edits = changes[occurrence.document.uri] ?? [];
      edits.push(TextEdit.replace(occurrence.range, newName));
      changes[occurrence.document.uri] = edits;
    }
    return { changes };
  }
  if (!/^[A-Z][A-Za-z0-9_]*$/.test(newName)) {
    return null;
  }
  const target = await resolveComponentTarget(document, position);
  if (target === undefined) {
    return null;
  }
  const occurrences = await componentOccurrences(document.uri, target);
  const changes: Record<string, TextEdit[]> = {};
  for (const occurrence of occurrences) {
    const edits = changes[occurrence.document.uri] ?? [];
    edits.push(TextEdit.replace(occurrence.range, newName));
    changes[occurrence.document.uri] = edits;
  }
  return { changes };
}

const TOP_LEVEL_SNIPPETS = [
  snippetItem('package', 'package ${1:name}', 'Package declaration'),
  snippetItem('import', 'import ${1:alias }"${2:path/to/pkg}"', 'Import declaration'),
  snippetItem('component', 'component ${1:Name}(${2}) {\n  ${0}\n}', 'Component declaration'),
  snippetItem('if', 'if ${1:condition} {\n  ${0}\n}', 'If block'),
  snippetItem('for', 'for ${1:_, item := range items} {\n  ${0}\n}', 'For-range block')
];

function tagItems(document: TextDocument): CompletionItem[] {
  const items = HTML_TAGS.map((tag) => tagCompletion(tag, CompletionItemKind.Property));
  items.push(...GSX_TAGS.map((tag) => tagCompletion(tag, CompletionItemKind.Keyword)));
  for (const component of localComponents(document.getText())) {
    items.push(componentItem(component, 'Local GSX component'));
  }
  for (const alias of importedAliases(document.getText())) {
    items.push({
      label: `${alias}.`,
      kind: CompletionItemKind.Module,
      insertText: `${alias}.`,
      detail: 'Imported GSX component namespace'
    });
  }
  return items;
}

function attributeItems(tagName: string): CompletionItem[] {
  const names = new Set<string>(COMMON_ATTRIBUTES);
  for (const attr of TAG_ATTRIBUTES[tagName] ?? []) {
    names.add(attr);
  }
  return [...names].sort().map((attr) => ({
    label: attr,
    kind: CompletionItemKind.Field,
    insertText: attributeSnippet(attr),
    insertTextFormat: InsertTextFormat.Snippet,
    detail: `Attribute for <${tagName}>`
  }));
}

function attributeSnippet(attr: string): string {
  if (attr === 'slot' || attr === 'class' || attr === 'id' || attr === 'title' || attr === 'name' || attr === 'content' || attr === 'href' || attr === 'src' || attr === 'alt' || attr === 'for' || attr === 'type' || attr === 'value') {
    return `${attr}="$1"`;
  }
  if (attr.startsWith('data-') || attr.startsWith('aria-')) {
    return `${attr}="$1"`;
  }
  if (attr === 'html') {
    return 'html={$1}';
  }
  return attr;
}

function tagCompletion(label: string, kind: CompletionItemKind): CompletionItem {
  const selfClosing = label === 'slot' || label === 'raw' || isVoidTag(label);
  return {
    label,
    kind,
    insertText: selfClosing ? `${label} $1/>` : `${label}$1>$0</${label}>`,
    insertTextFormat: InsertTextFormat.Snippet,
    detail: GSX_TAGS.includes(label) ? 'Built-in GSX tag' : 'HTML tag'
  };
}

function snippetItem(label: string, snippet: string, detail: string): CompletionItem {
  return {
    label,
    kind: CompletionItemKind.Snippet,
    insertText: snippet,
    insertTextFormat: InsertTextFormat.Snippet,
    detail
  };
}

function componentItem(label: string, detail: string): CompletionItem {
  return {
    label,
    kind: CompletionItemKind.Class,
    insertText: `${label}$1>$0</${label}>`,
    insertTextFormat: InsertTextFormat.Snippet,
    detail
  };
}

function localComponents(text: string): string[] {
  const matches = text.matchAll(/\bcomponent\s+([A-Z][A-Za-z0-9_]*)\s*\(/g);
  const names = new Set<string>();
  for (const match of matches) {
    names.add(match[1]);
  }
  return [...names].sort();
}

function importedAliases(text: string): string[] {
  return [...parseImports(text).keys()].sort();
}

async function importedComponentItems(document: TextDocument, alias: string): Promise<CompletionItem[] | undefined> {
  const imports = parseImports(document.getText());
  const importPath = imports.get(alias);
  if (importPath === undefined) {
    return undefined;
  }
  const names = await importedComponents(document.uri, importPath);
  return names.map((name) => componentItem(`${alias}.${name}`, `Component from ${importPath}`));
}

function parseImports(text: string): Map<string, string> {
  const imports = new Map<string, string>();
  const lines = text.split(/\r?\n/);
  let inImportBlock = false;
  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith('component ')) {
      break;
    }
    if (trimmed === '' || trimmed.startsWith('//')) {
      continue;
    }
    if (trimmed.startsWith('import (')) {
      inImportBlock = true;
      continue;
    }
    if (inImportBlock) {
      if (trimmed === ')') {
        inImportBlock = false;
        continue;
      }
      const parsed = parseImportLine(trimmed);
      if (parsed !== undefined) {
        imports.set(parsed.alias, parsed.path);
      }
      continue;
    }
    if (trimmed.startsWith('import ')) {
      const parsed = parseImportLine(trimmed.slice('import '.length).trim());
      if (parsed !== undefined) {
        imports.set(parsed.alias, parsed.path);
      }
    }
  }
  return imports;
}

function parseImportLine(line: string): { alias: string; path: string } | undefined {
  const match = /^(?:(?<alias>[A-Za-z_][A-Za-z0-9_]*)\s+)?"(?<path>[^"]+)"$/.exec(line);
  if (match?.groups === undefined) {
    return undefined;
  }
  const importPath = match.groups.path;
  const alias = match.groups.alias ?? path.posix.basename(importPath);
  return { alias, path: importPath };
}

async function importedComponents(uri: string, importPath: string): Promise<string[]> {
  const module = await findNearestModule(uriToPath(uri));
  if (module === undefined) {
    return [];
  }
  if (!(importPath === module.modulePath || importPath.startsWith(module.modulePath + '/'))) {
    return [];
  }
  const rel = importPath === module.modulePath ? '' : importPath.slice(module.modulePath.length + 1);
  const targetDir = path.join(module.rootDir, ...rel.split('/').filter(Boolean));
  try {
    const entries = await fs.readdir(targetDir, { withFileTypes: true });
    const names = new Set<string>();
    for (const entry of entries) {
      if (!entry.isFile() || !entry.name.endsWith('.gsx')) {
        continue;
      }
      const text = await fs.readFile(path.join(targetDir, entry.name), 'utf8');
      for (const match of text.matchAll(/\bcomponent\s+([A-Z][A-Za-z0-9_]*)\s*\(/g)) {
        names.add(match[1]);
      }
    }
    return [...names].sort();
  } catch {
    return [];
  }
}

async function packageImportPathForURI(uri: string): Promise<string | undefined> {
  const filePath = uriToPath(uri);
  const module = await findNearestModule(filePath);
  if (module === undefined) {
    return undefined;
  }
  return packageImportPath(module, path.dirname(filePath));
}

function packageImportPath(
  module: { rootDir: string; modulePath: string },
  dir: string
): string {
  const rel = path.relative(module.rootDir, dir);
  if (rel === '') {
    return module.modulePath;
  }
  return path.posix.join(module.modulePath, ...rel.split(path.sep).filter(Boolean));
}

async function scanModuleGSXFiles(uri: string): Promise<GSXFileSnapshot[]> {
  const module = await findNearestModule(uriToPath(uri));
  if (module === undefined) {
    return [];
  }
  const filePaths = await collectGSXFiles(module.rootDir);
  const snapshots: GSXFileSnapshot[] = [];
  for (const filePath of filePaths) {
    const text = await fs.readFile(filePath, 'utf8');
    const fileUri = pathToFileURL(filePath).toString();
    const document = TextDocument.create(fileUri, 'gsx', 1, text);
    snapshots.push({
      document,
      importPath: packageImportPath(module, path.dirname(filePath)),
      imports: parseImports(text),
      definitions: parseComponentDefinitions(text, fileUri)
    });
  }
  return snapshots;
}

async function collectGSXFiles(dir: string): Promise<string[]> {
  const out: string[] = [];
  const entries = await fs.readdir(dir, { withFileTypes: true });
  for (const entry of entries) {
    if (entry.name === 'node_modules' || entry.name.startsWith('.')) {
      continue;
    }
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      out.push(...await collectGSXFiles(fullPath));
      continue;
    }
    if (entry.isFile() && entry.name.endsWith('.gsx')) {
      out.push(fullPath);
    }
  }
  return out;
}

async function findNearestModule(startPath: string): Promise<{ rootDir: string; modulePath: string } | undefined> {
  let current = path.dirname(startPath);
  while (true) {
    const goModPath = path.join(current, 'go.mod');
    try {
      const text = await fs.readFile(goModPath, 'utf8');
      const modulePath = parseModulePath(text);
      if (modulePath !== undefined) {
        return { rootDir: current, modulePath };
      }
    } catch {
      // Keep walking upward.
    }
    const parent = path.dirname(current);
    if (parent === current) {
      return undefined;
    }
    current = parent;
  }
}

function parseModulePath(text: string): string | undefined {
  for (const line of text.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (trimmed.startsWith('module ')) {
      const parts = trimmed.split(/\s+/);
      return parts[1];
    }
  }
  return undefined;
}

function tokenInfoAt(document: TextDocument, position: Position): TokenInfo | undefined {
  const offset = document.offsetAt(position);
  const text = document.getText();
  if (offset >= text.length || !isTokenChar(text[offset])) {
    if (offset === 0 || !isTokenChar(text[offset - 1])) {
      return undefined;
    }
  }
  const pivot = offset < text.length && isTokenChar(text[offset]) ? offset : offset - 1;
  let fullStart = pivot;
  let fullEnd = pivot + 1;
  while (fullStart > 0 && isTokenChar(text[fullStart - 1])) {
    fullStart--;
  }
  while (fullEnd < text.length && isTokenChar(text[fullEnd])) {
    fullEnd++;
  }
  const full = text.slice(fullStart, fullEnd);
  if (full === '') {
    return undefined;
  }
  const relative = pivot - fullStart;
  const segments = full.split('.');
  let segmentStart = 0;
  for (let index = 0; index < segments.length; index++) {
    const segment = segments[index];
    const segmentEnd = segmentStart + segment.length;
    if (relative >= segmentStart && relative < segmentEnd) {
      return { token: segment, full, segmentIndex: index };
    }
    segmentStart = segmentEnd + 1;
  }
  return { token: full, full, segmentIndex: 0 };
}

async function hoverTextForPosition(
  document: TextDocument,
  position: Position,
  tokenInfo: TokenInfo
): Promise<string | undefined> {
  const text = document.getText();
  const localDefinitions = parseComponentDefinitions(text, document.uri);

  const componentProp = await componentPropHover(document, position, tokenInfo, localDefinitions);
  if (componentProp !== undefined) {
    return componentProp;
  }

  const structs = await loadStructFieldIndex(document.uri);

  const typedValue = await typedValueHover(document, position, tokenInfo, localDefinitions, structs);
  if (typedValue !== undefined) {
    return typedValue;
  }

  const componentSignature = localDefinitions.get(tokenInfo.token);
  if (componentSignature !== undefined) {
    return formatComponentHover(componentSignature);
  }

  const importedDefinition = await importedComponentDefinition(document.uri, text, tokenInfo.full);
  if (importedDefinition !== undefined) {
    return formatComponentHover(importedDefinition);
  }

  const typeDoc = typeHover(tokenInfo, structs);
  if (typeDoc !== undefined) {
    return typeDoc;
  }

  const normalized = normalizeHoverToken(tokenInfo.token);
  if (tokenInfo.token === normalized && normalized in HOVER_DOCS) {
    return `**${normalized}**\n\n${HOVER_DOCS[normalized]}`;
  }

  return undefined;
}

function parseComponentDefinitions(text: string, sourceUri?: string): Map<string, ComponentDefinition> {
  const ordered: ComponentDefinition[] = [];
  const lines = text.split(/\r?\n/);
  for (let index = 0; index < lines.length; index++) {
    const line = lines[index];
    const match = /^(\s*)component\s+([A-Z][A-Za-z0-9_]*)\s*\((.*)\)\s*\{\s*$/.exec(line);
    if (match === null) {
      continue;
    }
    const nameStartCharacter = line.indexOf(match[2], match[1].length + 'component'.length);
    const paramsStartCharacter = line.indexOf('(', nameStartCharacter) + 1;
    const params = parseParams(match[3], paramsStartCharacter);
    ordered.push({
      name: match[2],
      signature: `${match[2]}(${match[3].trim()})`,
      params,
      paramsByName: new Map(params.map((param) => [param.name, param.type])),
      sourceUri,
      startLine: index,
      startCharacter: match[1].length,
      endLine: lines.length - 1,
      endCharacter: 0,
      nameStartCharacter,
      nameEndCharacter: nameStartCharacter + match[2].length
    });
  }
  for (let index = 0; index < ordered.length; index++) {
    ordered[index].endLine = index + 1 < ordered.length ? ordered[index + 1].startLine - 1 : lines.length - 1;
    ordered[index].endCharacter = lines[ordered[index].endLine]?.length ?? 0;
  }
  return new Map(ordered.map((definition) => [definition.name, definition]));
}

function parseParams(text: string, baseCharacter: number): ComponentParam[] {
  const params: ComponentParam[] = [];
  for (const segment of splitTopLevelSegments(text)) {
    const trimmedStart = segment.text.search(/\S/);
    if (trimmedStart < 0) {
      continue;
    }
    const trimmed = segment.text.slice(trimmedStart).trimEnd();
    if (trimmed === '') {
      continue;
    }
    const match = /^([A-Za-z_][A-Za-z0-9_]*)\s+(.+)$/.exec(trimmed);
    if (match !== null) {
      const nameStartCharacter = baseCharacter + segment.start + trimmedStart;
      params.push({
        name: match[1],
        type: match[2].trim(),
        nameStartCharacter,
        nameEndCharacter: nameStartCharacter + match[1].length
      });
    }
  }
  return params;
}

function slotOutletSymbols(document: TextDocument, definition: ComponentDefinition): DocumentSymbol[] {
  const startOffset = document.offsetAt({ line: definition.startLine, character: 0 });
  const endOffset = document.offsetAt({ line: definition.endLine, character: definition.endCharacter });
  const body = document.getText().slice(startOffset, endOffset);
  const symbols: DocumentSymbol[] = [];
  for (const match of body.matchAll(/<slot\b([^>]*)\/?>/g)) {
    const attrs = match[1] ?? '';
    const name = /\bname="([^"]+)"/.exec(attrs)?.[1];
    const absoluteStart = startOffset + (match.index ?? 0) + 1;
    const absoluteEnd = absoluteStart + 'slot'.length;
    const range = offsetRange(document, absoluteStart, absoluteEnd);
    symbols.push(DocumentSymbol.create(
      name === undefined ? 'slot' : `slot:${name}`,
      name === undefined ? 'default slot outlet' : 'named slot outlet',
      SymbolKind.Field,
      range,
      range
    ));
  }
  return symbols;
}

function componentRange(definition: ComponentDefinition): Range {
  return {
    start: { line: definition.startLine, character: definition.startCharacter },
    end: { line: definition.endLine, character: definition.endCharacter }
  };
}

function componentSelectionRange(definition: ComponentDefinition): Range {
  return {
    start: { line: definition.startLine, character: definition.nameStartCharacter },
    end: { line: definition.startLine, character: definition.nameEndCharacter }
  };
}

function offsetRange(document: TextDocument, startOffset: number, endOffset: number): Range {
  return {
    start: document.positionAt(startOffset),
    end: document.positionAt(endOffset)
  };
}

function embeddedExpressionAtPosition(
  document: TextDocument,
  position: Position
): { text: string; cursorOffset: number } | undefined {
  const lineText = document.getText().split(/\r?\n/)[position.line] ?? '';
  const regions: Array<{ start: number; end: number }> = [];
  const stack: number[] = [];
  let quote: '"' | '\'' | '`' | undefined;
  for (let index = 0; index < lineText.length; index++) {
    const ch = lineText[index];
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
      stack.push(index);
      continue;
    }
    if (ch === '}' && stack.length > 0) {
      regions.push({ start: stack.pop()!, end: index });
    }
  }
  while (stack.length > 0) {
    regions.push({ start: stack.pop()!, end: lineText.length });
  }
  const region = regions
    .filter((candidate) => position.character > candidate.start && position.character <= candidate.end)
    .sort((a, b) => b.start - a.start)[0];
  if (region === undefined) {
    return undefined;
  }
  return {
    text: lineText.slice(region.start + 1, region.end),
    cursorOffset: Math.max(0, position.character - (region.start + 1))
  };
}

function headerBeforeFirstComponent(text: string): string {
  const lines: string[] = [];
  for (const line of text.split(/\r?\n/)) {
    if (line.trim().startsWith('component ')) {
      break;
    }
    lines.push(line);
  }
  return lines.join('\n').trimEnd();
}

async function probeOverlayBase(uri: string): Promise<{ uri: string; text: string } | undefined> {
  const dir = path.dirname(uriToPath(uri));
  try {
    const entries = await fs.readdir(dir, { withFileTypes: true });
    const candidates = entries
      .filter((entry) => entry.isFile() && entry.name.endsWith('.go') && !entry.name.endsWith('_test.go'))
      .sort((left, right) => {
        const leftScore = overlayCandidateScore(left.name);
        const rightScore = overlayCandidateScore(right.name);
        if (leftScore !== rightScore) {
          return leftScore - rightScore;
        }
        return left.name.localeCompare(right.name);
      });
    const entry = candidates[0];
    if (entry === undefined) {
      return undefined;
    }
    const filePath = path.join(dir, entry.name);
    return {
      uri: pathToFileURL(filePath).toString(),
      text: await fs.readFile(filePath, 'utf8')
    };
  } catch {
    return undefined;
  }
}

function overlayCandidateScore(name: string): number {
  if (!name.endsWith('.go')) {
    return 3;
  }
  if (name.endsWith('.gsx.go')) {
    return 2;
  }
  return 1;
}

let syntheticProbeCounter = 0;

function syntheticProbeURI(baseUri: string): string {
  const dir = path.dirname(uriToPath(baseUri));
  const name = `__gsx_gopls_probe_${process.pid}_${syntheticProbeCounter++}.go`;
  return pathToFileURL(path.join(dir, name)).toString();
}

function completionReplacementRange(document: TextDocument, position: Position): Range {
  const text = document.getText();
  const offset = document.offsetAt(position);
  let start = offset;
  let end = offset;
  while (start > 0 && /[A-Za-z0-9_]/.test(text[start - 1])) {
    start--;
  }
  while (end < text.length && /[A-Za-z0-9_]/.test(text[end])) {
    end++;
  }
  return offsetRange(document, start, end);
}

async function resolveComponentTarget(
  document: TextDocument,
  position: Position
): Promise<ComponentTarget | undefined> {
  const text = document.getText();
  const definitions = parseComponentDefinitions(text, document.uri);
  const localImportPath = await packageImportPathForURI(document.uri);
  if (localImportPath === undefined) {
    return undefined;
  }
  const declaration = componentDefinitionAtPosition(definitions, position);
  if (declaration !== undefined) {
    return {
      importPath: localImportPath,
      name: declaration.name,
      originRange: componentSelectionRange(declaration)
    };
  }
  const tag = componentTagNameContext(document, position);
  if (tag === undefined || !isComponentReference(tag.tagName)) {
    return undefined;
  }
  if (tag.tagName.includes('.')) {
    const [alias, name] = tag.tagName.split('.', 2);
    const importPath = parseImports(text).get(alias);
    if (importPath === undefined) {
      return undefined;
    }
    return {
      importPath,
      name,
      originRange: offsetRange(document, tag.nameStartOffset, tag.nameEndOffset)
    };
  }
  return {
    importPath: localImportPath,
    name: tag.tagName,
    originRange: offsetRange(document, tag.nameStartOffset, tag.nameEndOffset)
  };
}

function componentDefinitionAtPosition(
  definitions: Map<string, ComponentDefinition>,
  position: Position
): ComponentDefinition | undefined {
  for (const definition of definitions.values()) {
    if (position.line !== definition.startLine) {
      continue;
    }
    if (position.character >= definition.nameStartCharacter && position.character <= definition.nameEndCharacter) {
      return definition;
    }
  }
  return undefined;
}

function formatComponentHover(definition: ComponentDefinition): string {
  return `**component ${definition.name}**\n\nSignature: \`${definition.signature}\``;
}

function normalizeHoverToken(token: string): string {
  return token.toLowerCase();
}

function isTokenChar(ch: string): boolean {
  return /[A-Za-z0-9_.:-]/.test(ch);
}

function uriToPath(uri: string): string {
  return fileURLToPath(uri);
}

function isComponentReference(tagName: string): boolean {
  return /^[A-Z][A-Za-z0-9_]*$/.test(tagName) || /^[A-Za-z_][A-Za-z0-9_]*\.[A-Z][A-Za-z0-9_]*$/.test(tagName);
}

function componentTagNameContext(document: TextDocument, position: Position): TagNameContext | undefined {
  const text = document.getText();
  const offset = document.offsetAt(position);
  const windowStart = Math.max(0, offset - 2048);
  const before = text.slice(windowStart, offset);
  const lt = before.lastIndexOf('<');
  const gt = before.lastIndexOf('>');
  if (lt <= gt) {
    return undefined;
  }
  const absoluteLt = windowStart + lt;
  const probe = text.slice(absoluteLt, Math.min(text.length, absoluteLt + 256));
  const match = /^(<\/?)([A-Za-z_][A-Za-z0-9_.:-]*)/.exec(probe);
  if (match === null) {
    return undefined;
  }
  const tagStartOffset = absoluteLt + match[1].length;
  const dot = match[2].lastIndexOf('.');
  const nameStartOffset = dot >= 0 ? tagStartOffset + dot + 1 : tagStartOffset;
  const nameEndOffset = tagStartOffset + match[2].length;
  if (offset < nameStartOffset || offset > nameEndOffset) {
    return undefined;
  }
  return { tagName: match[2], nameStartOffset, nameEndOffset };
}

function openComponentTagContextAt(document: TextDocument, position: Position): OpenComponentTagContext | undefined {
  const text = document.getText();
  const offset = document.offsetAt(position);
  const windowStart = Math.max(0, offset - 4096);
  const before = text.slice(windowStart, offset);
  const lt = before.lastIndexOf('<');
  const gt = before.lastIndexOf('>');
  if (lt <= gt) {
    return undefined;
  }
  const absoluteLt = windowStart + lt;
  const probe = text.slice(absoluteLt, offset);
  if (probe.startsWith('</')) {
    return undefined;
  }
  const match = /^<([A-Za-z_][A-Za-z0-9_.:-]*)(.*)$/.exec(probe);
  if (match === null) {
    return undefined;
  }
  return {
    tagName: match[1],
    nameStartOffset: absoluteLt + 1,
    nameEndOffset: absoluteLt + 1 + match[1].length,
    attrsTailBeforeCursor: match[2]
  };
}

function activeComponentParameter(
  definition: ComponentDefinition,
  attrsTailBeforeCursor: string,
  cursorOffset: number,
  attrsBaseOffset: number
): number {
  if (definition.params.length === 0) {
    return 0;
  }
  const attrs = parseTagAttributes(attrsTailBeforeCursor, attrsBaseOffset);
  for (let index = attrs.length - 1; index >= 0; index--) {
    const attr = attrs[index];
    if (cursorOffset >= attr.startOffset && cursorOffset <= attr.spanEndOffset) {
      const paramIndex = definition.params.findIndex((param) => param.name === attr.name);
      if (paramIndex >= 0) {
        return paramIndex;
      }
    }
  }
  const used = new Set(attrs.map((attr) => attr.name));
  for (let index = 0; index < definition.params.length; index++) {
    if (!used.has(definition.params[index].name)) {
      return index;
    }
  }
  const lastUsed = attrs.length > 0
    ? definition.params.findIndex((param) => param.name === attrs[attrs.length - 1].name)
    : -1;
  return lastUsed >= 0 ? lastUsed : 0;
}

function parseTagAttributes(tail: string, baseOffset: number): TagAttributeOccurrence[] {
  const attrs: TagAttributeOccurrence[] = [];
  let index = 0;
  while (index < tail.length) {
    index = skipSpaces(tail, index);
    if (index >= tail.length || tail[index] === '/' || tail[index] === '>') {
      break;
    }
    const nameStart = index;
    while (index < tail.length && isAttributeTokenChar(tail[index])) {
      index++;
    }
    if (nameStart === index) {
      index++;
      continue;
    }
    const name = tail.slice(nameStart, index);
    const occurrence: TagAttributeOccurrence = {
      name,
      startOffset: baseOffset + nameStart,
      endOffset: baseOffset + index,
      spanEndOffset: baseOffset + index
    };
    index = skipSpaces(tail, index);
    if (tail[index] !== '=') {
      attrs.push(occurrence);
      continue;
    }
    index++;
    index = skipSpaces(tail, index);
    if (index >= tail.length) {
      occurrence.spanEndOffset = baseOffset + index;
      attrs.push(occurrence);
      break;
    }
    if (tail[index] === '"' || tail[index] === '\'') {
      const quote = tail[index];
      const valueStart = index + 1;
      const valueEnd = findQuotedValueEnd(tail, index);
      occurrence.value = tail.slice(valueStart, valueEnd);
      occurrence.valueRange = {
        startOffset: baseOffset + valueStart,
        endOffset: baseOffset + valueEnd
      };
      index = valueEnd < tail.length && tail[valueEnd] === quote ? valueEnd + 1 : valueEnd;
    } else if (tail[index] === '{') {
      index = skipBracedValue(tail, index);
    } else {
      while (index < tail.length && !/\s|\/|>/.test(tail[index])) {
        index++;
      }
    }
    occurrence.spanEndOffset = baseOffset + index;
    attrs.push(occurrence);
  }
  return attrs;
}

function parseOpenTag(
  text: string,
  startOffset: number
): { tagName: string; attrs: TagAttributeOccurrence[]; selfClosing: boolean; nextOffset: number } | undefined {
  const probe = text.slice(startOffset, Math.min(text.length, startOffset + 512));
  const match = /^<([A-Za-z_][A-Za-z0-9_.:-]*)/.exec(probe);
  if (match === null) {
    return undefined;
  }
  const tagName = match[1];
  const tagCloseOffset = findTagCloseOffset(text, startOffset);
  if (tagCloseOffset === undefined) {
    return undefined;
  }
  const attrsStart = startOffset + 1 + tagName.length;
  const attrsEnd = trailingTagSlashOffset(text, attrsStart, tagCloseOffset);
  const attrs = parseTagAttributes(text.slice(attrsStart, attrsEnd), attrsStart);
  return {
    tagName,
    attrs,
    selfClosing: trailingNonSpaceOffset(text, tagCloseOffset - 1) >= 0 && text[trailingNonSpaceOffset(text, tagCloseOffset - 1)] === '/',
    nextOffset: tagCloseOffset + 1
  };
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

function parseClosingTag(
  text: string,
  startOffset: number
): { tagName: string; nextOffset: number } | undefined {
  const probe = text.slice(startOffset, Math.min(text.length, startOffset + 256));
  const match = /^<\/([A-Za-z_][A-Za-z0-9_.:-]*)/.exec(probe);
  if (match === null) {
    return undefined;
  }
  const tagCloseOffset = findTagCloseOffset(text, startOffset);
  if (tagCloseOffset === undefined) {
    return undefined;
  }
  return {
    tagName: match[1],
    nextOffset: tagCloseOffset + 1
  };
}

function skipSpaces(text: string, index: number): number {
  while (index < text.length && /\s/.test(text[index])) {
    index++;
  }
  return index;
}

function skipQuotedValue(text: string, index: number): number {
  const end = findQuotedValueEnd(text, index);
  if (end < text.length && text[end] === text[index]) {
    return end + 1;
  }
  return end;
}

function skipBracedValue(text: string, index: number): number {
  let depth = 0;
  while (index < text.length) {
    const ch = text[index];
    if (ch === '"' || ch === '\'' || ch === '`') {
      index = skipQuotedValue(text, index);
      continue;
    }
    if (ch === '{') {
      depth++;
    } else if (ch === '}') {
      depth--;
      if (depth === 0) {
        return index + 1;
      }
    }
    index++;
  }
  return index;
}

function findQuotedValueEnd(text: string, index: number): number {
  const quote = text[index];
  index++;
  while (index < text.length) {
    if (text[index] === '\\') {
      index += 2;
      continue;
    }
    if (text[index] === quote) {
      return index;
    }
    index++;
  }
  return index;
}

function trailingNonSpaceOffset(text: string, index: number): number {
  while (index >= 0 && /\s/.test(text[index])) {
    index--;
  }
  return index;
}

function trailingTagSlashOffset(text: string, attrsStart: number, tagCloseOffset: number): number {
  const candidate = trailingNonSpaceOffset(text, tagCloseOffset - 1);
  if (candidate >= attrsStart && text[candidate] === '/') {
    return candidate;
  }
  return tagCloseOffset;
}

function popTagFrame(stack: TagFrame[], tagName: string): void {
  for (let index = stack.length - 1; index >= 0; index--) {
    if (stack[index].tagName === tagName) {
      stack.splice(index, 1);
      return;
    }
  }
}

function resolveTagFrame(snapshot: GSXFileSnapshot, tagName: string): TagFrame {
  if (!isComponentReference(tagName)) {
    return { tagName, isComponent: false };
  }
  if (tagName.includes('.')) {
    const [alias, componentName] = tagName.split('.', 2);
    const importPath = snapshot.imports.get(alias);
    if (importPath !== undefined) {
      return {
        tagName,
        isComponent: true,
        componentImportPath: importPath,
        componentName
      };
    }
  }
  return {
    tagName,
    isComponent: true,
    componentImportPath: snapshot.importPath,
    componentName: tagName
  };
}

async function importedComponentDefinition(uri: string, text: string, fullToken: string): Promise<ComponentDefinition | undefined> {
  const imports = parseImports(text);
  if (fullToken.includes('.')) {
    const [alias, name] = fullToken.split('.', 2);
    const importPath = imports.get(alias);
    if (importPath === undefined) {
      return undefined;
    }
    const definitions = await importedComponentDefinitions(uri, importPath);
    return definitions.get(name);
  }
  return undefined;
}

async function importedComponentDefinitions(uri: string, importPath: string): Promise<Map<string, ComponentDefinition>> {
  const module = await findNearestModule(uriToPath(uri));
  const out = new Map<string, ComponentDefinition>();
  if (module === undefined) {
    return out;
  }
  if (!(importPath === module.modulePath || importPath.startsWith(module.modulePath + '/'))) {
    return out;
  }
  const rel = importPath === module.modulePath ? '' : importPath.slice(module.modulePath.length + 1);
  const targetDir = path.join(module.rootDir, ...rel.split('/').filter(Boolean));
  try {
    const entries = await fs.readdir(targetDir, { withFileTypes: true });
    for (const entry of entries) {
      if (!entry.isFile() || !entry.name.endsWith('.gsx')) {
        continue;
      }
      const source = await fs.readFile(path.join(targetDir, entry.name), 'utf8');
      for (const definition of parseComponentDefinitions(source, pathToFileURL(path.join(targetDir, entry.name)).toString()).values()) {
        out.set(definition.name, definition);
      }
    }
  } catch {
    return out;
  }
  return out;
}

async function componentOccurrences(
  uri: string,
  target: ComponentTarget
): Promise<Array<{ document: TextDocument; range: Range; isDeclaration: boolean }>> {
  const snapshots = await scanModuleGSXFiles(uri);
  const occurrences: Array<{ document: TextDocument; range: Range; isDeclaration: boolean }> = [];
  for (const snapshot of snapshots) {
    const definition = snapshot.importPath === target.importPath ? snapshot.definitions.get(target.name) : undefined;
    if (definition !== undefined) {
      occurrences.push({
        document: snapshot.document,
        range: componentSelectionRange(definition),
        isDeclaration: true
      });
    }
    for (const reference of componentTagOccurrences(snapshot)) {
      if (reference.importPath === target.importPath && reference.name === target.name) {
        occurrences.push({
          document: snapshot.document,
          range: reference.range,
          isDeclaration: false
        });
      }
    }
  }
  return occurrences;
}

async function resolveSlotTarget(
  document: TextDocument,
  position: Position
): Promise<SlotTarget | undefined> {
  const importPath = await packageImportPathForURI(document.uri);
  if (importPath === undefined) {
    return undefined;
  }
  const snapshot: GSXFileSnapshot = {
    document,
    importPath,
    imports: parseImports(document.getText()),
    definitions: parseComponentDefinitions(document.getText(), document.uri)
  };
  const offset = document.offsetAt(position);
  for (const occurrence of slotOccurrencesInSnapshot(snapshot)) {
    const start = document.offsetAt(occurrence.range.start);
    const end = document.offsetAt(occurrence.range.end);
    if (offset < start || offset > end) {
      continue;
    }
    return {
      componentImportPath: occurrence.componentImportPath,
      componentName: occurrence.componentName,
      slotName: occurrence.slotName,
      originRange: occurrence.range
    };
  }
  return undefined;
}

async function slotDeclarationOccurrences(
  uri: string,
  target: SlotTarget
): Promise<Array<{ document: TextDocument; range: Range }>> {
  const snapshots = await scanModuleGSXFiles(uri);
  const occurrences: Array<{ document: TextDocument; range: Range }> = [];
  for (const snapshot of snapshots) {
    for (const occurrence of slotOccurrencesInSnapshot(snapshot)) {
      if (
        occurrence.isDeclaration &&
        occurrence.componentImportPath === target.componentImportPath &&
        occurrence.componentName === target.componentName &&
        occurrence.slotName === target.slotName
      ) {
        occurrences.push({ document: snapshot.document, range: occurrence.range });
      }
    }
  }
  return occurrences;
}

async function slotOccurrences(
  uri: string,
  target: SlotTarget
): Promise<Array<{ document: TextDocument; range: Range; isDeclaration: boolean }>> {
  const snapshots = await scanModuleGSXFiles(uri);
  const occurrences: Array<{ document: TextDocument; range: Range; isDeclaration: boolean }> = [];
  for (const snapshot of snapshots) {
    for (const occurrence of slotOccurrencesInSnapshot(snapshot)) {
      if (
        occurrence.componentImportPath === target.componentImportPath &&
        occurrence.componentName === target.componentName &&
        occurrence.slotName === target.slotName
      ) {
        occurrences.push({
          document: snapshot.document,
          range: occurrence.range,
          isDeclaration: occurrence.isDeclaration
        });
      }
    }
  }
  return occurrences;
}

function slotOccurrencesInSnapshot(snapshot: GSXFileSnapshot): SlotOccurrence[] {
  const text = snapshot.document.getText();
  const definitions = [...snapshot.definitions.values()].sort((a, b) => a.startLine - b.startLine);
  const occurrences: SlotOccurrence[] = [];
  for (const definition of definitions) {
    const startOffset = snapshot.document.offsetAt({ line: definition.startLine, character: 0 });
    const endOffset = snapshot.document.offsetAt({ line: definition.endLine, character: definition.endCharacter });
    const stack: TagFrame[] = [];
    let index = startOffset;
    while (index < endOffset) {
      if (text.startsWith('<!--', index)) {
        const commentEnd = text.indexOf('-->', index + 4);
        index = commentEnd >= 0 ? commentEnd + 3 : endOffset;
        continue;
      }
      if (text[index] !== '<') {
        index++;
        continue;
      }
      if (text.startsWith('</', index)) {
        const close = parseClosingTag(text, index);
        if (close === undefined) {
          index++;
          continue;
        }
        popTagFrame(stack, close.tagName);
        index = close.nextOffset;
        continue;
      }
      if (text.startsWith('<!', index)) {
        const end = findTagCloseOffset(text, index);
        index = end === undefined ? endOffset : end + 1;
        continue;
      }
      const tag = parseOpenTag(text, index);
      if (tag === undefined) {
        index++;
        continue;
      }
      if (tag.tagName === 'slot') {
        const nameAttr = tag.attrs.find((attr) => attr.name === 'name' && attr.value !== undefined && attr.valueRange !== undefined);
        if (nameAttr?.value !== undefined && nameAttr.valueRange !== undefined) {
          occurrences.push({
            componentImportPath: snapshot.importPath,
            componentName: definition.name,
            slotName: nameAttr.value,
            range: offsetRange(snapshot.document, nameAttr.valueRange.startOffset, nameAttr.valueRange.endOffset),
            isDeclaration: true
          });
        }
      }
      const slotAttr = tag.attrs.find((attr) => attr.name === 'slot' && attr.value !== undefined && attr.valueRange !== undefined);
      const parent = stack[stack.length - 1];
      if (slotAttr?.value !== undefined && slotAttr.valueRange !== undefined && parent?.isComponent) {
        occurrences.push({
          componentImportPath: parent.componentImportPath ?? snapshot.importPath,
          componentName: parent.componentName ?? parent.tagName,
          slotName: slotAttr.value,
          range: offsetRange(snapshot.document, slotAttr.valueRange.startOffset, slotAttr.valueRange.endOffset),
          isDeclaration: false
        });
      }
      if (!tag.selfClosing) {
        stack.push(resolveTagFrame(snapshot, tag.tagName));
      }
      index = tag.nextOffset;
    }
  }
  return occurrences;
}

function componentTagOccurrences(
  snapshot: GSXFileSnapshot
): Array<{ importPath: string; name: string; range: Range }> {
  const results: Array<{ importPath: string; name: string; range: Range }> = [];
  const text = snapshot.document.getText();
  const regex = /<\/?([A-Za-z_][A-Za-z0-9_.:-]*)/g;
  for (const match of text.matchAll(regex)) {
    const tagName = match[1];
    if (!isComponentReference(tagName)) {
      continue;
    }
    const prefixLength = match[0].startsWith('</') ? 2 : 1;
    const tagStart = (match.index ?? 0) + prefixLength;
    if (tagName.includes('.')) {
      const [alias, name] = tagName.split('.', 2);
      const importPath = snapshot.imports.get(alias);
      if (importPath === undefined) {
        continue;
      }
      const componentStart = tagStart + alias.length + 1;
      results.push({
        importPath,
        name,
        range: offsetRange(snapshot.document, componentStart, componentStart + name.length)
      });
      continue;
    }
    results.push({
      importPath: snapshot.importPath,
      name: tagName,
      range: offsetRange(snapshot.document, tagStart, tagStart + tagName.length)
    });
  }
  return results;
}

async function componentPropHover(
  document: TextDocument,
  position: Position,
  tokenInfo: TokenInfo,
  localDefinitions: Map<string, ComponentDefinition>
): Promise<string | undefined> {
  const offset = document.offsetAt(position);
  const before = document.getText({ start: { line: 0, character: 0 }, end: position });
  const after = document.getText({ start: position, end: document.positionAt(Math.min(document.getText().length, offset + 256)) });
  const tagContext = openTagContext(before);
  if (tagContext === undefined) {
    return undefined;
  }
  const attrName = attributeNameAtOffset(document.getText(), offset);
  if (attrName === undefined || attrName !== tokenInfo.token) {
    return undefined;
  }
  const definition = localDefinitions.get(tagContext.tagName) ??
    await importedComponentDefinition(document.uri, document.getText(), tagContext.tagName);
  if (definition === undefined) {
    return undefined;
  }
  const propType = definition.paramsByName.get(attrName);
  if (propType === undefined) {
    return undefined;
  }
  void after; // keep the bounded read symmetrical for future use
  return `**${attrName}**\n\nType: \`${propType}\`\n\nParameter of component \`${definition.name}\`.`;
}

async function typedValueHover(
  document: TextDocument,
  position: Position,
  tokenInfo: TokenInfo,
  localDefinitions: Map<string, ComponentDefinition>,
  structs: StructFieldIndex
): Promise<string | undefined> {
  const component = componentAtLine(localDefinitions, position.line);
  if (component === undefined) {
    return undefined;
  }
  const bindings = bindingsAtLine(document.getText(), component, position.line, structs);
  if (tokenInfo.segmentIndex === 0) {
    const binding = bindings.get(tokenInfo.token);
    if (binding !== undefined) {
      const label = binding.kind === 'parameter' ? 'Component parameter' : 'Loop variable';
      return `**${tokenInfo.token}**\n\nType: \`${binding.type}\`\n\n${label} in \`${binding.owner}\`.`;
    }
    return undefined;
  }
  const segments = tokenInfo.full.split('.');
  const selectorType = resolveSelectorType(segments.slice(0, tokenInfo.segmentIndex + 1), bindings, structs);
  if (selectorType === undefined) {
    return undefined;
  }
  const parentType = resolveSelectorType(segments.slice(0, tokenInfo.segmentIndex), bindings, structs);
  if (parentType === undefined) {
    return undefined;
  }
  return `**${tokenInfo.token}**\n\nType: \`${selectorType}\`\n\nField of \`${normalizeTypeName(parentType)}\`.`;
}

function typeHover(tokenInfo: TokenInfo, structs: StructFieldIndex): string | undefined {
  if (tokenInfo.segmentIndex !== 0) {
    return undefined;
  }
  const builtin = GO_BUILTIN_TYPE_DOCS[tokenInfo.token];
  if (builtin !== undefined) {
    return `**${tokenInfo.token}**\n\n${builtin}`;
  }
  const fields = structs.get(tokenInfo.token);
  if (fields === undefined) {
    return undefined;
  }
  return formatStructHover(tokenInfo.token, fields);
}

function formatStructHover(typeName: string, fields: Map<string, string>): string {
  if (fields.size === 0) {
    return `**type ${typeName} struct**\n\nStruct type declared in the current Go package.`;
  }
  const lines = [...fields.entries()].map(([name, type]) => `- \`${name}\`: \`${type}\``).join('\n');
  return `**type ${typeName} struct**\n\nFields:\n${lines}`;
}

function componentAtLine(
  definitions: Map<string, ComponentDefinition>,
  line: number
): ComponentDefinition | undefined {
  for (const definition of definitions.values()) {
    if (line >= definition.startLine && line <= definition.endLine) {
      return definition;
    }
  }
  return undefined;
}

function bindingsAtLine(
  text: string,
  component: ComponentDefinition,
  targetLine: number,
  structs: StructFieldIndex
): Map<string, HoverBinding> {
  const lines = text.split(/\r?\n/);
  let depth = 1;
  const scopeStack: Array<{ depth: number; bindings: Map<string, HoverBinding> }> = [];
  const base = new Map<string, HoverBinding>();
  for (const param of component.params) {
    base.set(param.name, { kind: 'parameter', type: param.type, owner: component.name });
  }

  for (let lineIndex = component.startLine + 1; lineIndex <= targetLine && lineIndex < lines.length; lineIndex++) {
    const trimmed = lines[lineIndex].trim();
    if (trimmed === '') {
      continue;
    }
    const loopBindings = inferLoopBindings(trimmed, currentBindings(base, scopeStack), structs, component.name);
    if (loopBindings.size > 0) {
      scopeStack.push({ depth: depth + 1, bindings: loopBindings });
      depth++;
      continue;
    }
    if (opensBlock(trimmed)) {
      depth++;
      continue;
    }
    if (trimmed === '}') {
      depth = Math.max(0, depth - 1);
      while (scopeStack.length > 0 && scopeStack[scopeStack.length - 1].depth > depth) {
        scopeStack.pop();
      }
    }
  }

  return currentBindings(base, scopeStack);
}

function currentBindings(
  base: Map<string, HoverBinding>,
  scopeStack: Array<{ depth: number; bindings: Map<string, HoverBinding> }>
): Map<string, HoverBinding> {
  const merged = new Map(base);
  for (const scope of scopeStack) {
    for (const [name, binding] of scope.bindings) {
      merged.set(name, binding);
    }
  }
  return merged;
}

function inferLoopBindings(
  trimmedLine: string,
  bindings: Map<string, HoverBinding>,
  structs: StructFieldIndex,
  owner: string
): Map<string, HoverBinding> {
  const match = /^for\s+(.+?)\s*:=\s*range\s+(.+?)\s*\{\s*$/.exec(trimmedLine);
  if (match === null) {
    return new Map();
  }
  const lhs = match[1].split(',').map((part) => part.trim()).filter(Boolean);
  const rangeType = resolveExpressionType(match[2].trim(), bindings, structs);
  if (rangeType === undefined) {
    return new Map();
  }
  const bindingTypes = rangeBindingTypes(rangeType, lhs.length);
  const out = new Map<string, HoverBinding>();
  for (let index = 0; index < lhs.length && index < bindingTypes.length; index++) {
    if (lhs[index] === '_' || bindingTypes[index] === undefined) {
      continue;
    }
    out.set(lhs[index], { kind: 'loopVariable', type: bindingTypes[index], owner });
  }
  return out;
}

function rangeBindingTypes(rangeType: string, count: number): string[] {
  const normalized = rangeType.replace(/\s+/g, ' ').trim();
  const mapMatch = /^map\[(.+?)\](.+)$/.exec(normalized);
  if (mapMatch !== null) {
    return count === 1 ? [mapMatch[1].trim()] : [mapMatch[1].trim(), mapMatch[2].trim()];
  }
  const sliceMatch = /^\[(?:[^\]]*)\](.+)$/.exec(normalized);
  if (sliceMatch !== null) {
    return count === 1 ? ['int'] : ['int', sliceMatch[1].trim()];
  }
  if (normalized === 'string') {
    return count === 1 ? ['int'] : ['int', 'rune'];
  }
  const chanMatch = /^(?:<-chan|chan<-|chan)\s+(.+)$/.exec(normalized);
  if (chanMatch !== null) {
    return [chanMatch[1].trim()];
  }
  return [];
}

function resolveExpressionType(
  expr: string,
  bindings: Map<string, HoverBinding>,
  structs: StructFieldIndex
): string | undefined {
  const trimmed = expr.trim();
  if (/^[A-Za-z_][A-Za-z0-9_]*$/.test(trimmed)) {
    return bindings.get(trimmed)?.type;
  }
  if (/^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)+$/.test(trimmed)) {
    return resolveSelectorType(trimmed.split('.'), bindings, structs);
  }
  return undefined;
}

function resolveSelectorType(
  parts: string[],
  bindings: Map<string, HoverBinding>,
  structs: StructFieldIndex
): string | undefined {
  if (parts.length === 0) {
    return undefined;
  }
  let currentType = bindings.get(parts[0])?.type;
  if (currentType === undefined) {
    return undefined;
  }
  for (const field of parts.slice(1)) {
    const fields = structs.get(normalizeTypeName(currentType));
    if (fields === undefined) {
      return undefined;
    }
    currentType = fields.get(field);
    if (currentType === undefined) {
      return undefined;
    }
  }
  return currentType;
}

function normalizeTypeName(typeName: string): string {
  let normalized = typeName.trim();
  while (normalized.startsWith('*')) {
    normalized = normalized.slice(1).trim();
  }
  return normalized;
}

async function loadStructFieldIndex(uri: string): Promise<StructFieldIndex> {
  const dir = path.dirname(uriToPath(uri));
  const index: StructFieldIndex = new Map();
  try {
    const entries = await fs.readdir(dir, { withFileTypes: true });
    for (const entry of entries) {
      if (!entry.isFile() || !entry.name.endsWith('.go')) {
        continue;
      }
      const source = await fs.readFile(path.join(dir, entry.name), 'utf8');
      for (const [typeName, fields] of parseStructs(source)) {
        index.set(typeName, fields);
      }
    }
  } catch {
    return index;
  }
  return index;
}

function parseStructs(source: string): StructFieldIndex {
  const structs: StructFieldIndex = new Map();
  const regex = /type\s+([A-Z][A-Za-z0-9_]*)\s+struct\s*\{([\s\S]*?)^\}/gm;
  for (const match of source.matchAll(regex)) {
    const fields = new Map<string, string>();
    for (const line of match[2].split(/\r?\n/)) {
      const trimmed = line.trim();
      if (trimmed === '' || trimmed.startsWith('//')) {
        continue;
      }
      const fieldMatch = /^([A-Za-z_][A-Za-z0-9_]*)\s+([^`/]+?)(?:\s+`[^`]*`)?$/.exec(trimmed);
      if (fieldMatch !== null) {
        fields.set(fieldMatch[1], fieldMatch[2].trim());
      }
    }
    structs.set(match[1], fields);
  }
  return structs;
}

function attributeNameAtOffset(text: string, offset: number): string | undefined {
  let start = offset;
  let end = offset;
  while (start > 0 && isAttributeTokenChar(text[start - 1])) {
    start--;
  }
  while (end < text.length && isAttributeTokenChar(text[end])) {
    end++;
  }
  if (start === end) {
    return undefined;
  }
  const token = text.slice(start, end);
  return /^[A-Za-z_:][A-Za-z0-9:._-]*$/.test(token) ? token : undefined;
}

function isAttributeTokenChar(ch: string): boolean {
  return /[A-Za-z0-9:._-]/.test(ch);
}

function opensBlock(trimmedLine: string): boolean {
  return /^(?:if|for|else if|else)\b.*\{\s*$/.test(trimmedLine);
}

function splitTopLevel(text: string): string[] {
  return splitTopLevelSegments(text).map((segment) => segment.text);
}

function splitTopLevelSegments(text: string): Array<{ text: string; start: number }> {
  const segments: Array<{ text: string; start: number }> = [];
  let start = 0;
  let parenDepth = 0;
  let bracketDepth = 0;
  let braceDepth = 0;
  let quote: '"' | '\'' | '`' | undefined;
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
    if (ch === ',' && parenDepth === 0 && bracketDepth === 0 && braceDepth === 0) {
      segments.push({ text: text.slice(start, index), start });
      start = index + 1;
    }
  }
  segments.push({ text: text.slice(start), start });
  return segments;
}

function isTagNameContext(before: string): boolean {
  const tail = currentTagTail(before);
  return /^<\/?[A-Za-z_][A-Za-z0-9_.:-]*$/.test(tail) || /^<$/.test(tail);
}

function importedAliasContext(before: string): string | undefined {
  const tail = currentTagTail(before);
  const match = /^<\/?([A-Za-z_][A-Za-z0-9_]*)\.$/.exec(tail);
  return match?.[1];
}

function openTagContext(before: string): { tagName: string; tail: string } | undefined {
  const tail = currentTagTail(before);
  if (!tail.startsWith('<') || tail.startsWith('</')) {
    return undefined;
  }
  const tagMatch = /^<([A-Za-z_][A-Za-z0-9_.:-]*)(.*)$/.exec(tail);
  if (tagMatch === null) {
    return undefined;
  }
  return { tagName: tagMatch[1], tail: tagMatch[2] };
}

function currentTagTail(before: string): string {
  const lt = before.lastIndexOf('<');
  const gt = before.lastIndexOf('>');
  if (lt <= gt) {
    return '';
  }
  return before.slice(lt);
}

function isAttributeContext(context: { tagName: string; tail: string }): boolean {
  if (context.tail.includes('>')) {
    return false;
  }
  if (/=\s*(?:"[^"]*|'[^']*|\{[^}]*$)/.test(context.tail)) {
    return false;
  }
  return /\s+[A-Za-z_:.-]*$/.test(context.tail) || /\s+$/.test(context.tail);
}

function isTopLevelContext(linePrefix: string, before: string): boolean {
  if (/^\s*$/.test(linePrefix)) {
    return true;
  }
  return /(?:^|\n)\s*$/.test(before);
}

function isControlFlowContext(linePrefix: string, before: string): boolean {
  if (isTagNameContext(before)) {
    return false;
  }
  return /^\s*$/.test(linePrefix) || /(?:\{|\n)\s*$/.test(before);
}

function isVoidTag(tag: string): boolean {
  return ['area', 'base', 'br', 'col', 'embed', 'hr', 'img', 'input', 'link', 'meta', 'source'].includes(tag);
}
