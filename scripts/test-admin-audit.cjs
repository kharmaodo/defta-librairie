const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const read = name => fs.readFileSync(path.join(__dirname, '..', name), 'utf8');
const source = read('static/js/admin-audit.js');
function setup(apiFetch) {
  const nodes = new Map();
  const get = id => {
    if (!nodes.has(id)) nodes.set(id, {handlers:{}, rows:[], elements:{},
      addEventListener(event, fn) { (this.handlers[event] ||= []).push(fn); },
      replaceChildren() { this.rows = []; },
      insertRow() { const row = {cells:[]}; this.rows.push(row); return row; }
    });
    return nodes.get(id);
  };
  for (const name of ['actor','action','resourceType','resourceId','success','from','to']) get('#audit-filters').elements[name] = {value:''};
  const window = {};
  vm.runInNewContext(source, {window, document:{querySelector:get}, URLSearchParams, Date});
  const errorBox = {};
  const module = window.DeftaAudit.create({apiFetch, errorBox,
    textCell(row, value, className) { const cell = {textContent:value, className}; row.cells.push(cell); return cell; },
    showError(box, error) { box.textContent = error.message; box.hidden = false; }
  });
  return {module, get, errorBox, async event(id, event='click') {
    for (const handler of get(id).handlers[event] || []) await handler({preventDefault(){}});
  }};
}
const entry = {createdAt:'2026-09-10T00:00:00Z', action:'<script>text only</script>', actorUserId:'owner', resourceType:'BOOK', resourceId:'1', success:false, ipAddress:'127.0.0.1'};
const page = (offset=0, results=[entry], total=21) => ({offset, results, total, limit:20});
test('loading module does not query DOM or start authentication', () => {
  const window = {};
  vm.runInNewContext(source, {window});
  assert.equal(typeof window.DeftaAudit.create, 'function');
});
test('filters preserve false, trim values and encode dates', async () => {
  let url;
  const h = setup(async value => { url = new URL(value, 'https://example.test'); return page(); });
  h.get('#audit-filters').elements.actor.value = ' A & B ';
  h.get('#audit-filters').elements.success.value = 'false';
  h.get('#audit-filters').elements.from.value = '2026-09-10T00:00:00Z';
  await h.module.reload();
  assert.equal(url.pathname, '/api/audit-logs');
  assert.equal(url.searchParams.get('actor'), 'A & B');
  assert.equal(url.searchParams.get('success'), 'false');
  assert.equal(url.searchParams.get('from'), '2026-09-10T00:00:00.000Z');
  assert.equal(url.searchParams.has('action'), false);
  assert.equal(h.get('#audit-body').rows[0].cells[1].textContent, entry.action);
  assert.equal(h.get('#audit-body').rows[0].cells[4].textContent, 'Échec');
});
test('init is idempotent, next/previous and filters reset pagination', async () => {
  const offsets = [];
  const h = setup(async value => { const offset = Number(new URL(value,'https://example.test').searchParams.get('offset')); offsets.push(offset); return page(offset); });
  h.module.init(); h.module.init();
  await h.event('#audit-next');
  assert.equal(h.get('#audit-page-label').textContent, 'Page 2 sur 2');
  assert.equal(h.get('#audit-next').disabled, true);
  await h.event('#audit-previous');
  await h.event('#audit-next');
  await h.event('#audit-filters','submit');
  assert.deepEqual(offsets, [20,0,20,0]);
  assert.equal(h.get('#audit-previous').disabled, true);
});
test('empty later page falls back and renders empty first page', async () => {
  const offsets = [];
  const h = setup(async value => { const offset = Number(new URL(value,'https://example.test').searchParams.get('offset')); offsets.push(offset); return page(offset,[],0); });
  h.module.init(); await h.event('#audit-next');
  assert.deepEqual(offsets,[20,0]);
  assert.equal(h.get('#audit-body').rows[0].cells[0].colSpan, 6);
  assert.equal(h.get('#audit-next').disabled, true);
});
test('errors surface once without retrying the supplied client', async () => {
  let calls = 0;
  const h = setup(async () => { calls++; throw new Error('Session expirée'); });
  h.module.init(); await h.event('#audit-filters','submit');
  assert.equal(calls,1);
  assert.equal(h.errorBox.textContent,'Session expirée');
  assert.equal(h.errorBox.hidden,false);
});
test('dashboard loads module first and retains refresh hook', () => {
  const template = read('templates/admin.html');
  assert.ok(template.indexOf('/static/js/admin-audit.js') < template.indexOf('/static/js/admin-auth.js'));
  const auth = read('static/js/admin-auth.js');
  assert.match(auth, /DeftaAudit\.create\(\{apiFetch, textCell, showError, errorBox\}\)/);
  assert.match(auth, /const reloadAudit = \(\) => audit\.reload\(\)/);
  assert.doesNotMatch(read('templates/login.html'), /admin-audit/);
});
