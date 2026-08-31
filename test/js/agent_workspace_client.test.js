'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path').posix;
const test = require('node:test');

const agent = require('../../clients/vscode-agent-workspace/agent-workspace');

function descriptor() {
  return {
    session_id: 'opaque',
    environment: 'agent-0123456789abcdef0123',
    workspace_path: '/tmp/worktree',
    remote_workspace: '/workspace',
    ssh_alias: 'haco-agent-0123456789abcdef',
    host_port: 2222,
    folder_uri: 'vscode-remote://ssh-remote+haco-agent-0123456789abcdef/workspace'
  };
}

test('extension manifest registers create/release commands as a Windows UI extension', () => {
  const manifestPath = require('node:path').resolve(__dirname, '../../clients/vscode-agent-workspace/package.json');
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
  assert.deepEqual(manifest.extensionKind, ['ui']);
  const commands = new Map(manifest.contributes.commands.map((entry) => [entry.command, entry.title]));
  assert.equal(commands.get('hacocoon.newAgentWorkspace'), 'Hacocoon: New Agent Workspace');
  assert.equal(commands.get('hacocoon.releaseAgentWorkspace'), 'Hacocoon: Release Agent Workspace');
  assert.equal(manifest.contributes.configuration.properties['hacocoon.agentWorkspace.wslDistro'].default, 'Hacocoon');
});

test('descriptor validation binds the folder URI to the exact prepared Hacocoon SSH alias', () => {
  assert.equal(agent.validateDescriptor(JSON.stringify(descriptor())).environment, descriptor().environment);
  assert.throws(() => agent.validateDescriptor({ ...descriptor(), remote_workspace: '/root' }), /remote workspace/);
  assert.throws(() => agent.validateDescriptor({ ...descriptor(), ssh_alias: 'prod-server' }), /SSH alias/);
  assert.throws(() => agent.validateDescriptor({ ...descriptor(), folder_uri: 'file:///workspace' }), /remote folder URI/);
  assert.throws(() => agent.validateDescriptor({ ...descriptor(), folder_uri: 'vscode-remote://ssh-remote+haco-agent-deadbeef/workspace' }), /remote folder URI/);
});

test('WSL invocation wraps executable and argument arrays without shell interpolation', () => {
  const invocation = agent.buildInvocation('git', ['-C', '/repo with spaces', 'status', ';echo nope'], {
    executable: 'wsl.exe',
    args: ['-d', 'Hacocoon', '--']
  });
  assert.equal(invocation.executable, 'wsl.exe');
  assert.deepEqual(invocation.args, ['-d', 'Hacocoon', '--', 'git', '-C', '/repo with spaces', 'status', ';echo nope']);
  assert.deepEqual(agent.adapterPrepareArgs('opaque id', '/tmp/path with spaces/$HOME'), [
    'prepare', '--session', 'opaque id', '--json', '--no-launch', '/tmp/path with spaces/$HOME'
  ]);
});

test('worktree paths stay under an owned absolute Linux root and never equal the main checkout', () => {
  const repo = '/home/user/repos/demo';
  const got = agent.deriveWorktreePath(repo, '/home/user/worktrees', 'feature/API thing', '01234567-aaaa');
  assert.equal(got.root, '/home/user/worktrees');
  assert.equal(got.worktreePath, '/home/user/worktrees/feature-api-thing-01234567');
  assert.equal(agent.isPathInside(got.root, got.worktreePath), true);
  assert.throws(() => agent.normalizeWorktreeRoot('relative/path', repo), /absolute Linux path/);
  assert.throws(() => agent.assertOwnedWorktree({ repoRoot: repo, worktreeRoot: '/home/user/worktrees', worktreePath: repo, sessionId: 'x' }), /main checkout/);
  assert.throws(() => agent.assertOwnedWorktree({ repoRoot: repo, worktreeRoot: '/home/user/worktrees', worktreePath: '/tmp/other', sessionId: 'x' }), /outside/);
});

test('create flow checks branch, creates linked worktree, prepares Hacocoon, and returns state', async () => {
  const calls = [];
  const runner = async (command, args) => {
    calls.push({ command, args });
    if (command === 'git' && args.includes('rev-parse')) return { stdout: '/home/user/repos/demo\n', stderr: '' };
    if (command === 'git' && args.includes('check-ref-format')) return { stdout: 'agent/task\n', stderr: '' };
    if (command === 'mkdir') return { stdout: '', stderr: '' };
    if (command === 'git' && args.includes('worktree') && args.includes('add')) return { stdout: '', stderr: '' };
    if (command === 'haco-agent-host' && args[0] === 'prepare') return { stdout: JSON.stringify(descriptor()), stderr: '' };
    throw new Error(`unexpected call: ${command} ${args.join(' ')}`);
  };
  const record = await agent.createAgentWorkspace({
    workspacePath: '/home/user/repos/demo',
    branch: 'agent/task',
    configuredRoot: '/home/user/worktrees',
    runner,
    runnerConfig: { executable: 'wsl.exe', args: ['-d', 'Hacocoon', '--'] },
    sessionId: '01234567-89ab-cdef-0123-456789abcdef'
  });
  assert.equal(record.branch, 'agent/task');
  assert.equal(record.environment, descriptor().environment);
  assert.equal(record.worktreePath, '/home/user/worktrees/agent-task-01234567');
  assert.match(record.folderUri, /^vscode-remote:/);
  assert.equal(record.released, false);
  assert.ok(calls.some((call) => call.command === 'git' && call.args.includes('check-ref-format')));
  assert.ok(calls.some((call) => call.command === 'git' && call.args.includes('worktree') && call.args.includes('add')));
  assert.ok(calls.some((call) => call.command === 'haco-agent-host' && call.args[0] === 'prepare'));
});

test('wrapped create performs mkdir, Git, and adapter work through the same trusted runner', async () => {
  const runnerConfig = { executable: 'wsl.exe', args: ['-d', 'Hacocoon', '--'] };
  const invocations = [];
  const runner = async (command, args, options) => {
    invocations.push({ command, args, runnerConfig: options.runnerConfig });
    if (command === 'git' && args.includes('rev-parse')) return { stdout: '/repo\n', stderr: '' };
    if (command === 'git' && args.includes('check-ref-format')) return { stdout: '', stderr: '' };
    if (command === 'mkdir') return { stdout: '', stderr: '' };
    if (command === 'git' && args.includes('add')) return { stdout: '', stderr: '' };
    if (command === 'haco-agent-host') return { stdout: JSON.stringify(descriptor()), stderr: '' };
    throw new Error('unexpected command');
  };
  await agent.createAgentWorkspace({
    workspacePath: '/repo', branch: 'agent/a', configuredRoot: '/worktrees', runner, runnerConfig,
    sessionId: 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee'
  });
  assert.ok(invocations.length >= 5);
  for (const invocation of invocations) assert.deepEqual(invocation.runnerConfig, runnerConfig);
});

test('prepare failure rolls back only the linked worktree created by this invocation', async () => {
  const removed = [];
  const runner = async (command, args) => {
    if (command === 'git' && args.includes('rev-parse')) return { stdout: '/repo\n', stderr: '' };
    if (command === 'git' && args.includes('check-ref-format')) return { stdout: '', stderr: '' };
    if (command === 'mkdir') return { stdout: '', stderr: '' };
    if (command === 'git' && args.includes('worktree') && args.includes('add')) return { stdout: '', stderr: '' };
    if (command === 'haco-agent-host') throw new Error('prepare failed');
    if (command === 'git' && args.includes('worktree') && args.includes('remove')) {
      removed.push(args.at(-1));
      return { stdout: '', stderr: '' };
    }
    throw new Error(`unexpected command: ${command}`);
  };
  await assert.rejects(() => agent.createAgentWorkspace({
    workspacePath: '/repo', branch: 'agent/fail', configuredRoot: '/worktrees', runner,
    runnerConfig: { executable: 'wsl.exe', args: ['-d', 'Hacocoon', '--'] },
    sessionId: 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee'
  }), /prepare failed/);
  assert.deepEqual(removed, ['/worktrees/agent-fail-aaaaaaaa']);
  assert.equal(agent.isPathInside('/worktrees', removed[0]), true);
  assert.notEqual(removed[0], '/repo');
});

test('release calls Hacocoon first and preserves a dirty worktree after Environment release', async () => {
  const record = {
    id: 's', sessionId: 's', repoRoot: '/repo', worktreeRoot: '/worktrees',
    worktreePath: '/worktrees/task-s', branch: 'task', environment: 'agent-x', released: false
  };
  const calls = [];
  const runner = async (command, args) => {
    calls.push({ command, args });
    if (command === 'haco-agent-host') return { stdout: '', stderr: '' };
    if (command === 'git' && args.includes('status')) return { stdout: ' M file.go\n', stderr: '' };
    throw new Error('worktree removal must not run while dirty');
  };
  const result = await agent.releaseAgentWorkspace(record, { runner });
  assert.equal(result.removed, false);
  assert.equal(result.reason, 'dirty');
  assert.equal(result.record.released, true);
  assert.deepEqual(calls[0], { command: 'haco-agent-host', args: ['release', '--session', 's'] });
});

test('clean released worktree removal is guarded and never deletes its Git branch', async () => {
  const record = {
    id: 's', sessionId: 's', repoRoot: '/repo', worktreeRoot: '/worktrees',
    worktreePath: '/worktrees/task-s', branch: 'task', environment: 'agent-x', released: true
  };
  const calls = [];
  const runner = async (command, args) => {
    calls.push({ command, args });
    if (command === 'git' && args.includes('status')) return { stdout: '', stderr: '' };
    if (command === 'git' && args.includes('worktree') && args.includes('remove')) return { stdout: '', stderr: '' };
    throw new Error('unexpected command');
  };
  const result = await agent.removeOwnedWorktree(record, { runner });
  assert.equal(result.removed, true);
  const remove = calls.at(-1).args;
  assert.deepEqual(remove, ['-C', '/repo', 'worktree', 'remove', '/worktrees/task-s']);
  assert.equal(calls.some((call) => call.args.includes('-D') || call.args.includes('-d')), false, 'branch must not be deleted');
});
