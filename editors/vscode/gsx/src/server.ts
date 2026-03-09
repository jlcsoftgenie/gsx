import * as fs from 'node:fs/promises';
import * as os from 'node:os';
import * as path from 'node:path';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { fileURLToPath, pathToFileURL } from 'node:url';
import {
  CodeAction,
  CompletionItem,
  createConnection,
  Diagnostic,
  DiagnosticSeverity,
  DidChangeConfigurationNotification,
  DocumentSymbol,
  DocumentFormattingParams,
  LocationLink,
  Hover,
  InitializeParams,
  InitializeResult,
  Location,
  PrepareRenameResult,
  ProposedFeatures,
  SemanticTokens,
  SignatureHelp,
  TextDocumentSyncKind,
  TextEdit
} from 'vscode-languageserver/node';
import { TextDocument } from 'vscode-languageserver-textdocument';
import { TextDocuments } from 'vscode-languageserver';
import {
  provideCompletionItems,
  provideDefinition,
  provideDocumentSymbols,
  provideHover,
  prepareComponentRename,
  provideReferences,
  provideRenameEdits,
  provideSignatureHelp
} from './gsxFeatures';
import { provideCodeActions } from './codeActions';
import { provideSemanticTokens, semanticTokenLegend } from './semanticTokens';

const execFileAsync = promisify(execFile);
const connection = createConnection(ProposedFeatures.all);
const documents = new TextDocuments(TextDocument);

interface GsxSettings {
  command: string;
  commandArgs: string[];
  checkOnOpen: boolean;
  checkOnSave: boolean;
  diagnosticDebounceMs: number;
  goplsCommand: string;
}

interface RunCheckParams {
  uri: string;
}

interface RunCheckResult {
  message: string;
  hasErrors: boolean;
}

interface CheckExecutionResult {
  diagnosticsByFile: Map<string, Diagnostic[]>;
  hasErrors: boolean;
  commandFailed: boolean;
  errorMessage?: string;
}

interface CLIInvocation {
  command: string;
  argsPrefix: string[];
  cwd: string;
}

let workspaceFolders: string[] = [];
const publishedByRoot = new Map<string, Set<string>>();
const scheduledChecks = new Map<string, NodeJS.Timeout>();

connection.onInitialize((params: InitializeParams): InitializeResult => {
  workspaceFolders = extractWorkspaceFolders(params);
  return {
    capabilities: {
      textDocumentSync: {
        openClose: true,
        change: TextDocumentSyncKind.Incremental,
        save: { includeText: false }
      },
      completionProvider: {
        triggerCharacters: ['<', '/', ' ', '.', '{']
      },
      hoverProvider: true,
      definitionProvider: true,
      referencesProvider: true,
      codeActionProvider: true,
      documentSymbolProvider: true,
      documentFormattingProvider: true,
      renameProvider: {
        prepareProvider: true
      },
      signatureHelpProvider: {
        triggerCharacters: [' ', '='],
        retriggerCharacters: ['{', '"', '\'']
      },
      semanticTokensProvider: {
        legend: semanticTokenLegend,
        full: true
      }
    }
  };
});

connection.onInitialized(() => {
  void connection.client.register(DidChangeConfigurationNotification.type);
});

connection.onCompletion(async (params): Promise<CompletionItem[] | undefined> => {
  const document = documents.get(params.textDocument.uri);
  if (document === undefined) {
    return undefined;
  }
  const settings = await getSettings(document.uri);
  return provideCompletionItems(document, params.position, { goplsCommand: settings.goplsCommand });
});

connection.onHover(async (params): Promise<Hover | null> => {
  const document = documents.get(params.textDocument.uri);
  if (document === undefined) {
    return null;
  }
  const settings = await getSettings(document.uri);
  return provideHover(document, params.position, { goplsCommand: settings.goplsCommand });
});

connection.onDocumentSymbol((params): DocumentSymbol[] => {
  const document = documents.get(params.textDocument.uri);
  if (document === undefined) {
    return [];
  }
  return provideDocumentSymbols(document);
});

connection.onDefinition(async (params): Promise<LocationLink[] | null> => {
  const document = documents.get(params.textDocument.uri);
  if (document === undefined) {
    return null;
  }
  return provideDefinition(document, params.position);
});

connection.onReferences(async (params): Promise<Location[] | null> => {
  const document = documents.get(params.textDocument.uri);
  if (document === undefined) {
    return null;
  }
  return provideReferences(document, params.position, params.context.includeDeclaration);
});

connection.onPrepareRename(async (params): Promise<PrepareRenameResult | null> => {
  const document = documents.get(params.textDocument.uri);
  if (document === undefined) {
    return null;
  }
  return prepareComponentRename(document, params.position);
});

connection.onRenameRequest(async (params) => {
  const document = documents.get(params.textDocument.uri);
  if (document === undefined) {
    return null;
  }
  return provideRenameEdits(document, params.position, params.newName);
});

connection.onCodeAction(async (params): Promise<CodeAction[]> => {
  const document = documents.get(params.textDocument.uri);
  if (document === undefined) {
    return [];
  }
  let formattedText: string | undefined;
  if (params.context.diagnostics.some((diagnostic) => diagnostic.code === 'L007')) {
    formattedText = await formatWithCLI(document);
  }
  return provideCodeActions(document, params.context.diagnostics, formattedText);
});

connection.onSignatureHelp(async (params): Promise<SignatureHelp | null> => {
  const document = documents.get(params.textDocument.uri);
  if (document === undefined) {
    return null;
  }
  return provideSignatureHelp(document, params.position);
});

connection.onDocumentFormatting(async (params: DocumentFormattingParams): Promise<TextEdit[]> => {
  const document = documents.get(params.textDocument.uri);
  if (document === undefined) {
    return [];
  }
  const formatted = await formatWithCLI(document);
  if (formatted === undefined || formatted === document.getText()) {
    return [];
  }
  return [TextEdit.replace(fullDocumentRange(document), formatted)];
});

connection.languages.semanticTokens.on((params): SemanticTokens => {
  const document = documents.get(params.textDocument.uri);
  if (document === undefined) {
    return { data: [] };
  }
  return provideSemanticTokens(document);
});

documents.onDidOpen(async (event) => {
  const settings = await getSettings(event.document.uri);
  if (settings.checkOnOpen) {
    scheduleCheck(event.document.uri);
  }
});

documents.onDidSave(async (event) => {
  const settings = await getSettings(event.document.uri);
  if (settings.checkOnSave) {
    scheduleCheck(event.document.uri);
  }
});

documents.onDidClose((event) => {
  connection.sendDiagnostics({ uri: event.document.uri, diagnostics: [] });
});

connection.onDidChangeConfiguration(() => {
  for (const root of scheduledChecks.keys()) {
    clearScheduled(root);
  }
  for (const document of documents.all()) {
    scheduleCheck(document.uri);
  }
});

connection.onRequest('gsx/runCheck', async (params: RunCheckParams): Promise<RunCheckResult> => {
  const root = await workspaceRootForURI(params.uri);
  const result = await runCheck(root);
  if (!result.commandFailed) {
    publishDiagnostics(root, result.diagnosticsByFile);
  }
  if (result.commandFailed) {
    return {
      message: result.errorMessage ?? `GSX check failed for ${path.basename(root)}.`,
      hasErrors: true
    };
  }
  const counts = countDiagnostics(result.diagnosticsByFile);
  return {
    message: result.hasErrors || counts.warnings > 0
      ? `GSX check found ${counts.errors} error(s) and ${counts.warnings} warning(s) in ${path.basename(root)}.`
      : `GSX check completed successfully for ${path.basename(root)}.`,
    hasErrors: result.hasErrors
  };
});

function scheduleCheck(uri: string): void {
  void (async () => {
    const root = await workspaceRootForURI(uri);
    clearScheduled(root);
    const settings = await getSettings(uri);
    const timer = setTimeout(() => {
      void refreshDiagnostics(root);
    }, settings.diagnosticDebounceMs);
    scheduledChecks.set(root, timer);
  })();
}

async function refreshDiagnostics(root: string): Promise<void> {
  clearScheduled(root);
  const result = await runCheck(root);
  if (!result.commandFailed) {
    publishDiagnostics(root, result.diagnosticsByFile);
  }
}

function publishDiagnostics(root: string, byFile: Map<string, Diagnostic[]>): void {
  const previous = publishedByRoot.get(root) ?? new Set<string>();
  const current = new Set<string>();
  for (const [filePath, diagnostics] of byFile) {
    const uri = pathToFileURL(filePath).toString();
    connection.sendDiagnostics({ uri, diagnostics });
    current.add(uri);
  }
  for (const uri of previous) {
    if (!current.has(uri)) {
      connection.sendDiagnostics({ uri, diagnostics: [] });
    }
  }
  publishedByRoot.set(root, current);
}

async function runCheck(root: string): Promise<CheckExecutionResult> {
  const invocation = await resolveCLIInvocation(pathToFileURL(root).toString(), root);
  const args = [...invocation.argsPrefix, 'check', root];
  let stdout = '';
  let stderr = '';
  try {
    const result = await execFileAsync(invocation.command, args, { cwd: invocation.cwd, maxBuffer: 8 * 1024 * 1024 });
    stdout = result.stdout;
    stderr = result.stderr;
  } catch (err) {
    const execErr = err as { stdout?: string; stderr?: string; code?: number | string; message?: string };
    if (typeof execErr.code === 'string') {
      return {
        diagnosticsByFile: new Map(),
        hasErrors: true,
        commandFailed: true,
        errorMessage: buildCLIErrorMessage(invocation, execErr)
      };
    }
    stdout = execErr.stdout ?? '';
    stderr = execErr.stderr ?? '';
  }
  const diagnosticsByFile = parseCheckDiagnostics(stdout + stderr);
  let hasErrors = false;
  for (const diagnostics of diagnosticsByFile.values()) {
    if (diagnostics.some((diag) => diag.severity === DiagnosticSeverity.Error)) {
      hasErrors = true;
      break;
    }
  }
  return { diagnosticsByFile, hasErrors, commandFailed: false };
}

async function formatWithCLI(document: TextDocument): Promise<string | undefined> {
  const root = await workspaceRootForURI(document.uri);
  const invocation = await resolveCLIInvocation(document.uri, root);
  const tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'gsx-format-'));
  const fileName = path.basename(fileURLToPath(document.uri));
  const filePath = path.join(tmpDir, fileName);
  try {
    await fs.writeFile(filePath, document.getText(), 'utf8');
    const args = [...invocation.argsPrefix, 'fmt', '--write', filePath];
    await execFileAsync(invocation.command, args, { cwd: invocation.cwd, maxBuffer: 8 * 1024 * 1024 });
    return await fs.readFile(filePath, 'utf8');
  } catch (err) {
    const execErr = err as { stdout?: string; stderr?: string; code?: number | string; message?: string };
    connection.window.showErrorMessage(buildCLIErrorMessage(invocation, execErr, 'formatting failed'));
    return undefined;
  } finally {
    await fs.rm(tmpDir, { recursive: true, force: true });
  }
}

function parseCheckDiagnostics(output: string): Map<string, Diagnostic[]> {
  const byFile = new Map<string, Diagnostic[]>();
  const lines = output.split(/\r?\n/);
  for (const line of lines) {
    if (line.trim() === '' || line === 'check failed' || line === 'generation failed' || line.startsWith('exit status ')) {
      continue;
    }
    if (/^\s+\d+\s+\|/.test(line) || /^\s+\|/.test(line) || line.trim().startsWith('note:')) {
      continue;
    }
    const match = /^(.*?):(\d+):(\d+):\s+(.+?)(?:\s+\[([A-Z]\d+)\])?$/.exec(line);
    if (match === null) {
      continue;
    }
    const filePath = match[1];
    const lineNo = Number(match[2]);
    const colNo = Number(match[3]);
    const message = match[4];
    const code = match[5] ?? '';
    const severity = code.startsWith('L') ? DiagnosticSeverity.Warning : DiagnosticSeverity.Error;
    const diagnostic: Diagnostic = {
      severity,
      message,
      code,
      source: 'gsx',
      range: {
        start: { line: Math.max(0, lineNo - 1), character: Math.max(0, colNo - 1) },
        end: { line: Math.max(0, lineNo - 1), character: Math.max(0, colNo) }
      }
    };
    const items = byFile.get(filePath) ?? [];
    items.push(diagnostic);
    byFile.set(filePath, items);
  }
  return byFile;
}

async function getSettings(scopeUri: string): Promise<GsxSettings> {
  const settings = await connection.workspace.getConfiguration({ scopeUri, section: 'gsx' }) as Partial<GsxSettings> | null;
  return {
    command: typeof settings?.command === 'string' ? settings.command : '',
    commandArgs: Array.isArray(settings?.commandArgs)
      ? settings.commandArgs.filter((arg): arg is string => typeof arg === 'string')
      : [],
    checkOnOpen: settings?.checkOnOpen ?? true,
    checkOnSave: settings?.checkOnSave ?? true,
    diagnosticDebounceMs: settings?.diagnosticDebounceMs ?? 350,
    goplsCommand: typeof settings?.goplsCommand === 'string' ? settings.goplsCommand : ''
  };
}

async function resolveCLIInvocation(scopeUri: string, workingRoot: string): Promise<CLIInvocation> {
  const settings = await getSettings(scopeUri);
  const configuredCommand = settings.command.trim();
  if (configuredCommand !== '') {
    return { command: configuredCommand, argsPrefix: settings.commandArgs, cwd: workingRoot };
  }
  if (settings.commandArgs.length > 0) {
    return { command: 'gsx', argsPrefix: settings.commandArgs, cwd: workingRoot };
  }
  const localCLIRoot = await findLocalCLIRoot(workingRoot);
  if (localCLIRoot !== undefined) {
    return {
      command: 'go',
      argsPrefix: ['run', './cmd/gsx'],
      cwd: localCLIRoot
    };
  }
  return { command: 'gsx', argsPrefix: [], cwd: workingRoot };
}

async function findLocalCLIRoot(startDir: string): Promise<string | undefined> {
  let current = startDir;
  while (true) {
    if (
      await pathExists(path.join(current, 'go.mod')) &&
      await pathExists(path.join(current, 'cmd', 'gsx', 'main.go'))
    ) {
      return current;
    }
    const parent = path.dirname(current);
    if (parent === current) {
      return undefined;
    }
    current = parent;
  }
}

async function pathExists(filePath: string): Promise<boolean> {
  try {
    await fs.access(filePath);
    return true;
  } catch {
    return false;
  }
}

function buildCLIErrorMessage(
  invocation: CLIInvocation,
  err: { stdout?: string; stderr?: string; code?: number | string; message?: string },
  prefix = 'Unable to run GSX CLI'
): string {
  const summary = `${prefix}: ${formatCommand(invocation.command, invocation.argsPrefix)}`;
  if (typeof err.code === 'string') {
    if (err.code === 'ENOENT') {
      return `${summary}. Command not found. Install a \`gsx\` binary or set \`gsx.command\` and \`gsx.commandArgs\`.`;
    }
    return `${summary}. ${err.message ?? err.code}`;
  }
  const details = (err.stderr ?? err.stdout ?? err.message ?? '').trim();
  if (details !== '') {
    return `${summary}. ${details}`;
  }
  return summary;
}

function formatCommand(command: string, args: string[]): string {
  return [command, ...args].join(' ');
}

async function workspaceRootForURI(uri: string): Promise<string> {
  const filePath = fileURLToPath(uri);
  const fromFolders = longestContainingWorkspace(filePath);
  if (fromFolders !== undefined) {
    return fromFolders;
  }
  let current = path.dirname(filePath);
  while (true) {
    try {
      await fs.access(path.join(current, 'go.mod'));
      return current;
    } catch {
      // continue
    }
    const parent = path.dirname(current);
    if (parent === current) {
      return path.dirname(filePath);
    }
    current = parent;
  }
}

function longestContainingWorkspace(filePath: string): string | undefined {
  let best: string | undefined;
  for (const folder of workspaceFolders) {
    if (filePath === folder || filePath.startsWith(folder + path.sep)) {
      if (best === undefined || folder.length > best.length) {
        best = folder;
      }
    }
  }
  return best;
}

function extractWorkspaceFolders(params: InitializeParams): string[] {
  const folders = params.workspaceFolders?.map((folder) => fileURLToPath(folder.uri)) ?? [];
  if (folders.length > 0) {
    return folders;
  }
  if (params.rootUri !== null && params.rootUri !== undefined) {
    return [fileURLToPath(params.rootUri)];
  }
  return [];
}

function fullDocumentRange(document: TextDocument) {
  const lines = document.getText().split(/\r?\n/);
  const lastLine = Math.max(0, lines.length - 1);
  const lastLineText = lines[lastLine] ?? '';
  return {
    start: { line: 0, character: 0 },
    end: { line: lastLine, character: lastLineText.length }
  };
}

function countDiagnostics(byFile: Map<string, Diagnostic[]>): { errors: number; warnings: number } {
  let errors = 0;
  let warnings = 0;
  for (const diagnostics of byFile.values()) {
    for (const diagnostic of diagnostics) {
      if (diagnostic.severity === DiagnosticSeverity.Warning) {
        warnings++;
      } else {
        errors++;
      }
    }
  }
  return { errors, warnings };
}

function clearScheduled(root: string): void {
  const timer = scheduledChecks.get(root);
  if (timer !== undefined) {
    clearTimeout(timer);
    scheduledChecks.delete(root);
  }
}

documents.listen(connection);
connection.listen();
