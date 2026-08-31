'use strict';

const os = require('node:os');
const vscode = require('vscode');
const agent = require('./agent-workspace');
const tools = require('./tool-contract');

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

function storedRecords(context) {
  return agent.normalizeRecords(context.globalState.get(agent.stateKey, []));
}

async function storeRecords(context, records) {
  await context.globalState.update(agent.stateKey, records);
}

async function provisionAgentWorkspace(context, output, branch, { openFolder = false, notify = false } = {}) {
  const config = getConfig();
  const safeBranch = tools.normalizeToolBranch(branch);
  const executionContext = await resolveExecutionContext(config);
  const record = await agent.createAgentWorkspace({
    workspacePath: executionContext.workspacePath,
    branch: safeBranch,
    configuredRoot: config.worktreeRoot,
    gitExecutable: config.gitExecutable,
    adapterExecutable: config.adapterExecutable,
    runnerConfig: executionContext.runnerConfig
  });
  record.execution = executionContext.execution;

  const records = storedRecords(context);
  records.push(record);
  await storeRecords(context, records);
  output.appendLine(`created ${record.environment} for ${record.branch} at ${record.worktreePath}`);

  if (openFolder) {
    const folderUri = vscode.Uri.parse(record.folderUri, true);
    await vscode.commands.executeCommand('vscode.openFolder', folderUri, { forceNewWindow: true });
  }
  if (notify) {
    void vscode.window.showInformationMessage(
      `Hacocoon agent workspace ready: ${record.branch}. Start Copilot, Claude, Codex, or another Agent Host from the Agents window.`
    );
  }
  return record;
}

async function releaseWorkspaceRecord(context, output, record, { notify = false } = {}) {
  const records = storedRecords(context);
  const config = getConfig();
  const runnerConfig = runnerConfigForRecord(record);
  let result;
  try {
    if (record.released) {
      result = await agent.removeOwnedWorktree(record, {
        gitExecutable: config.gitExecutable,
        runnerConfig
      });
    } else {
      result = await agent.releaseAgentWorkspace(record, {
        gitExecutable: config.gitExecutable,
        adapterExecutable: config.adapterExecutable,
        runnerConfig
      });
    }
  } catch (error) {
    const next = error.releasedRecord || record;
    const updated = records.map((item) => item.id === next.id ? next : item);
    await storeRecords(context, updated);
    output.appendLine(`release failed: ${error.message}`);
    throw error;
  }

  if (result.reason === 'dirty') {
    const updated = records.map((item) => item.id === result.record.id ? result.record : item);
    await storeRecords(context, updated);
    output.appendLine(`released ${result.record.environment}; kept dirty worktree ${result.record.worktreePath}`);
    if (notify) {
      void vscode.window.showWarningMessage(
        `Hacocoon Environment released, but the worktree was kept because it has uncommitted changes: ${result.record.worktreePath}`
      );
    }
    return result;
  }

  const remaining = records.filter((item) => item.id !== result.record.id);
  await storeRecords(context, remaining);
  output.appendLine(`released ${result.record.environment}; removed worktree ${result.record.worktreePath}`);
  if (notify) {
    void vscode.window.showInformationMessage(`Hacocoon agent workspace released: ${result.record.branch}`);
  }
  return result;
}

async function newAgentWorkspace(context, output) {
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
      await provisionAgentWorkspace(context, output, branch, { openFolder: true, notify: true });
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
  const records = storedRecords(context);
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

  await vscode.window.withProgress({
    location: vscode.ProgressLocation.Notification,
    title: `Releasing Hacocoon agent workspace ${selected.record.branch}`,
    cancellable: false
  }, async () => {
    try {
      await releaseWorkspaceRecord(context, output, selected.record, { notify: true });
    } catch (error) {
      void vscode.window.showErrorMessage(`Hacocoon agent workspace release failed: ${error.message}`);
    }
  });
}

function textToolResult(text) {
  return new vscode.LanguageModelToolResult([new vscode.LanguageModelTextPart(text)]);
}

function createWorkspaceTool(context, output) {
  return {
    prepareInvocation(options) {
      const branch = tools.normalizeToolBranch(options && options.input && options.input.branch);
      return {
        invocationMessage: `Creating isolated Hacocoon workspace for ${branch}`,
        confirmationMessages: {
          title: 'Create Hacocoon agent workspace?',
          message: `Create a linked Git worktree and isolated Hacocoon Environment for ${branch}. Provider credentials are not copied into the Environment.`
        }
      };
    },
    async invoke(options) {
      const input = options && options.input ? options.input : {};
      const branch = tools.normalizeToolBranch(input.branch);
      const openFolder = tools.normalizeOpenFlag(input.open);
      const record = await provisionAgentWorkspace(context, output, branch, { openFolder, notify: false });
      return textToolResult(tools.toolResultEnvelope('create', record, {
        opened: openFolder,
        nativeAgentSessionStarted: false
      }));
    }
  };
}

function listWorkspacesTool(context) {
  return {
    async invoke() {
      return textToolResult(tools.listToolResult(storedRecords(context)));
    }
  };
}

function releaseWorkspaceTool(context, output) {
  return {
    prepareInvocation(options) {
      const branch = tools.normalizeToolBranch(options && options.input && options.input.branch);
      return {
        invocationMessage: `Releasing Hacocoon workspace for ${branch}`,
        confirmationMessages: {
          title: 'Release Hacocoon agent workspace?',
          message: `Release the Hacocoon Environment for ${branch}. A dirty worktree is preserved and the Git branch is never deleted automatically.`
        }
      };
    },
    async invoke(options) {
      const branch = tools.normalizeToolBranch(options && options.input && options.input.branch);
      const record = tools.selectOwnedRecordByBranch(storedRecords(context), branch);
      const result = await releaseWorkspaceRecord(context, output, record, { notify: false });
      return textToolResult(tools.toolResultEnvelope('release', result.record, {
        worktreeRemoved: Boolean(result.removed),
        reason: result.reason
      }));
    }
  };
}

function registerLanguageModelTools(context, output) {
  if (!vscode.lm || typeof vscode.lm.registerTool !== 'function') {
    output.appendLine('VS Code Language Model Tool API is unavailable; Hacocoon agent tools were not registered.');
    return;
  }
  context.subscriptions.push(vscode.lm.registerTool('hacocoon_createAgentWorkspace', createWorkspaceTool(context, output)));
  context.subscriptions.push(vscode.lm.registerTool('hacocoon_listAgentWorkspaces', listWorkspacesTool(context)));
  context.subscriptions.push(vscode.lm.registerTool('hacocoon_releaseAgentWorkspace', releaseWorkspaceTool(context, output)));
}

function activate(context) {
  const output = vscode.window.createOutputChannel('Hacocoon Agent Workspaces');
  context.subscriptions.push(output);
  context.subscriptions.push(vscode.commands.registerCommand('hacocoon.newAgentWorkspace', () => newAgentWorkspace(context, output)));
  context.subscriptions.push(vscode.commands.registerCommand('hacocoon.releaseAgentWorkspace', () => releaseAgentWorkspace(context, output)));
  registerLanguageModelTools(context, output);
}

function deactivate() {}

module.exports = {
  activate,
  deactivate,
  wslDistroFromURI,
  validateDistroName,
  resolveExecutionContext,
  runnerConfigForRecord,
  storedRecords,
  provisionAgentWorkspace,
  releaseWorkspaceRecord,
  recordLabel,
  createWorkspaceTool,
  listWorkspacesTool,
  releaseWorkspaceTool,
  registerLanguageModelTools
};
