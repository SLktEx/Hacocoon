'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');

const tools = require('../../clients/vscode-agent-workspace/tool-contract');

test('tool errors redact --session arguments and UUID session identities', () => {
  const raw = 'Command failed: haco-agent-host prepare --session 01234567-89ab-4def-8123-456789abcdef --json /worktree';
  const safe = tools.modelSafeError(new Error(raw));
  assert.match(safe.message, /--session <redacted>/);
  assert.doesNotMatch(safe.message, /01234567-89ab-4def-8123-456789abcdef/);
});

test('tool result projection never serializes internal ownership fields', () => {
  const record = {
    id: 'opaque-internal-id',
    sessionId: 'opaque-internal-id',
    branch: 'agent/task',
    environment: 'agent-abc',
    folderUri: 'vscode-remote://ssh-remote+haco-agent-abc/workspace',
    sshAlias: 'haco-agent-abc',
    repoRoot: '/repo',
    worktreeRoot: '/worktrees',
    worktreePath: '/worktrees/task',
    released: false
  };
  const serialized = tools.toolResultEnvelope('create', record, { opened: false });
  assert.doesNotMatch(serialized, /opaque-internal-id|sessionId|sshAlias|repoRoot|worktreeRoot|worktreePath/);
});
