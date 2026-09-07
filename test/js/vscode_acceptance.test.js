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
    version: 'fixture', env: {remoteName: options.remoteName || 'ssh-remote'},
    Uri: {joinPath: (uri, name) => ({...uri, path: uri.path + '/' + name})},
    workspace: {
      workspaceFolders: [{uri: folder}],
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
      showTextDocument: async () => { shown = true; },
      createTerminal: ({cwd}) => {
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
  const context = {
    exports: {}, Buffer,
    require: name => name === 'vscode' ? api : name === './fixture.json' ? fixture : {
      writeFileSync: (p, content, flags) => { assert.equal(flags.flag, 'wx'); assert.ok(!publications.has(p)); publications.set(p, content); },
      linkSync: (src, dst) => { assert.ok(!publications.has(dst)); publications.set(dst, publications.get(src)); },
      unlinkSync: p => publications.delete(p)
    },
    setTimeout: (fn, ms) => { if (ms < 360000) queueMicrotask(fn); return 1; },
    clearTimeout: () => {}
  };
  vm.runInNewContext(source, context);
  context.exports.activate();
  for (let i = 0; i < 1000; i++) await Promise.resolve();
  return {result: publications.has('/result') ? JSON.parse(publications.get('/result')) : undefined,
    files, shown, terminal, disposed, closed};
}
test('PASS requires remote editor read/write, terminal execution and cleanup', async () => {
  const r = await observe();
  assert.equal(r.result.status, 'passed');
  assert.deepEqual(r.result.checks, ['workspace-marker', 'editor-file-read-write', 'remote-terminal-exec', 'owned-probes-removed']);
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
  ['cleanup fails', {cleanupFailure: true}, 'cleanup']
]) {
  test(name + ' cannot pass', async () => {
    const r = await observe(options);
    assert.equal(r.result.status, 'failed');
    assert.equal(r.result.stage, stage);
  });
}
