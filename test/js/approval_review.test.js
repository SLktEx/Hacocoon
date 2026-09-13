"use strict";
const assert = require('node:assert/strict');
const { EventEmitter } = require('node:events');
const { PassThrough } = require('node:stream');
const test = require('node:test');
const { createReview, launchSpec, validView } = require('../../clients/vscode-notify/review');
const id = 'a'.repeat(32), token = 'b'.repeat(64);
const view = () => ({ request: { request_id: id, request: { capability: 'network.egress', action: 'connect', resource: 'example.com:443', environment: 'dev' } },
  token, digest: 'c'.repeat(64), options: ['allow', 'deny', 'ask'].flatMap(d => ['environment', 'global'].map(level => ({choice: `${d}-${level}`, scope: {decision: d === 'ask' ? 'require-approval' : d}}))) });

function harness(overrides = {}) {
  const output = [], writes = [], spawns = [], messages = [], panels = [], timers = [];
  let kills = 0, handler, dispose;
  const child = new EventEmitter();
  child.stdout = new PassThrough(); child.stderr = new PassThrough();
  child.stdin = new EventEmitter(); child.stdin.write = value => writes.push(JSON.parse(value));
  child.kill = () => { kills++; };
  const vscode = {
    UIKind: { Desktop: 1 }, ViewColumn: { Active: -1 }, env: { uiKind: 1, language: 'ja' },
    workspace: { isTrusted: true, getConfiguration() { return { inspect() { return { globalValue: 'Hacocoon', workspaceValue: 'evil' }; } }; } },
    window: {
      showWarningMessage(value) { messages.push(value); }, showErrorMessage(value) { messages.push(value); },
      createWebviewPanel(type, title, column, options) {
        const panel = { type, title, column, options, reveal() {}, dispose() { dispose(); }, onDidDispose(fn) { dispose = fn; },
          webview: { postMessage(m) { output.push(m); return Promise.resolve(true); }, onDidReceiveMessage(fn) { handler = fn; } } };
        panels.push(panel); return panel;
      }
    }
  };
  const review = createReview(vscode, { localUI: true, platform: 'linux', environment: {}, directory: '/home/operator',
    spawn(...args) { spawns.push(args); return child; }, schedule(fn, ms) { const timer = {fn, ms}; timers.push(timer); return timer; }, unschedule(timer) { timer.canceled = true; }, ...overrides });
  const reply = (type, fields = {}) => child.stdout.write(JSON.stringify({ version: 1, sequence: writes.at(-1).sequence, type, ...fields }) + '\n');
  return { review, vscode, child, output, writes, spawns, messages, panels, timers, reply, message: m => handler(m), get kills() { return kills; } };
}
function selected(h) {
  h.review(id); h.message({type: 'ready'});
  h.reply('pending', {pending: [{request_id: id, capability: 'network.egress', action: 'connect'}]});
  h.reply('selected', {view: view()});
}

test('Windows launches a private fixed local CLI with no workspace environment or answers in argv', () => {
  const spec = launchSpec('win32', {SystemRoot: 'C:\\Windows', USERPROFILE: 'C:\\Users\\operator', PATH: 'evil', WSLENV: 'HACO_CONTROL_SOCKET/u', HACO_CONTROL_SOCKET: '/tmp/evil', NODE_OPTIONS: '--require evil'}, 'C:\\Users\\operator', 'Hacocoon', id);
  assert.equal(spec.file, 'C:\\Windows\\System32\\wsl.exe');
  assert.deepEqual(spec.args, ['--distribution', 'Hacocoon', '--exec', '/usr/bin/env', '-i', 'PATH=/usr/local/bin:/usr/bin:/bin', 'LANG=C.UTF-8', '/usr/local/bin/haco', '_desktop-review']);
  assert.equal(spec.options.shell, false); assert.equal(spec.options.windowsHide, true);
  for (const key of ['PATH', 'WSLENV', 'HACO_CONTROL_SOCKET', 'NODE_OPTIONS']) assert.equal(spec.options.env[key], undefined);
});
test('malformed IDs, distributions and unsupported platforms never execute', () => {
  for (const value of ['--help', 'a'.repeat(31), 'A'.repeat(32), id + '\n', null, {}]) {
    const h = harness(); h.review(value); assert.equal(h.panels.length, 0);
  }
  assert.throws(() => launchSpec('win32', { SystemRoot: 'relative' }, '/', 'Hacocoon', id));
  assert.throws(() => launchSpec('linux', {}, '/', '--exec', id));
  assert.throws(() => launchSpec('darwin', {}, '/', 'Hacocoon', id));
});
test('browser, remote extension hosts, untrusted workspaces and later trust revocation refuse review', () => {
  for (const mode of ['browser', 'remote', 'untrusted']) {
    const h = harness({localUI: mode !== 'remote'});
    if (mode === 'browser') h.vscode.env.uiKind = 2;
    if (mode === 'untrusted') h.vscode.workspace.isTrusted = false;
    h.review(id); assert.equal(h.panels.length, 0); assert.equal(h.spawns.length, 0);
  }
  const h = harness(); selected(h); h.vscode.workspace.isTrusted = false;
  h.message({type: 'decide', request_id: id, token, approved: true, save: ''});
  assert.equal(h.writes.filter(m => m.action === 'decide').length, 0); assert.equal(h.kills, 1);
});
test('opening and selecting only read; saved ask keeps an explicit current answer and duplicate clicks cannot replay', () => {
  const h = harness(); h.review(id); assert.deepEqual(h.writes, []);
  h.message({type: 'ready'}); h.message({type: 'ready'}); assert.equal(h.writes.length, 1);
  h.reply('pending', {pending: [{request_id: id, capability: 'network.egress', action: 'connect'}]});
  h.reply('selected', {view: view()});
  assert.deepEqual(h.writes.map(m => m.action), ['list', 'select']);
  const answer = {type: 'decide', request_id: id, token, approved: false, save: 'ask-global'};
  h.message(answer); h.message(answer);
  assert.equal(h.writes.length, 3); assert.equal(h.writes[2].approved, false); assert.equal(h.writes[2].save, 'ask-global');
  h.reply('result', {result: {request_id: id, execution_state: 'not-executed', audit_complete: false, saved_choice: 'ask-global'}});
  h.message(answer); assert.equal(h.writes.length, 3); assert.equal(h.output.at(-1).type, 'result');
});
test('stale notification is checked by the controller without a decision', () => {
  const h = harness(); h.review(id); h.message({type: 'ready'}); h.reply('pending');
  assert.equal(h.writes.at(-1).action, 'select');
  h.reply('error', {error: 'no_longer_pending'});
  assert.equal(h.output.at(-1).error, 'no_longer_pending'); assert.equal(h.writes.length, 2);
});
test('invalid, unshown and inconsistent answers never leave the webview adapter', () => {
  for (const change of [{request_id: 'd'.repeat(32)}, {token: 'e'.repeat(64)}, {approved: 'yes'}, {save: 'allow-everything'}, {save: 'deny-global'}]) {
    const h = harness(); selected(h);
    h.message({type: 'decide', request_id: id, token, approved: true, save: '', ...change});
    assert.equal(h.writes.length, 2);
  }
  const h = harness(); selected(h);
  h.message({type: 'select', request_id: 'd'.repeat(32)}); assert.equal(h.writes.length, 2);
});
test('only the local user distribution is used; concurrent notifications reuse a single panel', () => {
  const h = harness({platform: 'win32', environment: {SystemRoot: 'C:\\Windows'}});
  const panel = h.review(id); h.review(id);
  assert.equal(h.panels.length, 1); assert.equal(h.spawns[0][1][1], 'Hacocoon');
  assert.deepEqual(panel.options.localResourceRoots, []); assert.equal(panel.options.enableCommandUris, false);
  panel.dispose(); panel.dispose(); assert.equal(h.kills, 1);
});
test('invalid protocol output, oversized output and child failures stop without retry or raw diagnostics', () => {
  for (const mode of ['sequence', 'malformed', 'oversize', 'stderr', 'exit', 'stdin', 'timeout', 'dispose']) {
    const h = harness(); selected(h);
    h.message({type: 'decide', request_id: id, token, approved: true, save: ''});
    if (mode === 'sequence') h.child.stdout.write('{"version":1,"sequence":999,"type":"result"}\n');
    if (mode === 'malformed') h.child.stdout.write('SECRET\n');
    if (mode === 'oversize') h.child.stdout.write('x'.repeat(256 * 1024 + 1));
    if (mode === 'stderr') h.child.stderr.write('SECRET'.repeat(3000));
    if (mode === 'exit') h.child.emit('close', 1);
    if (mode === 'stdin') h.child.stdin.emit('error', new Error('SECRET'));
    if (mode === 'timeout') h.timers.find(t => t.ms === 15 * 60 * 1000).fn();
    if (mode === 'dispose') h.review.dispose();
    assert.equal(h.spawns.length, 1); assert.equal(h.writes.filter(m => m.action === 'decide').length, 1);
    assert.equal(h.kills, mode === 'exit' ? 0 : 1); assert.doesNotMatch(JSON.stringify(h.output), /SECRET/);
    if (mode !== 'dispose') assert.equal(h.output.at(-1).type, 'unavailable');
  }
});
test('pending refresh invalidates removed selections and scope enums match the shared service', () => {
  assert.equal(validView(view()), true);
  const invalid = view(); invalid.options[0].choice = 'allow_environment'; assert.equal(validView(invalid), false);
  const h = harness(); selected(h); h.message({type: 'list'}); h.reply('pending');
  h.message({type: 'decide', request_id: id, token, approved: true, save: ''});
  assert.equal(h.writes.length, 3); assert.equal(h.output.at(-1).selected, undefined);
});
