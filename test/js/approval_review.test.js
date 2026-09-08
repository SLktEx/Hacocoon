"use strict";
const assert = require('node:assert/strict');
const { EventEmitter } = require('node:events');
const { PassThrough } = require('node:stream');
const test = require('node:test');
const { createReview, launchSpec } = require('../../clients/vscode-notify/review');
const id = 'a'.repeat(32);

function harness(overrides = {}) {
  const output = [], writes = [], spawns = [], messages = [], terminals = [], timers = [];
  let kills = 0;
  const child = new EventEmitter();
  child.stdout = new PassThrough(); child.stderr = new PassThrough();
  child.stdin = new EventEmitter(); child.stdin.write = (value) => writes.push(value);
  child.kill = () => { kills++; };
  class Emitter {
    event = () => {};
    fire(value) { output.push(value); }
    dispose() {}
  }
  const vscode = {
    UIKind: { Desktop: 1 }, env: { uiKind: 1 }, EventEmitter: Emitter,
    workspace: { isTrusted: true, getConfiguration() { return { inspect() { return { globalValue: 'Hacocoon', workspaceValue: 'evil' }; } }; } },
    window: {
      showWarningMessage(value) { messages.push(value); }, showErrorMessage(value) { messages.push(value); },
      createTerminal(options) { const terminal = { options, show() {}, dispose() { options.pty.close(); } }; terminals.push(terminal); return terminal; }
    }
  };
  const review = createReview(vscode, { localUI: true, platform: 'linux', environment: {}, directory: '/home/operator',
    spawn(...args) { spawns.push(args); return child; }, schedule(fn) { timers.push(fn); return timers.length; }, unschedule() {}, ...overrides });
  return { review, vscode, child, output, writes, spawns, messages, terminals, timers, get kills() { return kills; } };
}

test('Windows launches fixed local WSL CLI without shell or workspace environment', () => {
  const spec = launchSpec('win32', { SystemRoot: 'C:\\Windows', USERPROFILE: 'C:\\Users\\operator',
    PATH: 'evil', WSLENV: 'HACO_CONTROL_SOCKET/u', HACO_CONTROL_SOCKET: '/tmp/evil', NODE_OPTIONS: '--require evil' },
  'C:\\Users\\operator', 'Hacocoon', id);
  assert.equal(spec.file, 'C:\\Windows\\System32\\wsl.exe');
  assert.deepEqual(spec.args, ['--distribution', 'Hacocoon', '--exec', '/usr/bin/env', '-i',
    'PATH=/usr/local/bin:/usr/bin:/bin', 'LANG=C.UTF-8', '/usr/local/bin/haco', 'approve', id]);
  assert.equal(spec.options.shell, false);
  assert.equal(spec.options.windowsHide, true);
  for (const key of ['PATH', 'WSLENV', 'HACO_CONTROL_SOCKET', 'NODE_OPTIONS']) assert.equal(spec.options.env[key], undefined);
});

test('malformed IDs, distributions and unsupported platforms never execute', () => {
  for (const value of ['--help', 'a'.repeat(31), 'A'.repeat(32), id + '\n', null, {}]) {
    const h = harness(); h.review(value); assert.equal(h.terminals.length, 0);
  }
  assert.throws(() => launchSpec('win32', { SystemRoot: 'relative' }, '/', 'Hacocoon', id));
  assert.throws(() => launchSpec('linux', {}, '/', '--exec', id));
  assert.throws(() => launchSpec('darwin', {}, '/', 'Hacocoon', id));
});

test('browser, remote extension hosts and untrusted workspaces cannot start review', () => {
  for (const mode of ['browser', 'remote', 'untrusted']) {
    const h = harness({ localUI: mode !== 'remote' });
    if (mode === 'browser') h.vscode.env.uiKind = 2;
    if (mode === 'untrusted') h.vscode.workspace.isTrusted = false;
    h.review(id); assert.equal(h.terminals.length, 0); assert.equal(h.spawns.length, 0);
  }
});

test('opening review never sends an answer; saved ask uses separate user input', () => {
  const h = harness(); const terminal = h.review(id);
  assert.equal(h.spawns.length, 0);
  const pty = terminal.options.pty; pty.open(); pty.open();
  assert.equal(h.spawns.length, 1); assert.deepEqual(h.writes, []);
  h.child.stderr.write('Approve exact target? ');
  pty.handleInput('5'); assert.deepEqual(h.writes, []);
  pty.handleInput('\r'); assert.deepEqual(h.writes, ['5\n']);
  h.child.stderr.write('Allow this operation? ');
  pty.handleInput('n\r'); assert.deepEqual(h.writes, ['5\n', 'n\n']);
  assert.deepEqual(h.spawns[0][1], ['approve', id]);
});

test('workspace distribution cannot override the user setting and duplicate review is reused', () => {
  const h = harness({ platform: 'win32', environment: { SystemRoot: 'C:\\Windows' } });
  const terminal = h.review(id); h.review(id); terminal.options.pty.open();
  assert.equal(h.terminals.length, 1); assert.equal(h.spawns[0][1][1], 'Hacocoon');
  terminal.dispose(); h.review(id); assert.equal(h.terminals.length, 2);
});

test('backspace and CRLF preserve exact answers; malformed paste cannot become approval', () => {
  const h = harness(); const pty = h.review().options.pty; pty.open();
  pty.handleInput('y'); pty.handleInput('\x7f'); pty.handleInput('n\r\n');
  pty.handleInput('\x1b[200~yes\r'); assert.deepEqual(h.writes, ['n\n']);
  pty.handleInput('yesx\r'); assert.deepEqual(h.writes, ['n\n', 'yesx\n']);
});

test('cancel, EOF, timeout, close and disposal terminate only their local child without retry', () => {
  for (const action of ['cancel', 'eof', 'timeout', 'close', 'dispose']) {
    const h = harness(); const terminal = h.review(id); const pty = terminal.options.pty; pty.open();
    if (action === 'cancel') pty.handleInput('\x03');
    if (action === 'eof') pty.handleInput('\x04');
    if (action === 'timeout') h.timers[0]();
    if (action === 'close') pty.close();
    if (action === 'dispose') h.review.dispose();
    assert.equal(h.kills, 1); assert.equal(h.spawns.length, 1); assert.deepEqual(h.writes, []);
  }
});

test('output cannot run terminal escape sequences or grow without bound', () => {
  const h = harness(); const pty = h.review(id).options.pty; pty.open();
  h.child.stderr.write('\x1b]52;c;secret\x07\n');
  assert.doesNotMatch(h.output.join(''), /\x1b|\x07/);
  assert.match(h.output.join(''), /\\u001b/);
  h.child.stdout.write('x'.repeat(256 * 1024));
  assert.equal(h.kills, 1); assert.match(h.output.join(''), /unconfirmed/);
});

test('nonzero exit and asynchronous stdin failure never display success or retry', () => {
  for (const mode of ['exit', 'stdin']) {
    const h = harness(); h.review(id).options.pty.open();
    if (mode === 'exit') h.child.emit('close', 1); else h.child.stdin.emit('error', new Error('SECRET'));
    assert.match(h.output.join(''), /unconfirmed/); assert.doesNotMatch(h.output.join(''), /SECRET/);
    assert.equal(h.spawns.length, 1);
  }
});
