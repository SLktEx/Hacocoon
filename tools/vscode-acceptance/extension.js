'use strict';
// Trusted UI-only acceptance observer; it never runs inside an Environment.
const vscode = require('vscode');
const fs = require('node:fs');
const fixture = require('./fixture.json');

exports.activate = function (context) {
  const folder = vscode.workspace.workspaceFolders?.[0];
  if (!folder || folder.uri.scheme !== 'vscode-remote' ||
      folder.uri.authority !== fixture.authority || folder.uri.path !== '/workspace') return;
  let finished = false;
  let stage = 'remote-kind';
  let terminal;
  const ownedProbes = [];
  const cleanupProbes = async () => {
    for (const uri of ownedProbes) await vscode.workspace.fs.delete(uri);
    ownedProbes.length = 0;
  };
  let review;
  const checks = [];
  const reviewDiagnostics = {};
  const finish = status => {
    if (finished) return;
    finished = true;
    clearTimeout(deadline);
    terminal?.dispose();
    review?.dispose();
    // A pre-existing result is never overwritten or interpreted as this run.
    const temporary = fixture.result + '.tmp';
    fs.writeFileSync(temporary, JSON.stringify({
      status, stage, checks, reviewDiagnostics, authority: fixture.authority,
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
      ownedProbes.push(probe);
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
      ownedProbes.push(terminalURI);
      checks.push('remote-terminal-exec');
      stage = 'local-approval-review';
      let refusalObserved = false, readyObserved = false;
      reviewDiagnostics.step = "api";
      // VS Code exposes gated API getters. Never enumerate the complete API.
      const localAPI = { env: vscode.env, UIKind: vscode.UIKind, workspace: vscode.workspace,
        ViewColumn: vscode.ViewColumn, window: {
        showWarningMessage: (...args) => vscode.window.showWarningMessage(...args),
        showErrorMessage: (...args) => vscode.window.showErrorMessage(...args),
        createWebviewPanel(type, title, column, options) {
        reviewDiagnostics.panelCreated = true;
        if (!options.enableScripts || options.enableCommandUris || options.localResourceRoots.length !== 0) throw new Error('review webview is not isolated');
        const panel = vscode.window.createWebviewPanel(type, title, column, options);
        // Observe the real renderer's handshake and the real controller's stale
        // refusal without injecting a click, request detail or decision.
        panel.webview.onDidReceiveMessage(message => { if (message.type === 'ready') readyObserved = true; });
        const webview = {
          get html() { return panel.webview.html; }, set html(value) { panel.webview.html = value; },
          onDidReceiveMessage: (...args) => panel.webview.onDidReceiveMessage(...args),
          postMessage(message) {
            if (message.type === 'error' && message.error === 'no_longer_pending') refusalObserved = true;
            return panel.webview.postMessage(message);
          }
        };
        return { webview, reveal: (...args) => panel.reveal(...args), dispose: () => panel.dispose(),
          onDidDispose: (...args) => panel.onDidDispose(...args) };
      } } };
      reviewDiagnostics.step = "create";
      reviewDiagnostics.localUI = context.extension.extensionKind === vscode.ExtensionKind.UI;
      reviewDiagnostics.desktop = vscode.env.uiKind === vscode.UIKind.Desktop;
      reviewDiagnostics.trusted = vscode.workspace.isTrusted;
      review = require('./review').createReview(localAPI, { localUI: context.extension.extensionKind === vscode.ExtensionKind.UI });
      // An unpredictable stale ID exercises the real UI/WSL/management route,
      // without selecting or answering any pending operation.
      reviewDiagnostics.step = "open";
      review(fixture.nonce);
      reviewDiagnostics.step = "wait";
      for (let i = 0; i < 300 && !finished; i++) {
        if (readyObserved && refusalObserved) break;
        await new Promise(resolve => setTimeout(resolve, 100));
      }
      reviewDiagnostics.readyObserved = readyObserved;
      reviewDiagnostics.refusalObserved = refusalObserved;
      if (!readyObserved || !refusalObserved) {
        throw new Error('local review did not reach the installed controller');
      }
      checks.push('local-approval-stale-refusal');
      stage = 'cleanup';
      await cleanupProbes();
      checks.push('owned-probes-removed');
      stage = 'complete';
      finish('passed');
    } catch (error) {
      reviewDiagnostics.errorKind = error instanceof TypeError ? "type" : "other";
      try { await cleanupProbes(); reviewDiagnostics.cleanup = true; }
      catch { reviewDiagnostics.cleanup = false; }
      // Avoid emitting remote errors or SSH/server tokens into CI output.
      finish('failed');
    }
  })();
};
