"use strict";

const childProcess = require('node:child_process');
const crypto = require('node:crypto');
const os = require('node:os');
const path = require('node:path');
const { panelHTML } = require('./review_panel');
const validID = value => typeof value === 'string' && /^[a-f0-9]{32}$/.test(value);
const validToken = value => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);

// Executable, arguments and environment never come from the workspace or event.
function launchSpec(platform, environment, directory, distro = 'Hacocoon', requestID) {
  if (requestID !== undefined && !validID(requestID)) throw new Error('invalid request');
  if (typeof distro !== 'string' || !/^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/.test(distro)) throw new Error('invalid distribution');
  const args = ['_desktop-review'];
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

function validView(view) {
  return view && validID(view.request?.request_id) && validToken(view.token) && validToken(view.digest) &&
    typeof view.request.request?.capability === 'string' && typeof view.request.request.action === 'string' &&
    Array.isArray(view.options) && view.options.length <= 6 && new Set(view.options.map(o => o.choice)).size === view.options.length &&
    view.options.every(o => typeof o.choice === 'string' && /^(allow|deny|ask)-(environment|global)$/.test(o.choice) &&
      o.scope && ['allow', 'deny', 'require-approval'].includes(o.scope.decision));
}

function createReview(vscode, { spawn = childProcess.spawn, platform = process.platform,
  environment = process.env, directory = os.homedir(), localUI = false, schedule = setTimeout, unschedule = clearTimeout } = {}) {
  let active;
  const allowed = () => localUI && vscode.env.uiKind === vscode.UIKind.Desktop && vscode.workspace.isTrusted;
  function review(requestID) {
    if (!allowed()) {
      void vscode.window.showWarningMessage('Open Hacocoon review in a trusted desktop VS Code window.');
      return;
    }
    let spec, distro;
    try {
      // A workspace setting must never select the trusted controller.
      distro = vscode.workspace.getConfiguration('hacocoon.review').inspect('wslDistribution')?.globalValue ?? 'Hacocoon';
      spec = launchSpec(platform, environment, directory, distro, requestID);
    } catch (_) {
      void vscode.window.showErrorMessage('Hacocoon review requires a supported local installation and valid request.');
      return;
    }
    if (active) { active.panel.reveal(); if (requestID) active.select(requestID); return active.panel; }
    const panel = vscode.window.createWebviewPanel('hacocoon.approval', 'Hacocoon Approval', vscode.ViewColumn.Active,
      { enableScripts: true, enableCommandUris: false, localResourceRoots: [] });
    let child, exited = false, closed = false, ready = false, inflight, selected, sequence = 0;
    let pending = [], buffer = '', stderrBytes = 0, poll, deadline, initial = requestID;
    const stop = () => {
      if (closed) return;
      closed = true;
      if (poll) unschedule(poll);
      if (deadline) unschedule(deadline);
      if (child && !exited) child.kill();
      if (active?.panel === panel) active = undefined;
    };
    const post = message => {
      if (closed) return;
      Promise.resolve(panel.webview.postMessage(message)).catch(stop);
    };
    const fail = () => { post({ type: 'unavailable' }); stop(); };
    const send = (action, fields = {}) => {
      if (closed || !ready || inflight) return false;
      if (!allowed()) { fail(); return false; }
      if (poll) { unschedule(poll); poll = undefined; }
      inflight = { version: 1, sequence: ++sequence, action, ...fields };
      post({ type: 'busy' });
      try { child.stdin.write(JSON.stringify(inflight) + '\n'); } catch (_) { fail(); return false; }
      return true;
    };
    const select = id => {
      if (!ready || inflight) { initial = id; return; }
      if (!pending.some(p => p.request_id === id)) { initial = id; send('list'); return; }
      selected = undefined;
      send('select', { request_id: id });
    };
    const receive = reply => {
      if (!inflight || reply?.version !== 1 || reply.sequence !== inflight.sequence) { fail(); return; }
      const sent = inflight; inflight = undefined;
      if (reply.type === 'error' && typeof reply.error === 'string') {
        selected = undefined; post({ type: 'error', error: reply.error });
      } else if (sent.action === 'list' && reply.type === 'pending') {
        const items = reply.pending ?? [];
        if (!Array.isArray(items) || items.length > 128 || new Set(items.map(p => p.request_id)).size !== items.length ||
          items.some(p => !validID(p.request_id) || typeof p.capability !== 'string' || typeof p.action !== 'string')) { fail(); return; }
        pending = items;
        if (selected && !pending.some(p => p.request_id === selected.request.request_id)) selected = undefined;
        post({ type: 'pending', pending, distribution: platform === 'win32' ? distro : 'local', selected: selected?.request.request_id });
      } else if (sent.action === 'select' && reply.type === 'selected' && validView(reply.view) && reply.view.request.request_id === sent.request_id) {
        selected = reply.view; post({ type: 'selected', view: selected });
      } else if (sent.action === 'decide' && reply.type === 'result' &&
          (reply.result?.request_id === sent.request_id || reply.error === 'outcome_unconfirmed')) {
        selected = undefined;
        post({ type: 'result', result: reply.result, error: reply.error, approved: sent.approved, save: sent.save });
      } else { fail(); return; }
      if (initial) {
        const id = initial; initial = undefined;
        // A notification selects a matching current request; it never answers it.
        selected = undefined;
        send('select', { request_id: id }); return;
      }
      poll = schedule(() => send('list'), 5000);
    };
    panel.webview.onDidReceiveMessage(message => {
      if (closed) {
        if (message?.type === 'ready') void Promise.resolve(panel.webview.postMessage({type: 'unavailable'})).catch(() => {});
        return;
      }
      if (!allowed()) { fail(); return; }
      if (!message || typeof message !== 'object') return;
      if (message.type === 'ready' && !ready) { ready = true; send('list'); return; }
      if (!ready || inflight) return;
      if (message.type === 'list') send('list');
      else if (message.type === 'select' && validID(message.request_id) && pending.some(p => p.request_id === message.request_id)) select(message.request_id);
      else if (message.type === 'decide' && selected && message.request_id === selected.request.request_id &&
          message.token === selected.token && typeof message.approved === 'boolean' && typeof message.save === 'string') {
        const option = selected.options.find(o => o.choice === message.save);
        if (message.save !== '' && (!option || (option.scope.decision === 'allow' && !message.approved) ||
            (option.scope.decision === 'deny' && message.approved))) return;
        selected = undefined;
        send('decide', { request_id: message.request_id, token: message.token, approved: message.approved, save: message.save });
      }
    });
    panel.onDidDispose(stop);
    active = { panel, select };
    try {
      child = spawn(spec.file, spec.args, spec.options);
      child.stdout.setEncoding('utf8'); child.stderr.setEncoding('utf8');
      child.stdout.on('data', data => {
        if (closed) return;
        buffer += data;
        if (Buffer.byteLength(buffer) > 256 * 1024) { fail(); return; }
        let at;
        while (!closed && (at = buffer.indexOf('\n')) !== -1) {
          const line = buffer.slice(0, at); buffer = buffer.slice(at + 1);
          try { receive(JSON.parse(line)); } catch (_) { fail(); }
        }
      });
      child.stderr.on('data', data => { stderrBytes += Buffer.byteLength(data); if (stderrBytes > 16 * 1024) fail(); });
      child.stdin.on('error', fail); child.on('error', fail);
      child.on('close', () => { exited = true; fail(); });
      deadline = schedule(fail, 15 * 60 * 1000);
    } catch (_) { fail(); }
    panel.webview.html = panelHTML(crypto.randomBytes(18).toString('hex'), /^ja(?:-|$)/i.test(vscode.env.language ?? ''));
    return panel;
  }
  review.dispose = () => { if (active) active.panel.dispose(); };
  return review;
}

module.exports = { createReview, launchSpec, validView };
