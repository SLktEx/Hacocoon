'use strict';

const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
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
    const expanded = String(configuredRoot).startsWith('~/')
      ? path.join(os.homedir(), String(configuredRoot).slice(2))
      : String(configuredRoot);
    return path.resolve(expanded);
  }
  return path.resolve(path.dirname(repoRoot), '.hacocoon-worktrees', path.basename(repoRoot));
}

function deriveWorktreePath(repoRoot, configuredRoot, branch, sessionID) {
  const root = normalizeWorktreeRoot(configuredRoot, repoRoot);
  const suffix = String(sessionID).replace(/[^a-f0-9]/gi, '').slice(0, 8).toLowerCase() || 'session';
  return {
    root,
    worktreePath: path.join(root, `${slugify(branch)}-${suffix}`)
  };
}

function isPathInside(parent, child) {
  const relative = path.relative(path.resolve(parent), path.resolve(child));
  return relative !== '' && !relative.startsWith('..' + path.sep) && relative !== '..' && !path.isAbsolute(relative);
}

function assertOwnedWorktree(record) {
  if (!record || typeof record !== 'object') throw new Error('missing Hacocoon agent-workspace record');
  for (const key of ['repoRoot', 'worktreeRoot', 'worktreePath', 'sessionId']) {
    if (typeof record[key] !== 'string' || !record[key]) throw new Error(`invalid agent-workspace record field: ${key}`);
  }
  if (path.resolve(record.worktreePath) === path.resolve(record.repoRoot)) {
    throw new Error('refusing to remove the main checkout');
  }
  if (!isPathInside(record.worktreeRoot, record.worktreePath)) {
    throw new Error('refusing to remove a worktree outside the Hacocoon-owned root');
  }
  return record;
}

function validateDescriptor(input) {
  let descriptor = input;
  if (typeof input === 'string') {
    try {
      descriptor = JSON.parse(input);
    } catch (error) {
      throw new Error(`invalid haco-agent-host JSON: ${error.message}`);
    }
  }
  if (!descriptor || typeof descriptor !== 'object') throw new Error('invalid haco-agent-host descriptor');
  for (const key of ['environment', 'ssh_alias', 'remote_workspace', 'folder_uri']) {
    if (typeof descriptor[key] !== 'string' || !descriptor[key]) {
      throw new Error(`haco-agent-host descriptor missing ${key}`);
    }
  }
  if (descriptor.remote_workspace !== '/workspace') {
    throw new Error(`unexpected Hacocoon remote workspace: ${descriptor.remote_workspace}`);
  }
  if (!/^haco-agent-[a-f0-9]+$/.test(descriptor.ssh_alias)) {
    throw new Error('unexpected Hacocoon SSH alias');
  }
  let folder;
  try {
    folder = new URL(descriptor.folder_uri);
  } catch (error) {
    throw new Error(`invalid Hacocoon folder URI: ${error.message}`);
  }
  if (folder.protocol !== 'vscode-remote:' || folder.host !== `ssh-remote+${descriptor.ssh_alias}` || folder.pathname !== '/workspace') {
    throw new Error('unexpected Hacocoon remote folder URI');
  }
  return descriptor;
}

function gitCheckBranchArgs(branch) {
  return ['check-ref-format', '--branch', branch];
}

function gitWorktreeAddArgs(worktreePath, branch) {
  return ['worktree', 'add', '-b', branch, worktreePath, 'HEAD'];
}

function gitWorktreeRemoveArgs(worktreePath) {
  return ['worktree', 'remove', worktreePath];
}

function adapterPrepareArgs(sessionId, worktreePath) {
  return ['prepare', '--session', sessionId, '--json', '--no-launch', worktreePath];
}

function adapterReleaseArgs(sessionId) {
  return ['release', '--session', sessionId];
}

async function run(command, args, options = {}) {
  if (!command || typeof command !== 'string') throw new Error('executable is required');
  const result = await execFileAsync(command, args, {
    cwd: options.cwd,
    windowsHide: true,
    maxBuffer: maxOutputBytes,
    encoding: 'utf8',
    env: options.env || process.env
  });
  return { stdout: result.stdout || '', stderr: result.stderr || '' };
}

async function resolveRepoRoot(gitExecutable, cwd, runner = run) {
  const result = await runner(gitExecutable, ['rev-parse', '--show-toplevel'], { cwd });
  const repoRoot = result.stdout.trim();
  if (!repoRoot) throw new Error('Git did not return a repository root');
  return path.resolve(repoRoot);
}

async function ensureBranchName(gitExecutable, repoRoot, branch, runner = run) {
  const value = String(branch || '').trim();
  if (!value || value.length > 200 || /[\u0000-\u001f\u007f]/.test(value)) {
    throw new Error('invalid Git branch name');
  }
  await runner(gitExecutable, gitCheckBranchArgs(value), { cwd: repoRoot });
  return value;
}

async function ensureWorktreeAbsent(worktreePath) {
  try {
    await fs.promises.access(worktreePath);
    throw new Error(`worktree path already exists: ${worktreePath}`);
  } catch (error) {
    if (error && error.code === 'ENOENT') return;
    throw error;
  }
}

async function createAgentWorkspace({
  cwd,
  branch,
  configuredRoot,
  gitExecutable = 'git',
  adapterExecutable = 'haco-agent-host',
  runner = run,
  sessionId = crypto.randomUUID()
}) {
  const repoRoot = await resolveRepoRoot(gitExecutable, cwd, runner);
  const safeBranch = await ensureBranchName(gitExecutable, repoRoot, branch, runner);
  const { root: worktreeRoot, worktreePath } = deriveWorktreePath(repoRoot, configuredRoot, safeBranch, sessionId);
  await ensureWorktreeAbsent(worktreePath);
  await fs.promises.mkdir(worktreeRoot, { recursive: true });

  let createdWorktree = false;
  try {
    await runner(gitExecutable, gitWorktreeAddArgs(worktreePath, safeBranch), { cwd: repoRoot });
    createdWorktree = true;
    const prepared = await runner(adapterExecutable, adapterPrepareArgs(sessionId, worktreePath), { cwd: repoRoot });
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
      try {
        await runner(gitExecutable, ['worktree', 'remove', '--force', worktreePath], { cwd: repoRoot });
      } catch (cleanupError) {
        error.cleanupError = cleanupError;
      }
    }
    throw error;
  }
}

async function worktreeDirty(record, gitExecutable = 'git', runner = run) {
  assertOwnedWorktree(record);
  try {
    const result = await runner(gitExecutable, ['status', '--porcelain'], { cwd: record.worktreePath });
    return result.stdout.trim().length > 0;
  } catch (error) {
    if (error && (error.code === 'ENOENT' || /No such file/i.test(error.message || ''))) return false;
    throw error;
  }
}

async function removeOwnedWorktree(record, {
  gitExecutable = 'git',
  runner = run
} = {}) {
  assertOwnedWorktree(record);
  const dirty = await worktreeDirty(record, gitExecutable, runner);
  if (dirty) return { record, removed: false, reason: 'dirty' };
  try {
    await runner(gitExecutable, gitWorktreeRemoveArgs(record.worktreePath), { cwd: record.repoRoot });
    return { record, removed: true, reason: 'clean' };
  } catch (error) {
    if (error && (error.code === 'ENOENT' || /not a working tree|No such file/i.test(error.message || ''))) {
      return { record, removed: true, reason: 'missing' };
    }
    throw error;
  }
}

async function releaseAgentWorkspace(record, {
  gitExecutable = 'git',
  adapterExecutable = 'haco-agent-host',
  runner = run
} = {}) {
  assertOwnedWorktree(record);
  await runner(adapterExecutable, adapterReleaseArgs(record.sessionId), { cwd: record.repoRoot });

  const releasedRecord = { ...record, released: true };
  let dirty = false;
  try {
    dirty = await worktreeDirty(record, gitExecutable, runner);
  } catch (error) {
    error.releasedRecord = releasedRecord;
    throw error;
  }
  if (dirty) {
    return { record: releasedRecord, removed: false, reason: 'dirty' };
  }

  try {
    return await removeOwnedWorktree(releasedRecord, { gitExecutable, runner });
  } catch (error) {
    error.releasedRecord = releasedRecord;
    throw error;
  }
}

function normalizeRecords(value) {
  if (!Array.isArray(value)) return [];
  return value.filter((record) => {
    try {
      assertOwnedWorktree(record);
      return true;
    } catch (_) {
      return false;
    }
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
