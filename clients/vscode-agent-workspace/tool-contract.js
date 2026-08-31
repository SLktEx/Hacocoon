'use strict';

function normalizeToolBranch(value) {
  const branch = String(value || '').trim();
  if (!branch || branch.length > 200 || /[\u0000-\u001f\u007f]/.test(branch)) {
    throw new Error('branch is required and must be a valid Git branch name');
  }
  return branch;
}

function normalizeOpenFlag(value) {
  if (value === undefined) return false;
  if (typeof value !== 'boolean') throw new Error('open must be a boolean');
  return value;
}

function safeWorkspaceRecord(record) {
  if (!record || typeof record !== 'object') throw new Error('invalid Hacocoon agent workspace record');
  const safe = {
    branch: String(record.branch || ''),
    environment: String(record.environment || ''),
    folderUri: String(record.folderUri || ''),
    released: Boolean(record.released)
  };
  if (!safe.branch || !safe.environment || !safe.folderUri) {
    throw new Error('incomplete Hacocoon agent workspace record');
  }
  if (record.execution && record.execution.kind === 'wsl') {
    safe.execution = { kind: 'wsl', distro: String(record.execution.distro || '') };
  } else if (record.execution && record.execution.kind) {
    safe.execution = { kind: String(record.execution.kind) };
  }
  return safe;
}

function safeWorkspaceRecords(records) {
  if (!Array.isArray(records)) return [];
  const output = [];
  for (const record of records) {
    try {
      output.push(safeWorkspaceRecord(record));
    } catch (_) {
      // Invalid persisted records are not exposed to the model.
    }
  }
  return output;
}

function selectOwnedRecordByBranch(records, branch) {
  const wanted = normalizeToolBranch(branch);
  const matches = (Array.isArray(records) ? records : []).filter((record) => record && record.branch === wanted);
  if (matches.length === 0) throw new Error(`no VS Code-owned Hacocoon agent workspace found for branch ${wanted}`);
  if (matches.length !== 1) throw new Error(`multiple Hacocoon agent workspaces match branch ${wanted}; release is ambiguous`);
  return matches[0];
}

function redactToolErrorMessage(value) {
  let message = String(value || 'Hacocoon agent workspace operation failed');
  message = message.replace(/--session(?:=|\s+)[^\s]+/gi, '--session <redacted>');
  message = message.replace(/\b[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b/gi, '<redacted-session>');
  return message;
}

function modelSafeError(error) {
  const safe = new Error(redactToolErrorMessage(error && error.message));
  safe.name = error && error.name ? String(error.name) : 'Error';
  return safe;
}

function toolResultEnvelope(action, record, extra = {}) {
  const result = {
    action,
    workspace: safeWorkspaceRecord(record),
    ...extra
  };
  return JSON.stringify(result);
}

function listToolResult(records) {
  return JSON.stringify({ action: 'list', workspaces: safeWorkspaceRecords(records) });
}

module.exports = {
  normalizeToolBranch,
  normalizeOpenFlag,
  safeWorkspaceRecord,
  safeWorkspaceRecords,
  selectOwnedRecordByBranch,
  redactToolErrorMessage,
  modelSafeError,
  toolResultEnvelope,
  listToolResult
};
