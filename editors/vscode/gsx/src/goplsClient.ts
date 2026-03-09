import * as fs from 'node:fs/promises';
import * as os from 'node:os';
import { spawn } from 'node:child_process';
import { fileURLToPath, pathToFileURL } from 'node:url';
import * as path from 'node:path';
import {
  createMessageConnection,
  StreamMessageReader,
  StreamMessageWriter
} from 'vscode-jsonrpc/node';

const clients = new Map<string, Promise<GoplsClient | undefined>>();

export async function goplsHover(
  command: string | undefined,
  uri: string,
  text: string,
  position: { line: number; character: number }
): Promise<any | undefined> {
  const client = await getClient(command, uri);
  if (client === undefined) {
    return undefined;
  }
  return client.request('textDocument/hover', uri, text, position);
}

export async function goplsCompletion(
  command: string | undefined,
  uri: string,
  text: string,
  position: { line: number; character: number }
): Promise<any | undefined> {
  const client = await getClient(command, uri);
  if (client === undefined) {
    return undefined;
  }
  return client.request('textDocument/completion', uri, text, position, {
    triggerKind: 1
  });
}

export async function shutdownGoplsClients(): Promise<void> {
  const entries = [...clients.values()];
  clients.clear();
  const resolved = await Promise.all(entries);
  await Promise.all(resolved.map((client) => client?.shutdown()));
}

async function getClient(command: string | undefined, uri: string): Promise<GoplsClient | undefined> {
  const resolved = await resolveGoplsCommand(command);
  if (resolved === undefined) {
    return undefined;
  }
  const rootUri = await workspaceRootURIForFileURI(uri);
  const key = `${resolved}\n${rootUri ?? ''}`;
  const existing = clients.get(key);
  if (existing !== undefined) {
    return existing;
  }
  const created = createClient(resolved, rootUri, key);
  clients.set(key, created);
  return created;
}

async function createClient(command: string, rootUri: string | undefined, key: string): Promise<GoplsClient | undefined> {
  const proc = spawn(command, ['serve'], { stdio: ['pipe', 'pipe', 'pipe'] });
  const connection = createMessageConnection(
    new StreamMessageReader(proc.stdout),
    new StreamMessageWriter(proc.stdin)
  );
  connection.listen();
  proc.stderr.on('data', () => {
    // Silence background gopls logging in the GSX server process.
  });
  proc.on('exit', () => {
    connection.dispose();
    clients.delete(key);
  });
  try {
    await connection.sendRequest('initialize', {
      processId: process.pid,
      rootUri: rootUri ?? null,
      workspaceFolders: rootUri === undefined ? null : [{ uri: rootUri, name: path.basename(fileURLToPath(rootUri)) }],
      capabilities: {
        textDocument: {
          hover: { contentFormat: ['markdown', 'plaintext'] },
          completion: {
            completionItem: {
              snippetSupport: false
            }
          }
        }
      }
    });
    connection.sendNotification('initialized', {});
    return new GoplsClient(proc, connection, key);
  } catch {
    try {
      proc.kill();
    } catch {
      // Ignore cleanup failures.
    }
    clients.delete(key);
    return undefined;
  }
}

class GoplsClient {
  private version = 1;

  constructor(
    private readonly proc: ReturnType<typeof spawn>,
    private readonly connection: ReturnType<typeof createMessageConnection>,
    private readonly key: string
  ) {}

  async request(method: string, uri: string, text: string, position: { line: number; character: number }, context?: object): Promise<any | undefined> {
    const version = this.version++;
    this.connection.sendNotification('textDocument/didOpen', {
      textDocument: {
        uri,
        languageId: 'go',
        version,
        text
      }
    });
    try {
      return await this.connection.sendRequest(method, {
        textDocument: { uri },
        position,
        context
      });
    } catch {
      return undefined;
    } finally {
      this.connection.sendNotification('textDocument/didClose', {
        textDocument: { uri }
      });
    }
  }

  async shutdown(): Promise<void> {
    try {
      await this.connection.sendRequest('shutdown');
    } catch {
      // Ignore shutdown failures.
    }
    try {
      this.connection.sendNotification('exit');
    } catch {
      // Ignore exit failures.
    }
    this.connection.dispose();
    clients.delete(this.key);
    try {
      this.proc.kill();
    } catch {
      // Ignore cleanup failures.
    }
  }
}

async function resolveGoplsCommand(configured: string | undefined): Promise<string | undefined> {
  const candidates = [
    configured?.trim(),
    'gopls',
    `${os.homedir()}/.local/bin/gopls`,
    `${os.homedir()}/go/bin/gopls`
  ].filter((value): value is string => value !== undefined && value !== '');
  for (const candidate of candidates) {
    if (await commandExists(candidate)) {
      return candidate;
    }
  }
  return undefined;
}

async function commandExists(command: string): Promise<boolean> {
  if (command.includes('/')) {
    try {
      await fs.access(command);
      return true;
    } catch {
      return false;
    }
  }
  return await new Promise<boolean>((resolve) => {
    const proc = spawn(command, ['version'], { stdio: 'ignore' });
    proc.on('error', () => resolve(false));
    proc.on('exit', (code) => resolve(code === 0));
  });
}

async function workspaceRootURIForFileURI(uri: string | undefined): Promise<string | undefined> {
  if (uri === undefined) {
    return undefined;
  }
  const moduleRoot = await findNearestModule(fileURLToPath(uri));
  return moduleRoot === undefined ? undefined : pathToFileURL(moduleRoot).toString();
}

async function findNearestModule(startPath: string): Promise<string | undefined> {
  let current = path.dirname(startPath);
  while (true) {
    const goModPath = path.join(current, 'go.mod');
    try {
      await fs.access(goModPath);
      return current;
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
