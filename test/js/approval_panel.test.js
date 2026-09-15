"use strict";
const assert = require('node:assert/strict');
const test = require('node:test');
const vm = require('node:vm');
const {panelHTML} = require('../../clients/vscode-notify/review_panel');

// Run the actual bundled browser script against a minimal DOM. Assertions are
// about explicit clicks, visible scope and receipts, not a duplicate UI model.
function browser(japanese = false) {
  class Element {
    constructor(tag) { this.tag = tag; this.children = []; this.events = {}; this.value = ''; this.textContent = ''; this.disabled = false; }
    append(...nodes) { this.children.push(...nodes); }
    replaceChildren(...nodes) { this.children = nodes; }
    addEventListener(type, fn) { this.events[type] = fn; }
    querySelectorAll(tag) { return this.children.filter(n => n.tag === tag); }
    click() { this.events.click?.(); }
  }
  const html = panelHTML('a'.repeat(36), japanese);
  const elements = new Map([...html.matchAll(/id="([A-Za-z]+)"/g)].map(m => [m[1], new Element(m[1])]));
  const writes = []; let receive;
  vm.runInNewContext(html.match(/<script nonce="[a-f0-9]+">([\s\S]*)<\/script>/)[1], {
    acquireVsCodeApi: () => ({postMessage: m => writes.push(JSON.parse(JSON.stringify(m)))}),
    document: {getElementById: id => elements.get(id), createElement: tag => new Element(tag)},
    window: {addEventListener: (type, fn) => { assert.equal(type, 'message'); receive = fn; }}
  });
  return {html, writes, el: id => elements.get(id), receive: data => receive({data}),
    text: id => { const content = node => node.textContent + node.children.map(content).join('\n'); return content(elements.get(id)); }};
}
const id = 'a'.repeat(32), token = 'b'.repeat(64);
function selection() {
  const current = {capability: 'git.remote', action: 'push', environment: 'dev', environment_instance: 'env-' + id,
    resource: 'https://example.com/owner/repo.git', attributes: {branch: 'main', update_kind: 'create', new_commit: 'c'.repeat(40)}};
  return {request: {request_id: id, request: current, reason: 'Approval required'}, token, digest: 'd'.repeat(64),
    options: ['allow', 'deny', 'ask'].flatMap(d => ['environment', 'global'].map(level => ({choice: `${d}-${level}`,
      scope: {...current, environment: level === 'global' ? '*' : 'dev', environment_instance: level === 'global' ? '' : current.environment_instance,
        decision: d === 'ask' ? 'require-approval' : d}})))};
}
test('isolated HTML has no network resources or command URIs and accepts no injected nonce', () => {
  const b = browser();
  assert.match(b.html, /default-src 'none'/); assert.match(b.html, /form-action 'none'/);
  assert.doesNotMatch(b.html, /<iframe|<img|src=|href=|innerHTML|command:/);
  assert.throws(() => panelHTML('" onload="evil', false));
  assert.deepEqual(b.writes, [{type: 'ready'}]); assert.equal(b.el('allow').disabled, true);
});
test('complete scope is displayed as literal text; initial choice is one shot and explicit current answer is required', () => {
  const b = browser(true), view = selection();
  view.request.request.resource = '</pre><script>alert(1)</script>\x1b\u202e';
  b.receive({type: 'selected', view});
  assert.match(b.text('current'), /<script>alert\(1\)<\/script>\\u001b\\u202e/);
  assert.match(b.text('current'), /new_commit/); assert.match(b.text('current'), /main/);
  assert.equal(b.el('save').value, ''); assert.equal(b.writes.length, 1);
  b.el('save').value = 'ask-global'; b.el('save').events.change();
  assert.match(b.text('scopeNote'), /今後作成するものを含むすべてのEnv/);
  assert.match(b.text('scope'), /require-approval/); assert.match(b.text('scope'), /new_commit/);
  b.el('deny').click(); b.el('allow').click();
  assert.deepEqual(b.writes[1], {type: 'decide', request_id: id, token, approved: false, save: 'ask-global'});
  assert.equal(b.writes.length, 2); assert.equal(b.el('allow').disabled, true);
});
test('saving allow or deny disables the inconsistent current answer; current Env binding stays visible', () => {
  const b = browser(); b.receive({type: 'selected', view: selection()});
  b.el('save').value = 'allow-environment'; b.el('save').events.change();
  assert.equal(b.el('deny').disabled, true); b.el('deny').click(); assert.equal(b.writes.length, 1);
  assert.match(b.text('scope'), new RegExp('env-' + id));
  b.el('save').value = 'deny-environment'; b.el('save').events.change();
  assert.equal(b.el('allow').disabled, true); assert.equal(b.el('deny').disabled, false);
});
test('removed selections and closed connections cannot be answered or retried automatically', () => {
  const b = browser(); b.receive({type: 'selected', view: selection()});
  b.receive({type: 'pending', pending: [], distribution: 'Hacocoon'});
  b.el('allow').click(); assert.equal(b.writes.length, 1); assert.equal(b.el('detail').hidden, true);
  b.receive({type: 'selected', view: selection()}); b.receive({type: 'unavailable'});
  b.el('allow').click(); b.el('refresh').click(); assert.equal(b.writes.length, 1);
  assert.match(b.text('status'), /may have taken effect/); assert.equal(b.el('refresh').disabled, true);
});
test('success requires the actual receipt, saved choice and audit; denial is distinct from failed execution', () => {
  for (const [approved, result, error, expected] of [
    [true, {execution_state: 'succeeded', audit_complete: true}, undefined, /Allowed/],
    [false, {execution_state: 'not-executed', audit_complete: false}, undefined, /Denied/],
    [true, {execution_state: 'succeeded', audit_complete: false}, undefined, /unconfirmed/],
    [true, {execution_state: 'failed', audit_complete: true}, undefined, /unconfirmed/],
    [true, {execution_state: 'succeeded', audit_complete: true}, 'outcome_unconfirmed', /unconfirmed/],
    [true, {execution_state: 'succeeded', audit_complete: true, saved_choice: 'allow-global'}, undefined, /unconfirmed/]
  ]) {
    const b = browser(); b.receive({type: 'selected', view: selection()});
    b.receive({type: 'result', approved, save: '', result, error});
    assert.match(b.text('status'), expected); assert.equal(b.writes.length, 1);
  }
});
