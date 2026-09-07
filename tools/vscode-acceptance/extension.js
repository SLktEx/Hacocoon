'use strict';
// Trusted UI-only acceptance observer; it never runs inside an Environment.
const vscode = require('vscode');
const fs = require('node:fs');
const fixture = require('./fixture.json');

exports.activate = function () {
  const folder = vscode.workspace.workspaceFolders?.[0];
  if (!folder || folder.uri.scheme !== 'vscode-remote' ||
      folder.uri.authority !== fixture.authority || folder.uri.path !== '/workspace') return;
  let finished = false;
  let stage = 'remote-kind';
  let terminal;
  const checks = [];
  const finish = status => {
    if (finished) return;
    finished = true;
    clearTimeout(deadline);
    terminal?.dispose();
    // A pre-existing result is never overwritten or interpreted as this run.
    const temporary = fixture.result + '.tmp';
    fs.writeFileSync(temporary, JSON.stringify({
      status, stage, checks, authority: fixture.authority,
      nonce: fixture.nonce, vscode: vscode.version
    }), {flag: 'wx'});
    fs.linkSync(temporary, fixture.result);
    fs.unlinkSync(temporary);
    void vscode.commands.executeCommand('workbench.action.closeWindow');
  };
  const deadline = setTimeout(() => finish('failed'), 360000);
  void (async () => {
    try {
      if (vscode.env.remoteName !== 'ssh-remote') throw new Error('wrong remote kind');
      stage = 'remote-filesystem';
      const marker = vscode.Uri.joinPath(folder.uri, 'windows-marker');
      if (Buffer.from(await vscode.workspace.fs.readFile(marker)).toString().trim() !== 'windows-workspace-ok') {
        throw new Error('wrong workspace');
      }
      checks.push('workspace-marker');
      const probe = vscode.Uri.joinPath(folder.uri, '.haco-editor-' + fixture.nonce);
      await vscode.workspace.fs.writeFile(probe, Buffer.from(fixture.nonce));
      const document = await vscode.workspace.openTextDocument(probe);
      if (document.getText() !== fixture.nonce) throw new Error('editor read mismatch');
      await vscode.window.showTextDocument(document, {preview: true, preserveFocus: true});
      checks.push('editor-file-read-write');
      stage = 'remote-terminal';
      const terminalFile = '.haco-terminal-' + fixture.nonce;
      terminal = vscode.window.createTerminal({name: 'Hacocoon acceptance', cwd: folder.uri});
      terminal.sendText("test ! -e /init && test ! -e /run/WSL && test \"$(cat /workspace/.haco-editor-" + fixture.nonce + ")\" = '" + fixture.nonce + "' && printf '%s' '" + fixture.nonce + "' > /workspace/" + terminalFile, true);
      const terminalURI = vscode.Uri.joinPath(folder.uri, terminalFile);
      let observed = false;
      for (let i = 0; i < 90 && !finished; i++) {
        try {
          const value = Buffer.from(await vscode.workspace.fs.readFile(terminalURI)).toString();
          if (value !== fixture.nonce) throw new Error('terminal marker mismatch');
          observed = true;
          break;
        } catch (error) {
          if (error.code !== 'FileNotFound') throw error;
        }
        await new Promise(resolve => setTimeout(resolve, 1000));
      }
      if (!observed) throw new Error('terminal did not execute');
      checks.push('remote-terminal-exec');
      stage = 'cleanup';
      await vscode.workspace.fs.delete(probe);
      await vscode.workspace.fs.delete(terminalURI);
      checks.push('owned-probes-removed');
      stage = 'complete';
      finish('passed');
    } catch {
      // Avoid emitting remote errors or SSH/server tokens into CI output.
      finish('failed');
    }
  })();
};
