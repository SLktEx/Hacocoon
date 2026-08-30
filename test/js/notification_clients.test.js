'use strict';

const assert = require('node:assert/strict');
const { EventEmitter } = require('node:events');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const repoRoot = path.resolve(__dirname, '..', '..');
const browserSource = fs.readFileSync(path.join(repoRoot, 'pkg/interactionhttp/web/app.js'), 'utf8');
const extensionSource = fs.readFileSync(path.join(repoRoot, 'clients/vscode-notify/extension.js'), 'utf8');

async function settle() {
  for (let i = 0; i < 6; i += 1) {
    await new Promise((resolve) => setImmediate(resolve));
  }
}

function browserHarness({ storage = {}, batch, completed = false }) {
  const values = new Map(Object.entries(storage));
  const notifications = [];
  const fetches = [];
  const scheduled = [];
  const status = { textContent: '' };
  const events = {
    children: [],
    prepend(item) {
      this.children.unshift(item);
    },
    get lastElementChild() {
      return this.children[this.children.length - 1];
    }
  };
  const enable = {
    addEventListener(_name, callback) {
      this.callback = callback;
    }
  };
  const completedCheckbox = { checked: completed };
  const registration = {
    async showNotification(title, options) {
      notifications.push({ title, ...options });
    }
  };

  const sandbox = {
    document: {
      getElementById(id) {
        return { status, events, enable, completed: completedCheckbox }[id];
      },
      createElement() {
        return { textContent: '', remove() {} };
      }
    },
    localStorage: {
      getItem(key) {
        return values.has(key) ? values.get(key) : null;
      },
      setItem(key, value) {
        values.set(key, String(value));
      }
    },
    navigator: {
      serviceWorker: {
        async register() {
          return registration;
        },
        ready: Promise.resolve(registration)
      }
    },
    Notification: {
      permission: 'granted',
      async requestPermission() {
        return 'granted';
      }
    },
    async fetch(url, options) {
      fetches.push({ url, options });
      return {
        ok: true,
        status: 200,
        async json() {
          return batch;
        }
      };
    },
    window: {
      setTimeout(callback, delay) {
        scheduled.push({ callback, delay });
        return scheduled.length;
      }
    },
    console,
    encodeURIComponent,
    JSON,
    Number,
    Promise
  };

  vm.runInNewContext(browserSource, sandbox, { filename: 'pkg/interactionhttp/web/app.js' });
  return { values, notifications, fetches, scheduled, status, events };
}

function extensionHarness({ batch, state = {}, endpoint = 'http://127.0.0.1:18081' } = {}) {
  const messages = [];
  const output = [];
  const requests = [];
  const scheduled = [];
  const stateMap = new Map(Object.entries(state));

  const vscode = {
    window: {
      createOutputChannel() {
        return {
          appendLine(line) {
            output.push(line);
          }
        };
      },
      showWarningMessage(...args) {
        messages.push({ kind: 'warning', args });
        return Promise.resolve(undefined);
      },
      showErrorMessage(...args) {
        messages.push({ kind: 'error', args });
        return Promise.resolve(undefined);
      },
      showInformationMessage(...args) {
        messages.push({ kind: 'information', args });
        return Promise.resolve(undefined);
      }
    },
    workspace: {
      getConfiguration() {
        return {
          get(key, fallback) {
            if (key === 'endpoint') return endpoint;
            if (key === 'pollIntervalMs') return 1500;
            if (key === 'includeCompleted') return false;
            return fallback;
          }
        };
      }
    }
  };

  const transport = {
    get(url, _options, callback) {
      requests.push(url.toString());
      const request = new EventEmitter();
      request.setTimeout = () => {};
      request.destroy = (error) => {
        if (error) queueMicrotask(() => request.emit('error', error));
      };
      queueMicrotask(() => {
        const response = new EventEmitter();
        response.statusCode = 200;
        response.setEncoding = () => {};
        callback(response);
        response.emit('data', JSON.stringify(batch || { events: [], next_offset: 0 }));
        response.emit('end');
      });
      return request;
    }
  };

  const module = { exports: {} };
  const sandbox = {
    module,
    exports: module.exports,
    require(name) {
      if (name === 'vscode') return vscode;
      if (name === 'http' || name === 'https') return transport;
      return require(name);
    },
    URL,
    console,
    Promise,
    setTimeout(callback, delay) {
      scheduled.push({ callback, delay });
      return scheduled.length;
    },
    clearTimeout() {}
  };
  vm.runInNewContext(extensionSource, sandbox, { filename: 'clients/vscode-notify/extension.js' });

  const context = {
    subscriptions: { push() {} },
    globalState: {
      get(key, fallback) {
        return stateMap.has(key) ? stateMap.get(key) : fallback;
      },
      async update(key, value) {
        stateMap.set(key, value);
      }
    }
  };
  return { extension: module.exports, context, stateMap, messages, output, requests, scheduled };
}

test('browser client resumes, deduplicates, and never renders extra sensitive fields', async () => {
  const harness = browserHarness({
    storage: {
      'hacocoon.notifications.cursor.v1': '17',
      'hacocoon.notifications.seen.v1': JSON.stringify(['old:event'])
    },
    batch: {
      events: [
        { event_id: 'old:event', request_id: 'old', kind: 'approval-required', environment: 'old', next_offset: 20 },
        { event_id: 'new:event', request_id: 'new', kind: 'approval-required', environment: 'dev', capability: 'git.push', action: 'push', credential: 'TOP-SECRET', next_offset: 30 },
        { event_id: 'new:event', request_id: 'new', kind: 'approval-required', environment: 'dev', capability: 'git.push', action: 'push', credential: 'TOP-SECRET', next_offset: 40 }
      ],
      next_offset: 50
    }
  });
  await settle();

  assert.equal(harness.fetches.length, 1);
  assert.match(harness.fetches[0].url, /offset=17/);
  assert.equal(harness.fetches[0].options.cache, 'no-store');
  assert.equal(harness.notifications.length, 1);
  assert.equal(harness.notifications[0].tag, 'new:event');
  assert.equal(harness.notifications[0].body, 'dev · git.push · push');
  assert.doesNotMatch(JSON.stringify(harness.notifications[0]), /TOP-SECRET|credential/);
  assert.equal(harness.events.children.length, 1);
  assert.equal(harness.events.children[0].textContent, 'approval-required · dev · git.push · push');
  assert.doesNotMatch(harness.events.children[0].textContent, /TOP-SECRET/);
  assert.equal(harness.values.get('hacocoon.notifications.cursor.v1'), '50');
  assert.deepEqual(JSON.parse(harness.values.get('hacocoon.notifications.seen.v1')), ['old:event', 'new:event']);
  assert.equal(harness.scheduled.length, 1);
});

test('browser client commits trustworthy prefix and stops polling on corruption', async () => {
  const harness = browserHarness({
    batch: {
      events: [{ event_id: 'safe:event', request_id: 'safe', kind: 'operation-failed', environment: 'dev', next_offset: 80 }],
      next_offset: 80,
      error: { code: 'source-corruption', line: 3, byte_offset: 91, kind: 'malformed-json' }
    }
  });
  await settle();

  assert.equal(harness.notifications.length, 1);
  assert.equal(harness.values.get('hacocoon.notifications.cursor.v1'), '80');
  assert.equal(harness.status.textContent, 'Paused: source-corruption');
  assert.equal(harness.scheduled.length, 0);
});

test('VS Code client only accepts loopback endpoints', () => {
  const harness = extensionHarness();
  for (const endpoint of ['http://localhost:18081', 'http://127.0.0.1:18081', 'http://[::1]:18081']) {
    assert.doesNotThrow(() => harness.extension.validateEndpoint(endpoint));
  }
  for (const endpoint of ['http://0.0.0.0:18081', 'http://192.0.2.1:18081', 'https://example.com/']) {
    assert.throws(() => harness.extension.validateEndpoint(endpoint), /loopback-only/);
  }
  assert.throws(() => harness.extension.validateEndpoint('file:///tmp/events'), /HTTP\(S\)/);
});

test('VS Code client resumes, deduplicates, and keeps notifications presentation-only', async () => {
  const origin = 'http://127.0.0.1:18081';
  const harness = extensionHarness({
    state: {
      [`hacocoon.notifications.cursor.v1:${origin}`]: 17,
      [`hacocoon.notifications.seen.v1:${origin}`]: ['old:event']
    },
    batch: {
      events: [
        { event_id: 'old:event', request_id: 'old', kind: 'approval-required', environment: 'old', next_offset: 20 },
        { event_id: 'new:event', request_id: 'new', kind: 'approval-required', environment: 'dev', capability: 'git.push', action: 'push', approval_token: 'TOP-SECRET', next_offset: 30 },
        { event_id: 'new:event', request_id: 'new', kind: 'approval-required', environment: 'dev', capability: 'git.push', action: 'push', approval_token: 'TOP-SECRET', next_offset: 40 }
      ],
      next_offset: 50
    }
  });

  harness.extension.activate(harness.context);
  await settle();
  harness.extension.deactivate();

  assert.equal(harness.requests.length, 1);
  assert.match(harness.requests[0], /offset=17/);
  const eventMessages = harness.messages.filter((entry) => entry.args[0] === 'Hacocoon approval required — dev · git.push · push');
  assert.equal(eventMessages.length, 1);
  assert.equal(eventMessages[0].args.length, 1, 'notification must not expose an approval action button');
  assert.doesNotMatch(JSON.stringify(harness.messages), /TOP-SECRET|approval_token/);
  assert.equal(harness.stateMap.get(`hacocoon.notifications.cursor.v1:${origin}`), 50);
  assert.deepEqual(harness.stateMap.get(`hacocoon.notifications.seen.v1:${origin}`), ['old:event', 'new:event']);
  assert.equal(harness.scheduled.length, 1);
});

test('VS Code client stops polling when the bridge reports source corruption', async () => {
  const harness = extensionHarness({
    batch: {
      events: [{ event_id: 'safe:event', request_id: 'safe', kind: 'operation-failed', environment: 'dev', next_offset: 80 }],
      next_offset: 80,
      error: { code: 'source-corruption' }
    }
  });

  harness.extension.activate(harness.context);
  await settle();
  harness.extension.deactivate();

  assert.ok(harness.messages.some((entry) => entry.kind === 'error' && /operation failed/.test(entry.args[0])));
  assert.ok(harness.messages.some((entry) => entry.kind === 'warning' && /stream paused: source-corruption/.test(entry.args[0])));
  assert.ok(harness.output.some((line) => /interaction stream paused: source-corruption/.test(line)));
  assert.equal(harness.scheduled.length, 0);
});
