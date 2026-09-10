const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const read = name => fs.readFileSync(path.join(__dirname,'..',name),'utf8');
const source = read('static/js/admin-sessions.js');
const page = (offset=0, results=[{id:'current',username:'owner',role:'OWNER_LIBRARY'}], total=21) => ({offset,results,total,limit:20,currentSessionId:'current'});
function setup(api) {
  const nodes = new Map(), calls = [];
  const h = {confirmed:true, disconnected:0, audits:0, calls};
  const get = id => {
    if (!nodes.has(id)) nodes.set(id,{handlers:{},rows:[],elements:{},
      addEventListener(event,fn) { (this.handlers[event] ||= []).push(fn); },
      replaceChildren() { this.rows=[]; },
      insertRow() { const row={cells:[]}; this.rows.push(row); return row; }
    });
    return nodes.get(id);
  };
  for(const name of ['username','role','ipAddress','userAgent']) get('#session-filters').elements[name]={value:''};
  const window={confirm:()=>h.confirmed};
  vm.runInNewContext(source,{window,document:{querySelector:get},URLSearchParams});
  h.errorBox={}; h.get=get;
  h.module=window.DeftaSessions.create({
    apiFetch:async (url,options) => {calls.push({url,options}); return api ? api(url,options) : page();},
    textCell(row,value) {const cell={textContent:value,replaceChildren(button){this.button=button;}};row.cells.push(cell);return cell;},
    actionButton:(label,action,id)=>({label,action,id}),formatDate:value=>value,
    showError:(box,error)=>{box.textContent=error.message;box.hidden=false;},errorBox:h.errorBox,
    reloadAudit:async()=>{h.audits++;},onCurrentRevoked:()=>{h.disconnected++;}
  });
  h.event=async(id,event='click',session='current')=>{
    for(const fn of get(id).handlers[event]||[]) await fn({preventDefault(){},target:{closest:()=>({dataset:{id:session}})}});
  };
  return h;
}
test('module load is inert without a document or authentication',()=>{
  const window={};vm.runInNewContext(source,{window});assert.equal(typeof window.DeftaSessions.create,'function');
});
test('filters encode values and render current session action',async()=>{
  const h=setup();h.get('#session-filters').elements.username.value=' A & B ';
  await h.module.reload();
  assert.equal(new URL(h.calls[0].url,'https://example.test').searchParams.get('username'),'A & B');
  assert.equal(h.get('#sessions-body').rows[0].cells[7].button.label,'Révoquer et quitter');
});
test('init once, pagination and filter reset',async()=>{
  const h=setup(async url=>page(Number(new URL(url,'https://example.test').searchParams.get('offset'))));
  h.module.init();h.module.init();
  await h.event('#sessions-next');await h.event('#sessions-previous');await h.event('#sessions-next');
  await h.event('#session-filters','submit');
  assert.deepEqual(h.calls.map(c=>new URL(c.url,'https://example.test').searchParams.get('offset')),['20','0','20','0']);
  assert.equal(h.get('#sessions-previous').disabled,true);
});
test('empty later page falls back to first',async()=>{
  const h=setup(async url=>page(Number(new URL(url,'https://example.test').searchParams.get('offset')),[],0));
  h.module.init();await h.event('#sessions-next');
  assert.equal(h.calls.length,2);assert.equal(h.get('#sessions-body').rows[0].cells[0].colSpan,8);
});
test('current revocation disconnects only after DELETE resolves',async()=>{
  let resolveDelete;
  const h=setup(async (url,options)=>options?.method==='DELETE'?new Promise(resolve=>{resolveDelete=resolve;}):page());
  h.module.init();await h.module.reload();
  const pending=h.event('#sessions-body');
  assert.equal(h.disconnected,0);resolveDelete(null);await pending;
  assert.equal(h.disconnected,1);assert.equal(h.audits,0);assert.equal(h.calls.length,2);
});
test('refused revocation retains session and surfaces error',async()=>{
  const h=setup(async (url,options)=>{if(options?.method==='DELETE')throw new Error('Accès refusé');return page();});
  h.module.init();await h.module.reload();await h.event('#sessions-body');
  assert.equal(h.disconnected,0);assert.equal(h.audits,0);assert.equal(h.errorBox.textContent,'Accès refusé');assert.equal(h.calls.length,2);
});
test('other session revocation refreshes list and audit',async()=>{
  const h=setup();h.module.init();await h.module.reload();await h.event('#sessions-body','click','other');
  assert.equal(h.calls[1].url,'/api/auth/sessions/other');assert.equal(h.calls[1].options.method,'DELETE');
  assert.equal(h.audits,1);assert.equal(h.disconnected,0);assert.equal(h.calls.length,3);
});
test('revoke others keeps current session and displays count',async()=>{
  const h=setup(async (url,options)=>options?.method==='POST'?{revoked:2}:page());
  h.module.init();await h.event('#revoke-other-sessions-button');
  assert.equal(h.calls[0].url,'/api/auth/sessions/revoke-others');assert.equal(h.calls[0].options.method,'POST');
  assert.equal(h.disconnected,0);assert.equal(h.audits,1);assert.match(h.get('#session-scope-note').textContent,/2 autre/);
});
test('cancel confirmations send no requests',async()=>{
  const h=setup();h.module.init();h.confirmed=false;
  await h.event('#sessions-body');await h.event('#revoke-other-sessions-button');assert.equal(h.calls.length,0);
});
test('module loads before dashboard, not on login',()=>{
  const template=read('templates/admin.html');assert.ok(template.indexOf('/static/js/admin-sessions.js')<template.indexOf('/static/js/admin-auth.js'));
  assert.doesNotMatch(read('templates/login.html'),/admin-sessions/);
  assert.match(read('static/js/admin-auth.js'),/const reloadSessions = \(\) => sessions.reload\(\)/);
});
