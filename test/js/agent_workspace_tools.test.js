'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const repoRoot = path.resolve(__dirname, '..', '..');
const contract = require('../../clients/vscode-agent-workspace/tool-contract');
const extensionSource = fs.readFileSync(path.join(repoRoot, 'clients/vscode-agent-workspace/extension.js'), 'utf8');
const manifest = JSON.parse(fs.readFileSync(path.join(repoRoot, 'clients/vscode-agent-workspace/package.json'), 'utf8'));

function fullRecord(overrides = {}) {
  return {
    id: 'raw-session-secret',
    sessionId: 'raw-session-secret',
    repoRoot: '/repo',
    worktreeRoot: '/worktrees',
    worktreePath: '/worktrees/agent-task-1234',
    branch: 'agent/task',
    environment: 'agent-0123456789abcdef0123',
    sshAlias: 'haco-agent-0123456789abcdef',
    folderUri: 'vscode-remote://ssh-remote+haco-agent-0123456789abcdef/workspace',
    released: false,
    execution: { kind: 'local', platform: 'linux' },
    ...overrides
  };
}

function extensionHarness({ records = [] } = {}) {
  const registeredCommands = new Map();
  const registeredTools = new Map();
  const opened = [];
  const output = [];
  const state = new Map([['hacocoon.agentWorkspace.records.v1', records]]);
  const fakeAgent = {
    stateKey: 'hacocoon.agentWorkspace.records.v1',
    normalizeRecords(value) { return Array.isArray(value) ? value : []; },
    async createAgentWorkspace({ branch }) {
      return fullRecord({ branch, id: 'new-raw-session', sessionId: 'new-raw-session' });
    },
    async releaseAgentWorkspace(record) {
      return { record: { ...record, released: true }, removed: false, reason: 'dirty' };
    },
    async removeOwnedWorktree(record) {
      return { record, removed: true, reason: 'clean' };
    }
  };

  class LanguageModelTextPart {
    constructor(value) { this.value = value; }
  }
  class LanguageModelToolResult {
    constructor(content) { this.content = content; }
  }

  const vscode = {
    ProgressLocation: { Notification: 1 },
    Uri: { parse(value) { return { value }; } },
    LanguageModelTextPart,
    LanguageModelToolResult,
    workspace: {
      workspaceFolders: [{ uri: { scheme: 'file', fsPath: '/repo' } }],
      getConfiguration() {
        return { get(_key, fallback) { return fallback; } };
      }
    },
    window: {
      createOutputChannel() { return { appendLine(line) { output.push(line); } }; },
      showInputBox() { return Promise.resolve(undefined); },
      showQuickPick() { return Promise.resolve(undefined); },
      showInformationMessage() { return Promise.resolve(undefined); },
      showWarningMessage() { return Promise.resolve(undefined); },
      showErrorMessage() { return Promise.resolve(undefined); },
      withProgress(_options, callback) { return callback(); }
    },
    commands: {
      registerCommand(name, handler) {
        registeredCommands.set(name, handler);
        return { dispose() {} };
      },
      async executeCommand(name, ...args) {
        opened.push({ name, args });
      }
    },
    lm: {
      registerTool(name, implementation) {
        registeredTools.set(name, implementation);
        return { dispose() {} };
      }
    }
  };

  const module = { exports: {} };
  const sandbox = {
    module,
    exports: module.exports,
    require(name) {
      if (name === 'vscode') return vscode;
      if (name === './agent-workspace') return fakeAgent;
      if (name === './tool-contract') return contract;
      return require(name);
    },
    process: { platform: 'linux' },
    console,
    Promise,
    URL
  };
  vm.runInNewContext(extensionSource, sandbox, { filename: 'clients/vscode-agent-workspace/extension.js' });

  const context = {
    subscriptions: { push() {} },
    globalState: {
      get(key, fallback) { return state.has(key) ? state.get(key) : fallback; },
      async update(key, value) { state.set(key, value); }
    }
  };
  module.exports.activate(context);
  return { extension: module.exports, context, state, registeredCommands, registeredTools, opened, output };
}

function toolText(result) {
  assert.equal(result.content.length, 1);
  return result.content[0].value;
}

test('manifest contributes and activates all three Hacocoon language model tools', () => {
  const names = manifest.contributes.languageModelTools.map((entry) => entry.name).sort();
  assert.deepEqual(names, [
    'hacocoon_createAgentWorkspace',
    'hacocoon_listAgentWorkspaces',
    'hacocoon_releaseAgentWorkspace'
  ]);
  for (const name of names) {
    assert.ok(manifest.activationEvents.includes(`onLanguageModelTool:${name}`));
  }
});

test('activation registers all public LM tools without private Agents-window commands', () => {
  const harness = extensionHarness();
  assert.deepEqual([...harness.registeredTools.keys()].sort(), [
    'hacocoon_createAgentWorkspace',
    'hacocoon_listAgentWorkspaces',
    'hacocoon_releaseAgentWorkspace'
  ]);
  assert.equal(JSON.stringify(manifest).includes('workbench.action.chat'), false);
  assert.equal(extensionSource.includes('_workbench'), false);
});

test('safe tool projection never exposes raw session identity or SSH-private metadata', () => {
  const safe = contract.safeWorkspaceRecord(fullRecord());
  const text = JSON.stringify(safe);
  assert.equal('sessionId' in safe, false);
  assert.equal('id' in safe, false);
  assert.equal('sshAlias' in safe, false);
  assert.doesNotMatch(text, /raw-session-secret/);
  assert.equal(safe.branch, 'agent/task');
  assert.equal(safe.environment, 'agent-0123456789abcdef0123');
});

test('create tool confirms host mutation, shares provisioning backend, and can open only returned remote folder', async () => {
  const harness = extensionHarness();
  const create = harness.registeredTools.get('hacocoon_createAgentWorkspace');
  const prepared = create.prepareInvocation({ input: { branch: 'agent/new' } });
  assert.match(prepared.confirmationMessages.title, /Create Hacocoon agent workspace/);
  assert.match(prepared.confirmationMessages.message, /Provider credentials are not copied/);

  const result = await create.invoke({ input: { branch: 'agent/new', open: true } });
  const parsed = JSON.parse(toolText(result));
  assert.equal(parsed.action, 'create');
  assert.equal(parsed.workspace.branch, 'agent/new');
  assert.equal(parsed.opened, true);
  assert.equal(parsed.nativeAgentSessionStarted, false);
  assert.doesNotMatch(JSON.stringify(parsed), /new-raw-session|sessionId/);
  assert.equal(harness.opened.length, 1);
  assert.equal(harness.opened[0].name, 'vscode.openFolder');
  assert.equal(harness.opened[0].args[0].value, fullRecord().folderUri);
});

test('create tool defaults to batch-friendly no-open behavior', async () => {
  const harness = extensionHarness();
  const create = harness.registeredTools.get('hacocoon_createAgentWorkspace');
  const result = await create.invoke({ input: { branch: 'agent/batch' } });
  const parsed = JSON.parse(toolText(result));
  assert.equal(parsed.opened, false);
  assert.equal(harness.opened.length, 0);
});

test('list tool returns safe orchestration metadata only', async () => {
  const harness = extensionHarness({ records: [fullRecord()] });
  const list = harness.registeredTools.get('hacocoon_listAgentWorkspaces');
  const parsed = JSON.parse(toolText(await list.invoke({ input: {} })));
  assert.equal(parsed.action, 'list');
  assert.equal(parsed.workspaces.length, 1);
  assert.equal(parsed.workspaces[0].branch, 'agent/task');
  assert.doesNotMatch(JSON.stringify(parsed), /raw-session-secret|sessionId|worktreePath|sshAlias/);
});

test('release selection exact-matches branch and fails closed on missing or ambiguous records', () => {
  assert.equal(contract.selectOwnedRecordByBranch([fullRecord()], 'agent/task').environment, fullRecord().environment);
  assert.throws(() => contract.selectOwnedRecordByBranch([fullRecord()], 'agent/missing'), /no VS Code-owned/);
  assert.throws(() => contract.selectOwnedRecordByBranch([
    fullRecord(), fullRecord({ id: 'other', sessionId: 'other' })
  ], 'agent/task'), /ambiguous/);
});

test('release tool confirms destructive boundary, reuses release backend, and keeps internal session identity private', async () => {
  const harness = extensionHarness({ records: [fullRecord()] });
  const release = harness.registeredTools.get('hacocoon_releaseAgentWorkspace');
  const prepared = release.prepareInvocation({ input: { branch: 'agent/task' } });
  assert.match(prepared.confirmationMessages.title, /Release Hacocoon agent workspace/);
  assert.match(prepared.confirmationMessages.message, /dirty worktree is preserved/);

  const parsed = JSON.parse(toolText(await release.invoke({ input: { branch: 'agent/task' } })));
  assert.equal(parsed.action, 'release');
  assert.equal(parsed.workspace.released, true);
  assert.equal(parsed.worktreeRemoved, false);
  assert.equal(parsed.reason, 'dirty');
  assert.doesNotMatch(JSON.stringify(parsed), /raw-session-secret|sessionId/);
  const persisted = harness.state.get('hacocoon.agentWorkspace.records.v1');
  assert.equal(persisted[0].released, true);
});

test('tool inputs reject empty branch and non-boolean open values before provisioning', async () => {
  const harness = extensionHarness();
  const create = harness.registeredTools.get('hacocoon_createAgentWorkspace');
  assert.throws(() => create.prepareInvocation({ input: { branch: '' } }), /branch is required/);
  await assert.rejects(() => create.invoke({ input: { branch: 'agent/task', open: 'yes' } }), /open must be a boolean/);
});
