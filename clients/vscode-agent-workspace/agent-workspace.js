'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path').posix;
const { execFile } = require('node:child_process');
const { promisify } = require('node:util');

const execFileAsync = promisify(execFile);
const stateKey = 'hacocoon.agentWorkspace.records.v1';
const maxOutputBytes = 1024 * 1024;

function slugify(value) {
  const slug = String(value || '')
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, '-')
    .replace(/^[._-]+|[._-]+$/g, '')
    .slice(0, 48);
  return slug || 'task';
}

function normalizeWorktreeRoot(configuredRoot, repoRoot) {
  if (configuredRoot && String(configuredRoot).trim()) {
    const value = String(configuredRoot).trim();
    if (!path.isAbsolute(value)) throw new Error('Hacocoon worktreeRoot must be an absolute Linux path');
    return path.normalize(value);
  }
  return path.join(path.dirname(repoRoot), '.hacocoon-worktrees', path.basename(repoRoot));
}

function deriveWorktreePath(repoRoot, configuredRoot, branch, sessionID) {
  const root = normalizeWorktreeRoot(configuredRoot, repoRoot);
  const suffix = String(sessionID).replace(/[^a-f0-9]/gi, '').slice(0, 8).toLowerCase() || 'session';
  return { root, worktreePath: path.join(root, `${slugify(branch)}-${suffix}`) };
}

function isPathInside(parent, child) {
  const relative = path.relative(path.normalize(parent), path.normalize(child));
  return relative !== '' && relative !== '..' && !relative.startsWith('../') && !path.isAbsolute(relative);
}

function assertOwnedWorktree(record) {
  if (!record || typeof record !== 'object') throw new Error('missing Hacocoon agent-workspace record');
  for (const key of ['repoRoot', 'worktreeRoot', 'worktreePath', 'sessionId']) {
    if (typeof record[key] !== 'string' || !record[key]) throw new Error(`invalid agent-workspace record field: ${key}`);
  }
  if (path.normalize(record.worktreePath) === path.normalize(record.repoRoot)) throw new Error('refusing to remove the main checkout');
  if (!isPathInside(record.worktreeRoot, record.worktreePath)) throw new Error('refusing to remove a worktree outside the Hacocoon-owned root');
  return record;
}

function validateDescriptor(input) {
  let descriptor = input;
  if (typeof input === 'string') {
    try { descriptor = JSON.parse(input); } catch (error) { throw new Error(`invalid haco-agent-host JSON: ${error.message}`); }
  }
  if (!descriptor || typeof descriptor !== 'object') throw new Error('invalid haco-agent-host descriptor');
  for (const key of ['environment', 'ssh_alias', 'remote_workspace', 'folder_uri']) {
    if (typeof descriptor[key] !== 'string' || !descriptor[key]) throw new Error(`haco-agent-host descriptor missing ${key}`);
  }
  if (descriptor.remote_workspace !== '/workspace') throw new Error(`unexpected Hacocoon remote workspace: ${descriptor.remote_workspace}`);
  if (!/^haco-agent-[a-f0-9]+$/.test(descriptor.ssh_alias)) throw new Error('unexpected Hacocoon SSH alias');
  let folder;
  try { folder = new URL(descriptor.folder_uri); } catch (error) { throw new Error(`invalid Hacocoon folder URI: ${error.message}`); }
  if (folder.protocol !== 'vscode-remote:' || folder.host !== `ssh-remote+${descriptor.ssh_alias}` || folder.pathname !== '/workspace') {
    throw new Error('unexpected Hacocoon remote folder URI');
  }
  return descriptor;
}

function buildInvocation(command, args, runnerConfig = {}) {
  const executable = String(runnerConfig.executable || '').trim();
  const prefixArgs = Array.isArray(runnerConfig.args) ? runnerConfig.args.map(String) : [];
  if (!executable) return { executable: command, args: [...args] };
  return { executable, args: [...prefixArgs, command, ...args] };
}

async function run(command, args, options = {}) {
  if (!command || typeof command !== 'string') throw new Error('executable is required');
  const invocation = buildInvocation(command, args, options.runnerConfig);
  const result = await execFileAsync(invocation.executable, invocation.args, {
    cwd: options.runnerConfig && options.runnerConfig.executable ? undefined : options.cwd,
    windowsHide: true,
    maxBuffer: maxOutputBytes,
    encoding: 'utf8',
    env: options.env || process.env
  });
  return { stdout: result.stdout || '', stderr: result.stderr || '' };
}

function withRepo(repoRoot, args) {
  return ['-C', repoRoot, ...args];
}

function gitCheckBranchArgs(repoRoot, branch) {
  return withRepo(repoRoot, ['check-ref-format', '--branch', branch]);
}

function gitWorktreeAddArgs(repoRoot, worktreePath, branch) {
  return withRepo(repoRoot, ['worktree', 'add', '-b', branch, worktreePath, 'HEAD']);
}

function gitWorktreeRemoveArgs(repoRoot, worktreePath, force = false) {
  return withRepo(repoRoot, ['worktree', 'remove', ...(force ? ['--force'] : []), worktreePath]);
}

function adapterPrepareArgs(sessionId, worktreePath) {
  return ['prepare', '--session', sessionId, '--json', '--no-launch', worktreePath];
}

function adapterReleaseArgs(sessionId) {
  return ['release', '--session', sessionId];
}

async function resolveRepoRoot(gitExecutable, workspacePath, runner = run, runnerConfig = {}) {
  if (!path.isAbsolute(workspacePath)) throw new Error('workspace must be an absolute Linux path');
  const result = await runner(gitExecutable, ['-C', workspacePath, 'rev-parse', '--show-toplevel'], { runnerConfig });
  const repoRoot = result.stdout.trim();
  if (!repoRoot || !path.isAbsolute(repoRoot)) throw new Error('Git did not return an absolute repository root');
  return path.normalize(repoRoot);
}

async function ensureBranchName(gitExecutable, repoRoot, branch, runner = run, runnerConfig = {}) {
  const value = String(branch || '').trim();
  if (!value || value.length > 200 || /[\u0000-\u001f\u007f]/.test(value)) throw new Error('invalid Git branch name');
  await runner(gitExecutable, gitCheckBranchArgs(repoRoot, value), { runnerConfig });
  return value;
}

async function ensureWorktreeRoot(root, runner, runnerConfig) {
  if (runnerConfig && runnerConfig.executable) {
    await runner('mkdir', ['-p', root], { runnerConfig });
    return;
  }
  await fs.promises.mkdir(root, { recursive: true });
}

async function createAgentWorkspace({
  workspacePath,
  branch,
  configuredRoot,
  gitExecutable = 'git',
  adapterExecutable = 'haco-agent-host',
  runner = run,
  runnerConfig = {},
  sessionId = crypto.randomUUID()
}) {
  const repoRoot = await resolveRepoRoot(gitExecutable, workspacePath, runner, runnerConfig);
  const safeBranch = await ensureBranchName(gitExecutable, repoRoot, branch, runner, runnerConfig);
  const { root: worktreeRoot, worktreePath } = deriveWorktreePath(repoRoot, configuredRoot, safeBranch, sessionId);
  await ensureWorktreeRoot(worktreeRoot, runner, runnerConfig);

  let createdWorktree = false;
  try {
    await runner(gitExecutable, gitWorktreeAddArgs(repoRoot, worktreePath, safeBranch), { runnerConfig });
    createdWorktree = true;
    const prepared = await runner(adapterExecutable, adapterPrepareArgs(sessionId, worktreePath), { runnerConfig });
    const descriptor = validateDescriptor(prepared.stdout.trim());
    return {
      id: sessionId,
      sessionId,
      repoRoot,
      worktreeRoot,
      worktreePath,
      branch: safeBranch,
      environment: descriptor.environment,
      sshAlias: descriptor.ssh_alias,
      folderUri: descriptor.folder_uri,
      released: false,
      createdAt: new Date().toISOString()
    };
  } catch (error) {
    if (createdWorktree) {
      try { await runner(gitExecutable, gitWorktreeRemoveArgs(repoRoot, worktreePath, true), { runnerConfig }); }
      catch (cleanupError) { error.cleanupError = cleanupError; }
    }
    throw error;
  }
}

async function worktreeDirty(record, gitExecutable = 'git', runner = run, runnerConfig = {}) {
  assertOwnedWorktree(record);
  try {
    const result = await runner(gitExecutable, ['-C', record.worktreePath, 'status', '--porcelain'], { runnerConfig });
    return result.stdout.trim().length > 0;
  } catch (error) {
    if (error && (error.code === 'ENOENT' || /No such file|not a git repository/i.test(error.message || ''))) return false;
    throw error;
  }
}

async function removeOwnedWorktree(record, { gitExecutable = 'git', runner = run, runnerConfig = {} } = {}) {
  assertOwnedWorktree(record);
  if (await worktreeDirty(record, gitExecutable, runner, runnerConfig)) return { record, removed: false, reason: 'dirty' };
  try {
    await runner(gitExecutable, gitWorktreeRemoveArgs(record.repoRoot, record.worktreePath), { runnerConfig });
    return { record, removed: true, reason: 'clean' };
  } catch (error) {
    if (error && (error.code === 'ENOENT' || /not a working tree|No such file/i.test(error.message || ''))) return { record, removed: true, reason: 'missing' };
    throw error;
  }
}

async function releaseAgentWorkspace(record, {
  gitExecutable = 'git', adapterExecutable = 'haco-agent-host', runner = run, runnerConfig = {}
} = {}) {
  assertOwnedWorktree(record);
  await runner(adapterExecutable, adapterReleaseArgs(record.sessionId), { runnerConfig });
  const releasedRecord = { ...record, released: true };
  try {
    if (await worktreeDirty(releasedRecord, gitExecutable, runner, runnerConfig)) return { record: releasedRecord, removed: false, reason: 'dirty' };
    return await removeOwnedWorktree(releasedRecord, { gitExecutable, runner, runnerConfig });
  } catch (error) {
    error.releasedRecord = releasedRecord;
    throw error;
  }
}

function normalizeRecords(value) {
  if (!Array.isArray(value)) return [];
  return value.filter((record) => {
    try { assertOwnedWorktree(record); return true; } catch (_) { return false; }
  });
}

module.exports = {
  stateKey,
  slugify,
  normalizeWorktreeRoot,
  deriveWorktreePath,
  isPathInside,
  assertOwnedWorktree,
  validateDescriptor,
  buildInvocation,
  gitCheckBranchArgs,
  gitWorktreeAddArgs,
  gitWorktreeRemoveArgs,
  adapterPrepareArgs,
  adapterReleaseArgs,
  resolveRepoRoot,
  ensureBranchName,
  createAgentWorkspace,
  releaseAgentWorkspace,
  removeOwnedWorktree,
  normalizeRecords,
  run
};
