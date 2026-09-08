"use strict";

const childProcess = require('node:child_process');
const os = require('node:os');
const path = require('node:path');

// Executable, arguments and environment never come from the workspace or event.
function launchSpec(platform, environment, directory, distro = 'Hacocoon', requestID) {
  if (requestID !== undefined && (typeof requestID !== 'string' || !/^[a-f0-9]{32}$/.test(requestID))) throw new Error('invalid request');
  if (typeof distro !== 'string' || !/^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/.test(distro)) throw new Error('invalid distribution');
  const args = ['approve', ...(requestID ? [requestID] : [])];
  const options = { shell: false, windowsHide: true, cwd: directory, stdio: ['pipe', 'pipe', 'pipe'] };
  if (platform === 'win32') {
    const root = environment.SystemRoot;
    if (typeof root !== 'string' || !/^[A-Za-z]:\\[^\r\n]+$/.test(root)) throw new Error('invalid Windows root');
    const env = { SystemRoot: root, WINDIR: root };
    for (const key of ['USERPROFILE', 'LOCALAPPDATA', 'TEMP', 'TMP']) {
      if (typeof environment[key] === 'string') env[key] = environment[key];
    }
    return { file: path.win32.join(root, 'System32', 'wsl.exe'),
      args: ['--distribution', distro, '--exec', '/usr/bin/env', '-i', 'PATH=/usr/local/bin:/usr/bin:/bin', 'LANG=C.UTF-8', '/usr/local/bin/haco', ...args],
      options: { ...options, env } };
  }
  if (platform !== 'linux') throw new Error('unsupported platform');
  return { file: '/usr/local/bin/haco', args,
    options: { ...options, env: { PATH: '/usr/local/bin:/usr/bin:/bin', LANG: 'C.UTF-8' } } };
}

function createReview(vscode, { spawn = childProcess.spawn, platform = process.platform,
  environment = process.env, directory = os.homedir(), localUI = false, schedule = setTimeout, unschedule = clearTimeout } = {}) {
  const terminals = new Map();
  function review(requestID) {
    // UI extension host only. A workspace/remote extension host must never launch it.
    if (!localUI || vscode.env.uiKind !== vscode.UIKind.Desktop || !vscode.workspace.isTrusted) {
      void vscode.window.showWarningMessage('Open Hacocoon review in a trusted desktop VS Code window.');
      return;
    }
    const key = requestID === undefined ? 'pending' : requestID;
    if (terminals.has(key)) { terminals.get(key).show(); return; }
    let spec;
    try {
      const config = vscode.workspace.getConfiguration('hacocoon.review');
      // Even malformed workspace settings cannot override the local user value.
      const distro = config.inspect('wslDistribution')?.globalValue ?? 'Hacocoon';
      spec = launchSpec(platform, environment, directory, distro, requestID);
    } catch (_) {
      void vscode.window.showErrorMessage('Hacocoon review requires a supported local installation and valid request.');
      return;
    }
    const write = new vscode.EventEmitter();
    const close = new vscode.EventEmitter();
    let child, deadline, finished = false, disposed = false, started = false, exited = false;
    let line = '', bytes = 0, exitCode = 1;
    const emit = (text) => { if (!disposed) write.fire(text); };
    const stop = () => { if (child && !exited) child.kill(); };
    const finish = (code) => {
      if (finished || disposed) return;
      finished = true;
      exitCode = Number.isInteger(code) && code >= 0 ? code : 1;
      if (deadline) unschedule(deadline);
      emit(`\r\nReview ended (exit ${exitCode}). Press Enter to close.\r\n`);
      if (exitCode !== 0) emit('Outcome may be unconfirmed. Inspect Policy and audit before retrying.\r\n');
    };
    const output = (data) => {
      if (finished || disposed) return;
      bytes += Buffer.byteLength(data);
      if (bytes > 256 * 1024) { stop(); finish(1); return; }
      // Do not interpret ANSI/OSC from a subprocess, even in a trusted review pane.
      emit(String(data).replace(/[\x00-\x08\x0b-\x1f\x7f-\x9f]/g,
        (value) => `\\u${value.charCodeAt(0).toString(16).padStart(4, '0')}`).replace(/\n/g, '\r\n'));
    };
    const pty = {
      onDidWrite: write.event, onDidClose: close.event,
      open() {
        if (started || disposed) return;
        started = true;
        emit('Hacocoon trusted local review — answer only after inspecting the complete scope.\r\n');
        try {
          child = spawn(spec.file, spec.args, spec.options);
          child.stdout.setEncoding('utf8'); child.stderr.setEncoding('utf8');
          child.stdout.on('data', output); child.stderr.on('data', output);
          child.on('error', () => finish(1));
          child.stdin.on('error', () => { stop(); finish(1); });
          child.on('close', (code) => { exited = true; finish(code); });
          deadline = schedule(() => { stop(); finish(1); }, 15 * 60 * 1000);
        } catch (_) { stop(); finish(1); }
      },
      close() {
        if (disposed) return;
        stop(); disposed = true;
        if (deadline) unschedule(deadline);
        terminals.delete(key); write.dispose(); close.dispose();
      },
      handleInput(data) {
        if (disposed || !started) return;
        if (finished) { if (data === '\r' || data === '\n') close.fire(exitCode); return; }
        if (data === '\x03' || data === '\x04') { stop(); finish(1); return; }
        if (data === '\x7f' || data === '\b') {
          if (line) { line = line.slice(0, -1); emit('\b \b'); }
          return;
        }
        // Reject control sequences as a whole; never turn a malformed paste into yes.
        if (typeof data !== 'string' || /[^\x20-\x7e\r\n]/.test(data) || data.length > 4096) return;
        for (const character of data.replace(/\r\n/g, '\n')) {
          if (character === '\r' || character === '\n') {
            child.stdin.write(line + '\n'); line = ''; emit('\r\n');
          } else {
            if (line.length >= 128) { stop(); finish(1); return; }
            line += character; emit(character);
          }
        }
      }
    };
    const terminal = vscode.window.createTerminal({ name: 'Hacocoon Approval (local)', pty });
    terminals.set(key, terminal);
    terminal.show();
    return terminal;
  }
  review.dispose = () => { for (const terminal of terminals.values()) terminal.dispose(); terminals.clear(); };
  return review;
}

module.exports = { createReview, launchSpec };
