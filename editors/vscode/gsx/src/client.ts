import * as path from 'node:path';
import * as vscode from 'vscode';
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind
} from 'vscode-languageclient/node';

let client: LanguageClient | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  const serverModule = context.asAbsolutePath(path.join('dist', 'server.js'));
  const serverOptions: ServerOptions = {
    run: { module: serverModule, transport: TransportKind.ipc },
    debug: {
      module: serverModule,
      transport: TransportKind.ipc,
      options: { execArgv: ['--nolazy', '--inspect=6010'] }
    }
  };

  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ language: 'gsx', scheme: 'file' }],
    synchronize: {
      fileEvents: vscode.workspace.createFileSystemWatcher('**/*.gsx')
    },
    outputChannelName: 'GSX Language Server'
  };

  client = new LanguageClient('gsxLanguageServer', 'GSX Language Server', serverOptions, clientOptions);
  await client.start();
  context.subscriptions.push({
    dispose() {
      void client?.stop();
    }
  });

  context.subscriptions.push(vscode.commands.registerCommand('gsx.formatDocument', async () => {
    await vscode.commands.executeCommand('editor.action.formatDocument');
  }));

  context.subscriptions.push(vscode.commands.registerCommand('gsx.runCheck', async () => {
    const editor = vscode.window.activeTextEditor;
    if (editor === undefined || editor.document.languageId != 'gsx' || client === undefined) {
      return;
    }
    try {
      const result = await client.sendRequest<{ message: string; hasErrors: boolean }>('gsx/runCheck', {
        uri: editor.document.uri.toString()
      });
      if (result.hasErrors) {
        void vscode.window.showWarningMessage(result.message);
      } else {
        void vscode.window.showInformationMessage(result.message);
      }
    } catch (err) {
      void vscode.window.showErrorMessage(`GSX check failed: ${String(err)}`);
    }
  }));
}

export async function deactivate(): Promise<void> {
  if (client !== undefined) {
    await client.stop();
  }
}
