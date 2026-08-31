'use strict';

const os = require('node:os');
const vscode = require('vscode');
const agent = require('./agent-workspace');

function getConfig() {
  const config = vscode.workspace.getConfiguration('hacocoon.agentWorkspace');
  return {
    gitExecutable: config.get('gitExecutable', 'git'),
    adapterExecutable: config.get('adapterExecutable', 'haco-agent-host'),
    worktreeRoot: config.get('worktreeRoot', ''),
    wslDistro: config.get('wslDistro', 'Hacocoon'),
    repositoryPath: config.get('repositoryPath', '')
  };
}

function wslDistroFromURI(uri) {
  if (!uri || uri.scheme !== 'vscode-remote') return '';
  const authority = String(uri.authority || '');
  if (!authority.startsWith('wsl+')) return '';
  return authority.slice(4);
}

function validateDistroName(value) {
  const distro = String(value || '').trim();
  if (!distro || distro.length > 128 || /[\u0000-\u001f\u007f]/.test(distro)) {
    throw new Error('invalid WSL distribution name');
  }
  return distro;
}

async function resolveExecutionContext(config) {
  const folders = vscode.workspace.workspaceFolders || [];
  const folder = folders[0];

  if (process.platform === 'win32') {
    const distro = validateDistroName(config.wslDistro || 'Hacocoon');
    const remoteDistro = folder ? wslDistroFromURI(folder.uri) : '';
    let workspacePath = '';

    if (folder && remoteDistro && remoteDistro.toLowerCase() === distro.toLowerCase()) {
      workspacePath = folder.uri.path;
    } else if (String(config.repositoryPath || '').trim()) {
      workspacePath = String(config.repositoryPath).trim();
    } else {
      workspacePath = await vscode.window.showInputBox({
        title: 'Hacocoon repository path',
        prompt: `Absolute Linux repository path inside WSL distro ${distro}`,
        placeHolder: '/home/user/repos/project',
        ignoreFocusOut: true
      }) || '';
    }

    if (!workspacePath || !workspacePath.startsWith('/')) {
      throw new Error(`an absolute Linux repository path inside WSL distro ${distro} is required`);
    }
    return {
      workspacePath,
      runnerConfig: { executable: 'wsl.exe', args: ['-d', distro, '--'] },
      execution: { kind: 'wsl', distro }
    };
  }

  if (!folder || folder.uri.scheme !== 'file') {
    throw new Error('open a local Git workspace, or use Windows with the configured Hacocoon WSL distro');
  }
  return {
    workspacePath: folder.uri.fsPath,
    runnerConfig: {},
    execution: { kind: 'local', platform: os.platform() }
  };
}

function runnerConfigForRecord(record) {
  const execution = record && record.execution;
  if (execution && execution.kind === 'wsl') {
    const distro = validateDistroName(execution.distro);
    return { executable: 'wsl.exe', args: ['-d', distro, '--'] };
  }
  return {};
}

async function newAgentWorkspace(context, output) {
  const config = getConfig();
  let executionContext;
  try {
    executionContext = await resolveExecutionContext(config);
  } catch (error) {
    void vscode.window.showErrorMessage(`Hacocoon: ${error.message}`);
    return;
  }

  const branch = await vscode.window.showInputBox({
    title: 'Hacocoon: New Agent Workspace',
    prompt: 'New Git branch for this agent',
    placeHolder: 'agent/task-name',
    ignoreFocusOut: true
  });
  if (!branch) return;

  await vscode.window.withProgress({
    location: vscode.ProgressLocation.Notification,
    title: 'Creating Hacocoon agent workspace',
    cancellable: false
  }, async () => {
    try {
      const record = await agent.createAgentWorkspace({
        workspacePath: executionContext.workspacePath,
        branch,
        configuredRoot: config.worktreeRoot,
        gitExecutable: config.gitExecutable,
        adapterExecutable: config.adapterExecutable,
        runnerConfig: executionContext.runnerConfig
      });
      record.execution = executionContext.execution;

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
  const location = record.execution && record.execution.kind === 'wsl'
    ? `${record.worktreePath} · WSL ${record.execution.distro}`
    : record.worktreePath;
  return { label: record.branch, description: state, detail: location, record };
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
    const runnerConfig = runnerConfigForRecord(selected.record);
    try {
      if (selected.record.released) {
        result = await agent.removeOwnedWorktree(selected.record, {
          gitExecutable: config.gitExecutable,
          runnerConfig
        });
      } else {
        result = await agent.releaseAgentWorkspace(selected.record, {
          gitExecutable: config.gitExecutable,
          adapterExecutable: config.adapterExecutable,
          runnerConfig
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

module.exports = {
  activate,
  deactivate,
  wslDistroFromURI,
  validateDistroName,
  resolveExecutionContext,
  runnerConfigForRecord,
  recordLabel
};
