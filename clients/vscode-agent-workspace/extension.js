'use strict';

const vscode = require('vscode');
const agent = require('./agent-workspace');

function getConfig() {
  const config = vscode.workspace.getConfiguration('hacocoon.agentWorkspace');
  return {
    gitExecutable: config.get('gitExecutable', 'git'),
    adapterExecutable: config.get('adapterExecutable', 'haco-agent-host'),
    worktreeRoot: config.get('worktreeRoot', '')
  };
}

function currentWorkspacePath() {
  const folders = vscode.workspace.workspaceFolders || [];
  if (folders.length === 0) return '';
  return folders[0].uri.fsPath;
}

async function newAgentWorkspace(context, output) {
  const cwd = currentWorkspacePath();
  if (!cwd) {
    void vscode.window.showErrorMessage('Hacocoon: open a Git workspace before creating an agent workspace.');
    return;
  }

  const branch = await vscode.window.showInputBox({
    title: 'Hacocoon: New Agent Workspace',
    prompt: 'New Git branch for this agent',
    placeHolder: 'agent/task-name',
    ignoreFocusOut: true
  });
  if (!branch) return;

  const config = getConfig();
  await vscode.window.withProgress({
    location: vscode.ProgressLocation.Notification,
    title: 'Creating Hacocoon agent workspace',
    cancellable: false
  }, async () => {
    try {
      const record = await agent.createAgentWorkspace({
        cwd,
        branch,
        configuredRoot: config.worktreeRoot,
        gitExecutable: config.gitExecutable,
        adapterExecutable: config.adapterExecutable
      });
      const records = agent.normalizeRecords(context.globalState.get(agent.stateKey, []));
      records.push(record);
      await context.globalState.update(agent.stateKey, records);
      output.appendLine(`created ${record.environment} for ${record.branch} at ${record.worktreePath}`);

      const folderUri = vscode.Uri.parse(record.folderUri, true);
      await vscode.commands.executeCommand('vscode.openFolder', folderUri, { forceNewWindow: true });
      void vscode.window.showInformationMessage(
        `Hacocoon agent workspace ready: ${record.branch}. Start Copilot, Claude, or Codex from the Agents window.`
      );
    } catch (error) {
      output.appendLine(`create failed: ${error.message}`);
      if (error.cleanupError) output.appendLine(`cleanup failed: ${error.cleanupError.message}`);
      void vscode.window.showErrorMessage(`Hacocoon agent workspace creation failed: ${error.message}`);
    }
  });
}

function recordLabel(record) {
  const state = record.released ? 'released' : record.environment;
  return {
    label: record.branch,
    description: state,
    detail: record.worktreePath,
    record
  };
}

async function releaseAgentWorkspace(context, output) {
  const records = agent.normalizeRecords(context.globalState.get(agent.stateKey, []));
  if (records.length === 0) {
    void vscode.window.showInformationMessage('Hacocoon: no VS Code-created agent workspaces are recorded.');
    return;
  }
  const selected = await vscode.window.showQuickPick(records.map(recordLabel), {
    title: 'Hacocoon: Release Agent Workspace',
    placeHolder: 'Choose the workspace to release',
    matchOnDescription: true,
    matchOnDetail: true,
    ignoreFocusOut: true
  });
  if (!selected) return;

  const config = getConfig();
  await vscode.window.withProgress({
    location: vscode.ProgressLocation.Notification,
    title: `Releasing Hacocoon agent workspace ${selected.record.branch}`,
    cancellable: false
  }, async () => {
    let result;
    try {
      if (selected.record.released) {
        result = await agent.removeOwnedWorktree(selected.record, {
          gitExecutable: config.gitExecutable
        });
      } else {
        result = await agent.releaseAgentWorkspace(selected.record, {
          gitExecutable: config.gitExecutable,
          adapterExecutable: config.adapterExecutable
        });
      }
    } catch (error) {
      const next = error.releasedRecord || selected.record;
      const updated = records.map((record) => record.id === next.id ? next : record);
      await context.globalState.update(agent.stateKey, updated);
      output.appendLine(`release failed: ${error.message}`);
      void vscode.window.showErrorMessage(`Hacocoon agent workspace release failed: ${error.message}`);
      return;
    }

    if (result.reason === 'dirty') {
      const updated = records.map((record) => record.id === result.record.id ? result.record : record);
      await context.globalState.update(agent.stateKey, updated);
      output.appendLine(`released ${result.record.environment}; kept dirty worktree ${result.record.worktreePath}`);
      void vscode.window.showWarningMessage(
        `Hacocoon Environment released, but the worktree was kept because it has uncommitted changes: ${result.record.worktreePath}`
      );
      return;
    }

    const remaining = records.filter((record) => record.id !== result.record.id);
    await context.globalState.update(agent.stateKey, remaining);
    output.appendLine(`released ${result.record.environment}; removed worktree ${result.record.worktreePath}`);
    void vscode.window.showInformationMessage(`Hacocoon agent workspace released: ${result.record.branch}`);
  });
}

function activate(context) {
  const output = vscode.window.createOutputChannel('Hacocoon Agent Workspaces');
  context.subscriptions.push(output);
  context.subscriptions.push(vscode.commands.registerCommand('hacocoon.newAgentWorkspace', () => newAgentWorkspace(context, output)));
  context.subscriptions.push(vscode.commands.registerCommand('hacocoon.releaseAgentWorkspace', () => releaseAgentWorkspace(context, output)));
}

function deactivate() {}

module.exports = { activate, deactivate, currentWorkspacePath, recordLabel };
