const {test} = require('node:test');
const assert = require('node:assert/strict');
const vm = require('node:vm');
const fs = require('node:fs');
const path = require('node:path');
const source = fs.readFileSync(path.join(__dirname, '../static/js/admin-http.js'), 'utf8');
function setup(fetch, token = 'test-token') {
  const window = {location: {origin: 'https://example.test'}};
  vm.runInNewContext(source, {window, URL, Headers, SyntaxError, sessionStorage: {getItem: () => token}, fetch});
  return window.DeftaHTTP;
}
const response = (status, body, type = 'application/json') => new Response(body, {status, headers: {'Content-Type': type}});
test('JSON, auth, body headers and redirect protection', async () => {
  let calls = 0;
  const api = setup(async (url, options) => {
    calls++;
    assert.equal(url, 'https://example.test/api/settings');
    assert.equal(options.headers.get('Authorization'), 'Bearer test-token');
    assert.equal(options.headers.get('Content-Type'), 'application/json');
    assert.equal(options.headers.get('If-Match'), '3');
    assert.equal(options.redirect, 'error');
    assert.equal(options.cache, 'no-store');
    assert.equal(options.method, 'PUT');
    return response(200, '{"ok":true}');
  });
  assert.deepEqual(await api.json('/api/settings', {method:'PUT', body:'{}', headers:{'If-Match':'3'}}), {ok:true});
  assert.equal(calls, 1);
});
test('204 and CSV remain readable', async () => {
  assert.equal(await setup(async () => response(204, null)).json('/api/settings'), null);
  const result = await setup(async () => response(200, 'a,b\n', 'text/csv')).request('/api/export');
  assert.equal(await result.text(), 'a,b\n');
});
for (const [status, code, match] of [
  [401,'unauthorized',/reconnectez/], [403,'forbidden',/droits/],
  [403,'password_change_required',/mot de passe/], [409,'version_conflict',/rechargez/],
  [422,'export_too_large',/10 000/], [422,'other',/Données invalides/],
  [400,'invalid_settings',/champs/], [404,'missing',/introuvable/],
  [429,'limited',/patientez/], [503,'session_validation_failed',/indisponible/]
]) test(`HTTP ${status} ${code}`, async () => {
  let calls = 0;
  const api = setup(async () => { calls++; return response(status, JSON.stringify({error:code, message:'PRIVATE DETAIL'})); });
  await assert.rejects(api.json('/api/test'), e => e.status === status && e.code === code && match.test(e.message) && !e.message.includes('PRIVATE'));
  assert.equal(calls, 1);
});
test('no token and off-origin URL never call fetch', async () => {
  const fetch = () => assert.fail('fetch must not run');
  await assert.rejects(setup(fetch, '').json('/api/test'), /reconnectez/);
  for (const url of ['https://other.test/api/test', '//other.test/api/test', '/static/test', '/api/../login']) {
    await assert.rejects(setup(fetch).request(url), /non autorisée/);
  }
});
test('abort identity and signal are preserved', async () => {
  const controller = new AbortController(); controller.abort();
  const abort = controller.signal.reason;
  const api = setup(async (_, options) => { assert.equal(options.signal, controller.signal); throw abort; });
  await assert.rejects(api.json('/api/test', {signal:controller.signal}), e => e === abort);
});
test('network failure does not retry writes', async () => {
  let calls = 0;
  const api = setup(async () => { calls++; throw new TypeError('private details'); });
  await assert.rejects(api.json('/api/test', {method:'PUT', body:'{}'}), /vérifiez si elle a été enregistrée/);
  assert.equal(calls, 1);
});
test('unexpected and malformed success bodies', async () => {
  await assert.rejects(setup(async () => response(200,'<html>', 'text/html')).json('/api/test'), /inattendue/);
  await assert.rejects(setup(async () => response(200,'{')).json('/api/test'), /JSON invalide/);
  await assert.rejects(setup(async () => response(503,'<html>', 'text/html')).json('/api/test'), /indisponible/);
});

for (const [code, match] of [
  ['payment_exceeds_balance', /reste à payer/],
  ['refund_exceeds_payments', /encaissements disponibles/],
  ['payment_has_issued_refunds', /ne peut pas être annulé/],
  ['supplier_return_insufficient_stock', /Stock insuffisant/],
  ['supplier_return_quantity_exceeded', /achat réceptionné/],
  ['return_quantity_exceeded', /retournable de la vente/],
  ['customer_conflict', /référence client existe/],
  ['cash_register_conflict', /caisse porte déjà/],
  ['purchase_not_editable', /état actuel/],
  ['return_settlement_conflict', /référence de règlement/]
]) test(`business refusal ${code}`, async () => {
  await assert.rejects(setup(async () => response(409, JSON.stringify({error:code, message:'PRIVATE'}))).json('/api/test'),
    e => match.test(e.message) && e.code === code && e.status === 409 && !e.message.includes('PRIVATE'));
});
test('business messages require the matching HTTP status', async () => {
  for (const status of [401,403,500]) {
    await assert.rejects(setup(async () => response(status, '{"error":"payment_exceeds_balance"}')).json('/api/test'),
      e => !e.message.includes('reste à payer'));
  }
  await assert.rejects(setup(async () => response(422, '{"error":"supplier_unavailable"}')).json('/api/test'), /fournisseur est indisponible/);
  await assert.rejects(setup(async () => response(400, '{"error":"invalid_purchase_line"}')).json('/api/test'), /ligne sélectionnée/);
});
test('unknown and prototype property codes have safe fallbacks', async () => {
  for (const code of ['new_business_code','__proto__','constructor','toString']) {
    await assert.rejects(setup(async () => response(409, JSON.stringify({error:code, message:'PRIVATE'}))).json('/api/test'), /rechargez/);
  }
});
