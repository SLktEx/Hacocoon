'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');
const source = fs.readFileSync(path.join(__dirname, '../../tools/vscode-acceptance/extension.js'), 'utf8');

async function observe(options = {}) {
  const fixture = {authority: 'ssh-remote+haco-win-ssh-0123456789abcdef', nonce: 'ab'.repeat(16), result: '/result'};
  const files = new Map([['/workspace/windows-marker', Buffer.from(options.badMarker ? 'wrong' : 'windows-workspace-ok')]]);
  const publications = new Map();
  let shown = false, terminal = false, disposed = false, closed = false;
  const folder = {scheme: 'vscode-remote', authority: fixture.authority, path: '/workspace', ...options.uri};
  const api = {
    UIKind: { Desktop: 1 }, ViewColumn: {Active: -1}, ExtensionKind: { UI: 1 }, version: 'fixture', env: {uiKind: 1, remoteName: options.remoteName || 'ssh-remote'},
    Uri: {joinPath: (uri, name) => ({...uri, path: uri.path + '/' + name})},
    workspace: {
      isTrusted: true, workspaceFolders: [{uri: folder}],
      fs: {
        readFile: async uri => {
          if (!files.has(uri.path)) throw Object.assign(new Error('missing'), {code: 'FileNotFound'});
          return files.get(uri.path);
        },
        writeFile: async (uri, bytes) => files.set(uri.path, bytes),
        delete: async uri => { if (options.cleanupFailure) throw Error('cannot delete'); files.delete(uri.path); }
      },
      openTextDocument: async uri => ({getText: () => options.badEditor ? 'wrong' : files.get(uri.path).toString()})
    },
    window: {
      createWebviewPanel() {
        return {webview: {onDidReceiveMessage(fn) { fn({type: 'ready'}); }, postMessage() {}},
          reveal() {}, dispose() {}, onDidDispose() {}};
      },
      showTextDocument: async () => { shown = true; },
      createTerminal: ({cwd, pty}) => {
        if (pty) return { dispose() {} };
        assert.equal(cwd.authority, fixture.authority);
        terminal = true;
        return {
          sendText: command => {
            assert.ok(command.includes('test ! -e /init && test ! -e /run/WSL'));
            assert.ok(command.includes('cat /workspace/.haco-editor-' + fixture.nonce));
            if (!options.noTerminal) files.set('/workspace/.haco-terminal-' + fixture.nonce,
              Buffer.from(options.badTerminal ? 'wrong' : fixture.nonce));
          },
          dispose: () => { disposed = true; }
        };
      }
    },
    commands: {executeCommand: async () => { closed = true; }}
  };
  Object.defineProperty(api, 'gatedAPI', {enumerable: true, get() { throw Error('proposed API unavailable'); }});
  Object.defineProperty(api.window, 'gatedAPI', {enumerable: true, get() { throw Error('proposed API unavailable'); }});
  const context = {
    exports: {}, Buffer,
    require: name => name === './review' ? { createReview(localAPI, settings) {
      assert.equal(settings.localUI, true);
      const review = (id) => {
        assert.equal(id, fixture.nonce);
        const panel = localAPI.window.createWebviewPanel('hacocoon.approval', 'Review', -1, {enableScripts: true, enableCommandUris: false, localResourceRoots: []});
        panel.webview.postMessage({type: 'error', error: options.badReview ? 'wrong' : 'no_longer_pending'});
      };
      review.dispose = () => {};
      return review;
    } } : name === 'vscode' ? api : name === './fixture.json' ? fixture : {
      writeFileSync: (p, content, flags) => { assert.equal(flags.flag, 'wx'); assert.ok(!publications.has(p)); publications.set(p, content); },
      linkSync: (src, dst) => { assert.ok(!publications.has(dst)); publications.set(dst, publications.get(src)); },
      unlinkSync: p => publications.delete(p)
    },
    setTimeout: (fn, ms) => { if (ms < 360000) queueMicrotask(fn); return 1; },
    clearTimeout: () => {}
  };
  vm.runInNewContext(source, context);
  context.exports.activate({ extension: { extensionKind: 1 } });
  for (let i = 0; i < 1000; i++) await Promise.resolve();
  return {result: publications.has('/result') ? JSON.parse(publications.get('/result')) : undefined,
    files, shown, terminal, disposed, closed};
}
test('PASS uses only required stable APIs and requires editor, terminal and cleanup', async () => {
  const r = await observe();
  assert.equal(r.result.status, 'passed');
  assert.deepEqual(r.result.checks, ['workspace-marker', 'editor-file-read-write', 'remote-terminal-exec', 'local-approval-stale-refusal', 'owned-probes-removed']);
  assert.equal(r.files.size, 1);
  assert.ok(r.shown && r.terminal && r.disposed && r.closed);
});
for (const uri of [{scheme: 'file'}, {authority: 'ssh-remote+unrelated'}, {path: '/other'}]) {
  test('unrelated folder never publishes acceptance: ' + JSON.stringify(uri), async () => {
    const r = await observe({uri});
    assert.equal(r.result, undefined);
    assert.equal(r.terminal, false);
  });
}
for (const [name, options, stage] of [
  ['wrong remote kind', {remoteName: 'wsl'}, 'remote-kind'],
  ['wrong workspace', {badMarker: true}, 'remote-filesystem'],
  ['editor mismatch', {badEditor: true}, 'remote-filesystem'],
  ['terminal absent', {noTerminal: true}, 'remote-terminal'],
  ['terminal mismatch', {badTerminal: true}, 'remote-terminal'],
  ['wrong local review result', {badReview: true}, 'local-approval-review'],
  ['cleanup fails', {cleanupFailure: true}, 'cleanup']
]) {
  test(name + ' cannot pass', async () => {
    const r = await observe(options);
    assert.equal(r.result.status, 'failed');
    assert.equal(r.result.stage, stage);
  });
}

test('failed local review removes its proven owned remote probes', async () => {
  const r = await observe({badReview: true});
  assert.equal(r.result.status, 'failed');
  assert.equal(r.result.reviewDiagnostics.cleanup, true);
  assert.equal(r.files.size, 1);
});
