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
  const calls = [];
  const operation = name => {
    calls.push(name);
    if (options.failAt === name) throw options.error ?? new Error('secret remote detail');
  };
  let shown = false, terminal = false, disposed = false, closed = false;
  const folder = {scheme: 'vscode-remote', authority: fixture.authority, path: '/workspace', ...options.uri};
  const api = {
    UIKind: { Desktop: 1 }, ViewColumn: {Active: -1}, ExtensionKind: { UI: 1 }, version: 'fixture', env: {uiKind: 1, remoteName: options.remoteName || 'ssh-remote'},
    Uri: {joinPath: (uri, name) => ({...uri, path: uri.path + '/' + name})},
    workspace: {
      isTrusted: true, workspaceFolders: options.noFolder ? [] : [{uri: folder}],
      fs: {
        readFile: async uri => {
          operation('marker-read');
          if (options.stallFilesystem) return new Promise(() => {});
          if (!files.has(uri.path)) throw Object.assign(new Error('missing'), {code: 'FileNotFound'});
          return files.get(uri.path);
        },
        writeFile: async (uri, bytes) => { operation('editor-write'); files.set(uri.path, bytes); },
        delete: async uri => { operation('delete'); if (options.cleanupFailure) throw Error('cannot delete'); files.delete(uri.path); }
      },
      openTextDocument: async uri => {
        operation('editor-open');
        return {getText: () => { operation('editor-validate'); return options.badEditor ? 'wrong' : files.get(uri.path).toString(); }};
      }
    },
    window: {
      createWebviewPanel() {
        return {webview: {onDidReceiveMessage(fn) { fn({type: 'ready'}); }, postMessage() {}},
          reveal() {}, dispose() {}, onDidDispose() {}};
      },
      showTextDocument: async () => { operation('editor-show'); shown = true; },
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
          dispose: () => { if (options.disposeFailure) throw Error("secret disposal error"); disposed = true; }
        };
      }
    },
    commands: {executeCommand: async () => { closed = true; }}
  };
  Object.defineProperty(api, 'gatedAPI', {enumerable: true, get() { throw Error('proposed API unavailable'); }});
  Object.defineProperty(api.window, 'gatedAPI', {enumerable: true, get() { throw Error('proposed API unavailable'); }});
  let deadline;
  const context = {
    exports: {}, Buffer, TypeError,
    require: name => name === 'node:perf_hooks' ? {performance: {now() {
      if (options.clockFailure) throw Error('secret clock failure');
      return options.clockValue ?? calls.length * 10;
    }}} : name === './review' ? { createReview(localAPI, settings) {
      assert.equal(settings.localUI, true);
      const review = (id) => {
        assert.equal(id, fixture.nonce);
        const panel = localAPI.window.createWebviewPanel('hacocoon.approval', 'Review', -1, {enableScripts: true, enableCommandUris: false, localResourceRoots: []});
        panel.webview.postMessage({type: 'error', error: options.badReview ? 'wrong' : 'no_longer_pending'});
      };
      review.dispose = () => {};
      return review;
    } } : name === 'vscode' ? api : name === './fixture.json' ? fixture : {
      writeFileSync: (p, content, flags) => {
        assert.equal(flags.flag, 'wx');
        if ((options.progressFailure && p.includes('.progress-')) ||
            (options.receiptWriteFailure && p === '/result.tmp')) throw Error('secret write error');
        assert.ok(!publications.has(p)); publications.set(p, content);
      },
      linkSync: (src, dst) => {
        if (options.receiptLinkFailure) throw Error('secret link error');
        assert.ok(!publications.has(dst)); publications.set(dst, publications.get(src));
      },
      unlinkSync: p => publications.delete(p)
    },
    setTimeout: (fn, ms) => {
      if (ms < 360000) queueMicrotask(fn);
      else { assert.equal(ms, 360000); deadline = fn; }
      return 1;
    },
    clearTimeout: () => {}
  };
  vm.runInNewContext(source, context);
  context.exports.activate({ extension: { extensionKind: 1 } });
  if (options.repeatActivation) context.exports.activate({ extension: { extensionKind: 1 } });
  for (let i = 0; i < 1000; i++) await Promise.resolve();
  if (options.expire) deadline();
  const progress = [...publications.entries()].filter(([name]) => name.includes('.progress-'))
    .map(([, value]) => JSON.parse(value));
  for (const signal of progress) {
    assert.equal(signal.authority, fixture.authority);
    assert.equal(signal.nonce, fixture.nonce);
    assert.deepEqual(Object.keys(signal).sort(), ['authority', 'nonce', 'phase']);
  }
  return {progress: progress.map(signal => signal.phase), publications,
    result: publications.has('/result') ? JSON.parse(publications.get('/result')) : undefined,
    files, shown, terminal, disposed, closed, calls};
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

for (const [name, options, phase] of [
  ['no folder', {noFolder: true}, 'target-missing'],
  ['wrong scheme', {uri: {scheme: 'file'}}, 'target-scheme-mismatch'],
  ['wrong authority', {uri: {authority: 'secret-authority'}}, 'target-authority-mismatch'],
  ['wrong path', {uri: {path: '/secret-path'}}, 'target-path-mismatch']
]) {
  test(name + ' leaves bounded progress without acceptance', async () => {
    const r = await observe(options);
    assert.deepEqual(r.progress, ['activated', phase]);
    assert.equal(r.result, undefined);
    assert.equal(r.terminal, false);
    assert.ok(!JSON.stringify([...r.publications.values()]).includes('secret'));
  });
}
for (const [name, options, last] of [
  ['receipt write', {receiptWriteFailure: true}, 'review-disposed'],
  ['receipt publication', {receiptLinkFailure: true}, 'receipt-written'],
  ['terminal disposal', {disposeFailure: true}, 'finish-started']
]) {
  test(name + ' failure leaves progress and cannot pass', async () => {
    const r = await observe(options);
    assert.equal(r.result, undefined);
    assert.equal(r.progress.at(-1), last);
    assert.ok(!JSON.stringify([...r.publications.values()]).includes('secret'));
  });
}
test('progress write failure does not change successful receipt', async () => {
  const r = await observe({progressFailure: true});
  assert.equal(r.result.status, 'passed');
  assert.deepEqual(r.progress, []);
});
test('stalled observer retains 360s failure deadline and records its stage', async () => {
  const r = await observe({stallFilesystem: true, expire: true});
  assert.equal(r.result.status, 'failed');
  assert.equal(r.result.stage, 'remote-filesystem');
  assert.ok(r.progress.includes('deadline'));
});

test('repeated unrelated activation cannot overwrite or multiply fixed markers', async () => {
  const r = await observe({noFolder: true, repeatActivation: true});
  assert.deepEqual(r.progress, ['activated', 'target-missing']);
  assert.equal(r.publications.size, 2);
  assert.equal(r.result, undefined);
});


for (const [suboperation, options, checks, deleted] of [
  ['marker-read', {failAt: 'marker-read'}, [], false],
  ['marker-validate', {badMarker: true}, [], false],
  ['editor-write', {failAt: 'editor-write'}, ['workspace-marker'], false],
  ['editor-open', {failAt: 'editor-open'}, ['workspace-marker'], true],
  ['editor-validate', {failAt: 'editor-validate'}, ['workspace-marker'], true],
  ['editor-validate', {badEditor: true}, ['workspace-marker'], true],
  ['editor-show', {failAt: 'editor-show'}, ['workspace-marker'], true]
]) {
  test('filesystem failure identifies ' + suboperation + ' ' + JSON.stringify(options), async () => {
    const r = await observe(options);
    assert.equal(r.result.status, 'failed');
    assert.equal(r.result.stage, 'remote-filesystem');
    assert.deepEqual(r.result.checks, checks);
    assert.equal(r.result.filesystemFailure.suboperation, suboperation);
    assert.equal(r.result.filesystemFailure.error_category, 'other');
    assert.equal(r.result.filesystemFailure.duration_ms, (r.calls.length - Number(deleted)) * 10);
    assert.equal(r.result.reviewDiagnostics.cleanup, true);
    assert.equal(r.calls.includes('delete'), deleted);
    assert.equal(r.files.size, 1);
    assert.equal(r.terminal, false);
    assert.equal(r.closed, true);
    assert.ok(!JSON.stringify([...r.publications.values()]).includes('secret'));
  });
}

for (const [code, category] of [
  ['FileNotFound', 'not-found'], ['FileExists', 'exists'],
  ['FileNotADirectory', 'not-directory'], ['FileIsADirectory', 'is-directory'],
  ['NoPermissions', 'no-permissions'], ['Unavailable', 'unavailable'],
  ['secret bearer token', 'other'], [42, 'other'], [null, 'other']
]) {
  test('filesystem error code projection: ' + category + ' ' + code, async () => {
    const error = Object.assign(new Error('secret /path?token=value'), {code, stack: 'secret stack'});
    const r = await observe({failAt: 'editor-open', error});
    assert.equal(r.result.filesystemFailure.error_category, category);
    assert.equal(r.result.reviewDiagnostics.cleanup, true);
    assert.equal(r.files.size, 1);
    assert.ok(!JSON.stringify([...r.publications.values()]).includes('secret'));
  });
}

test('hostile error accessors cannot skip owned cleanup or replace failure', async () => {
  const error = Object.defineProperty({}, 'code', {get() { throw Error('secret accessor'); }});
  const r = await observe({failAt: 'editor-open', error});
  assert.equal(r.result.filesystemFailure.error_category, 'unobserved');
  assert.equal(r.result.status, 'failed');
  assert.equal(r.result.reviewDiagnostics.cleanup, true);
  assert.equal(r.files.size, 1);
  assert.ok(!JSON.stringify([...r.publications.values()]).includes('secret'));
});

test('type classification and cleanup failure preserve the original filesystem operation', async () => {
  const r = await observe({failAt: 'editor-open', error: new TypeError('secret'), cleanupFailure: true});
  assert.equal(r.result.filesystemFailure.suboperation, 'editor-open');
  assert.equal(r.result.filesystemFailure.error_category, 'type');
  assert.equal(r.result.reviewDiagnostics.errorKind, 'type');
  assert.equal(r.result.reviewDiagnostics.cleanup, false);
  assert.equal(r.result.status, 'failed');
  assert.equal(r.result.stage, 'remote-filesystem');
  assert.equal(r.files.size, 2);
});

for (const options of [{clockFailure: true}, {clockValue: NaN}, {clockValue: Infinity}]) {
  test('unavailable diagnostic clock cannot affect acceptance: ' + JSON.stringify(options), async () => {
    const passed = await observe(options);
    assert.equal(passed.result.status, 'passed');
    assert.equal(passed.result.filesystemFailure, undefined);
    const failed = await observe({...options, failAt: 'editor-open'});
    assert.equal(failed.result.status, 'failed');
    assert.equal(failed.result.filesystemFailure.duration_ms, null);
    assert.equal(failed.result.reviewDiagnostics.cleanup, true);
    assert.equal(failed.files.size, 1);
  });
}

test('diagnostic marker write failures preserve filesystem failure and cleanup', async () => {
  const r = await observe({progressFailure: true, failAt: 'editor-open'});
  assert.deepEqual(r.progress, []);
  assert.equal(r.result.status, 'failed');
  assert.equal(r.result.filesystemFailure.suboperation, 'editor-open');
  assert.equal(r.result.reviewDiagnostics.cleanup, true);
  assert.equal(r.files.size, 1);
});
