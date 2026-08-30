'use strict';

const vscode = require('vscode');
const http = require('http');
const https = require('https');

const maxSeen = 512;
let timer;
let stopped = false;

function activate(context) {
  const output = vscode.window.createOutputChannel('Hacocoon Notifications');
  context.subscriptions.push(output);
  stopped = false;

  const poll = async () => {
    if (stopped) return;
    const config = vscode.workspace.getConfiguration('hacocoon.notifications');
    const endpointText = config.get('endpoint', 'http://127.0.0.1:18081');
    const pollIntervalMs = Math.max(500, config.get('pollIntervalMs', 1500));
    const includeCompleted = config.get('includeCompleted', false);

    let endpoint;
    try {
      endpoint = validateEndpoint(endpointText);
      const cursorKey = `hacocoon.notifications.cursor.v1:${endpoint.origin}`;
      const seenKey = `hacocoon.notifications.seen.v1:${endpoint.origin}`;
      let cursor = context.globalState.get(cursorKey, 0);
      if (!Number.isSafeInteger(cursor) || cursor < 0) cursor = 0;
      let seen = context.globalState.get(seenKey, []);
      if (!Array.isArray(seen)) seen = [];
      seen = seen.filter((value) => typeof value === 'string').slice(-maxSeen);

      const url = new URL('/api/v1/events', endpoint);
      url.searchParams.set('offset', String(cursor));
      url.searchParams.set('limit', '100');
      const batch = await requestJson(url);

      for (const event of batch.events || []) {
        if (!seen.includes(event.event_id)) {
          await present(event, includeCompleted);
          if (event.event_id) {
            seen.push(event.event_id);
            if (seen.length > maxSeen) seen = seen.slice(-maxSeen);
            await context.globalState.update(seenKey, seen);
          }
        }
        if (Number.isSafeInteger(event.next_offset) && event.next_offset >= cursor) {
          cursor = event.next_offset;
          await context.globalState.update(cursorKey, cursor);
        }
      }

      if (Number.isSafeInteger(batch.next_offset) && batch.next_offset >= cursor) {
        cursor = batch.next_offset;
        await context.globalState.update(cursorKey, cursor);
      }
      if (batch.error) {
        output.appendLine(`interaction stream paused: ${batch.error.code}`);
        await vscode.window.showWarningMessage(`Hacocoon interaction stream paused: ${batch.error.code}`);
        return;
      }
    } catch (error) {
      output.appendLine(`poll failed: ${error.message}`);
    }

    timer = setTimeout(poll, pollIntervalMs);
  };

  poll();
}

function deactivate() {
  stopped = true;
  if (timer) clearTimeout(timer);
}

function validateEndpoint(value) {
  const url = new URL(value);
  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    throw new Error('Hacocoon notification endpoint must use HTTP(S)');
  }
  const host = url.hostname.toLowerCase();
  if (host !== 'localhost' && host !== '127.0.0.1' && host !== '::1') {
    throw new Error('Hacocoon notification endpoint must be loopback-only');
  }
  return url;
}

function requestJson(url) {
  return new Promise((resolve, reject) => {
    const transport = url.protocol === 'https:' ? https : http;
    const request = transport.get(url, {
      headers: { 'Accept': 'application/json' }
    }, (response) => {
      let body = '';
      response.setEncoding('utf8');
      response.on('data', (chunk) => {
        body += chunk;
        if (body.length > 1024 * 1024) {
          request.destroy(new Error('Hacocoon notification response exceeded 1 MiB'));
        }
      });
      response.on('end', () => {
        if (response.statusCode !== 200) {
          reject(new Error(`Hacocoon notification endpoint returned HTTP ${response.statusCode}`));
          return;
        }
        try {
          resolve(JSON.parse(body));
        } catch (error) {
          reject(new Error(`invalid Hacocoon notification response: ${error.message}`));
        }
      });
    });
    request.setTimeout(5000, () => request.destroy(new Error('Hacocoon notification request timed out')));
    request.on('error', reject);
  });
}

async function present(event, includeCompleted) {
  const details = [event.environment, event.capability, event.action].filter(Boolean).join(' · ');
  switch (event.kind) {
    case 'approval-required':
      await vscode.window.showWarningMessage(message('Hacocoon approval required', details));
      break;
    case 'recovery-required':
      await vscode.window.showErrorMessage(message('Hacocoon needs recovery', details || event.code));
      break;
    case 'operation-failed':
      await vscode.window.showErrorMessage(message('Hacocoon operation failed', details || event.code));
      break;
    case 'policy-denied':
      await vscode.window.showWarningMessage(message('Hacocoon policy denied', details || event.code));
      break;
    case 'approval-denied':
      await vscode.window.showWarningMessage(message('Hacocoon approval denied', details || event.code));
      break;
    case 'operation-completed':
      if (includeCompleted) {
        await vscode.window.showInformationMessage(message('Hacocoon operation completed', details));
      }
      break;
  }
}

function message(title, details) {
  return details ? `${title} — ${details}` : title;
}

module.exports = { activate, deactivate, validateEndpoint, present };
