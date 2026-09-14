"use strict";

// This function runs only inside the isolated webview. Controller text is always
// assigned through textContent, never HTML, command URIs or executable strings.
function browserMain(ja) {
  const api = acquireVsCodeApi();
  const el = id => document.getElementById(id);
  const text = (en, jp) => ja ? jp : en;
  const printable = value => String(value ?? '').replace(/[\x00-\x08\x0b-\x1f\x7f-\x9f\u202a-\u202e\u2066-\u2069]/g,
    c => `\\u${c.charCodeAt(0).toString(16).padStart(4, '0')}`);
  let selected, busy = false, closed = false;
  const words = {
    heading: text('Review an operation', '操作を確認する'),
    intro: text('Inspect the complete target and saved scope, then answer below. Closing this view sends no answer.',
      '対象と保存範囲をすべて確認してから回答してください。画面を閉じても回答は送信されません。'),
    refresh: text('Refresh pending requests', '承認待ちを更新'),
    pendingHeading: text('Pending requests', '承認待ちの操作'),
    currentHeading: text('This operation', '今回の操作'),
    futureLabel: text('Policy for later matching requests', '今後の一致する操作への設定'),
    scopeHeading: text('Exact scope to save', '保存する正確な範囲'),
    deny: text('Deny this operation', '今回は拒否する'), allow: text('Allow this operation', '今回は許可する')
  };
  for (const [id, value] of Object.entries(words)) el(id).textContent = value;
  const optionLabel = choice => {
    if (!choice) return text('Only this operation — do not save', '今回のみ・設定を保存しない');
    const [decision, level] = choice.split('-');
    const action = { allow: text('Allow', '許可'), deny: text('Deny', '拒否'), ask: text('Ask each time', '毎回確認') }[decision];
    return action + ' / ' + (level === 'global' ? text('all Environments, including future ones', '今後作成するものを含むすべてのEnv') : text('this Env creation only', '今回作成されたEnvのみ'));
  };
  function fields(target, object) {
    target.replaceChildren();
    for (const [key, value] of Object.entries(object ?? {})) {
      const row = document.createElement('div'); row.className = 'field';
      const label = document.createElement('strong');
      const names = {
        capability: text('Capability', '機能'), action: text('Action', '操作'), resource: text('Target', '対象'),
        environment: 'Env', environment_instance: text('Env creation identity', 'Envの作成識別子'),
        attributes: text('Exact constraints', '正確な制約'), decision: text('Saved decision', '保存する判断'),
        reason: text('Reason', '理由'), expires_at: text('Expiry', '有効期限')
      };
      label.textContent = names[key] ?? printable(key);
      const content = document.createElement('pre');
      content.textContent = typeof value === 'object' ? printable(JSON.stringify(value, null, 2)) : printable(value);
      row.append(label, content); target.append(row);
    }
  }
  function controls() {
    const choice = selected?.options.find(o => o.choice === el('save').value);
    el('allow').disabled = closed || busy || !selected || choice?.scope.decision === 'deny';
    el('deny').disabled = closed || busy || !selected || choice?.scope.decision === 'allow';
    el('save').disabled = closed || busy || !selected;
    el('refresh').disabled = closed || busy;
    for (const button of el('pending').querySelectorAll('button')) button.disabled = closed || busy;
  }
  function scope() {
    const option = selected?.options.find(o => o.choice === el('save').value);
    fields(el('scope'), option?.scope);
    el('scopeNote').textContent = option ? optionLabel(option.choice) : text('No Policy will be saved.', '設定は保存されません。');
    controls();
  }
  function clear() { selected = undefined; el('detail').hidden = true; controls(); }
  function submit(approved) {
    if (busy || closed || !selected) return;
    const message = { type: 'decide', request_id: selected.request.request_id, token: selected.token, approved, save: el('save').value };
    busy = true; controls(); api.postMessage(message);
  }
  el('refresh').addEventListener('click', () => { if (!busy && !closed) api.postMessage({ type: 'list' }); });
  el('save').addEventListener('change', scope);
  el('allow').addEventListener('click', () => { if (!el('allow').disabled) submit(true); });
  el('deny').addEventListener('click', () => { if (!el('deny').disabled) submit(false); });
  window.addEventListener('message', event => {
    const m = event.data;
    if (!m || closed) return;
    if (m.type === 'busy') { busy = true; controls(); return; }
    busy = false;
    if (m.type === 'pending') {
      el('source').textContent = text('Local controller: ', 'ローカルの管理先: ') + printable(m.distribution);
      el('pending').replaceChildren();
      for (const item of m.pending) {
        const button = document.createElement('button'); button.className = 'request';
        button.textContent = [item.environment, item.capability, item.action, item.resource].map(printable).filter(Boolean).join(' · ');
        button.addEventListener('click', () => { if (!busy && !closed) api.postMessage({ type: 'select', request_id: item.request_id }); });
        el('pending').append(button);
      }
      if (!m.pending.length) el('pending').textContent = text('No pending requests.', '承認待ちはありません。');
      if (selected && m.selected !== selected.request.request_id) {
        clear(); el('status').textContent = text('This request is no longer pending. Refresh to inspect current requests.', 'この操作は承認待ちではなくなりました。更新して現在の操作を確認してください。');
      }
    } else if (m.type === 'selected') {
      selected = m.view; el('detail').hidden = false;
      fields(el('current'), { ...selected.request.request, reason: selected.request.reason ?? '' });
      el('save').replaceChildren();
      for (const choice of ['', ...selected.options.map(o => o.choice)]) {
        const option = document.createElement('option'); option.value = choice; option.textContent = optionLabel(choice); el('save').append(option);
      }
      el('save').value = ''; scope();
      el('status').textContent = text('Choose whether to save a Policy, then allow or deny this operation.', '設定を保存するか選び、今回の操作を許可または拒否してください。');
    } else if (m.type === 'result') {
      clear();
      const result = m.result;
      // Denial deliberately has no successful provider-completion audit.
      const confirmed = !m.error && result && (m.approved ? result.audit_complete === true && result.execution_state === 'succeeded' : result.execution_state === 'not-executed') &&
        (result.saved_choice ?? '') === m.save;
      el('status').textContent = confirmed ?
        (m.approved ? text('Allowed. The controller reported success. ', '許可しました。管理処理の成功が報告されました。') : text('Denied. The operation was not executed. ', '拒否しました。操作は実行されていません。')) +
          text('Saved Policy: ', '保存した設定: ') + optionLabel(result.saved_choice ?? '') :
        text('Outcome is unconfirmed. Inspect Policy and audit before retrying; nothing is automatically resent.', '結果を確定できません。再試行の前に設定と監査を確認してください。自動再送はしません。');
    } else if (m.type === 'error') {
      clear(); el('status').textContent = text('The request could not be reviewed or changed before submission. Refresh and inspect it again.', '操作を確認できないか、回答前に内容が変わりました。更新してもう一度確認してください。');
    } else if (m.type === 'unavailable') {
      clear(); closed = true;
      el('status').textContent = text('Review connection ended. A submitted answer may have taken effect. Inspect Policy and audit before retrying.', '確認用の接続が終了しました。送信済みの回答は反映されている場合があります。再試行の前に設定と監査を確認してください。');
    }
    controls();
  });
  controls(); api.postMessage({ type: 'ready' });
}

function panelHTML(nonce, japanese) {
  if (typeof nonce !== 'string' || !/^[a-f0-9]{36}$/.test(nonce)) throw new Error('invalid nonce');
  return `<!doctype html><html lang="${japanese ? 'ja' : 'en'}"><head><meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'nonce-${nonce}'; style-src 'nonce-${nonce}'; base-uri 'none'; form-action 'none'">
<meta name="viewport" content="width=device-width, initial-scale=1"><title>Hacocoon Approval</title>
<style nonce="${nonce}">body{font-family:var(--vscode-font-family);color:var(--vscode-foreground);background:var(--vscode-editor-background);padding:20px;max-width:920px;line-height:1.6}h1{font-size:1.5em}h2{font-size:1.15em;margin-top:1.8em}button,select{font:inherit;padding:8px 12px;color:var(--vscode-button-foreground);background:var(--vscode-button-background);border:1px solid var(--vscode-contrastBorder,transparent);border-radius:3px;max-width:100%}button{cursor:pointer}button:disabled,select:disabled{opacity:.5;cursor:default}button:focus,select:focus{outline:2px solid var(--vscode-focusBorder)}.request{display:block;text-align:left;margin:8px 0;white-space:pre-wrap;overflow-wrap:anywhere;width:100%}.field{border-bottom:1px solid var(--vscode-panel-border);padding:8px 0}pre{white-space:pre-wrap;overflow-wrap:anywhere;margin:4px 0;font-family:var(--vscode-editor-font-family)}#status{border-left:3px solid var(--vscode-focusBorder);padding:12px}#actions{display:flex;gap:12px;margin-top:24px;flex-wrap:wrap}#deny{background:var(--vscode-button-secondaryBackground);color:var(--vscode-button-secondaryForeground)}label{display:block;margin:12px 0}select{color:var(--vscode-dropdown-foreground);background:var(--vscode-dropdown-background)}</style></head>
<body><h1 id="heading"></h1><p id="intro"></p><p id="source"></p><button id="refresh"></button>
<h2 id="pendingHeading"></h2><div id="pending"></div><p id="status" role="status" aria-live="polite"></p>
<section id="detail" hidden><h2 id="currentHeading"></h2><div id="current"></div><label id="futureLabel" for="save"></label><select id="save"></select><h2 id="scopeHeading"></h2><p id="scopeNote"></p><div id="scope"></div><div id="actions"><button id="deny"></button><button id="allow"></button></div></section>
<script nonce="${nonce}">(${browserMain.toString()})(${japanese === true});</script></body></html>`;
}
module.exports = { panelHTML };
