const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs'), path=require('node:path'), vm=require('node:vm');
const read=name=>fs.readFileSync(path.join(__dirname,'..',name),'utf8');
const source=read('static/js/admin-owners.js');
const owner={id:'owner',username:'test',email:'test@example.test',status:'ACTIVE',library:{id:'library',name:'Librairie',status:'ACTIVE'}};
const page=(offset=0,results=[owner],total=11)=>({offset,results,total,limit:10});
function setup(api) {
 const nodes=new Map(), h={calls:[],audits:0,sessions:0,options:[],confirmed:true};
 const get=id=>{
  if(!nodes.has(id)) nodes.set(id,{handlers:{},elements:{},rows:[],closed:0,opened:0,
   addEventListener(event,fn){(this.handlers[event]||=[]).push(fn);},
   replaceChildren(){this.rows=[];},insertRow(){const row={cells:[]};this.rows.push(row);return row;},
   reset(){for(const field of Object.values(this.elements))field.value='';},querySelectorAll(){return [];},
   close(){this.closed++;},showModal(){this.opened++;}
  });return nodes.get(id);
 };
 for(const [id,fields] of Object.entries({
  '#owner-filters':['q','status','libraryStatus'],
  '#owner-form':['id','username','email','password','status','libraryName','libraryDescription','libraryStatus'],
  '#owner-password-reset-form':['id','password','confirmation']
 })) for(const name of fields)get(id).elements[name]={value:''};
 const window={confirm:()=>h.confirmed};vm.runInNewContext(source,{window,document:{querySelector:get},URLSearchParams});
 h.get=get;h.errorBox={};
 h.module=window.DeftaOwners.create({
  apiFetch:async(url,options)=>{h.calls.push({url,options});return api?api(url,options):page();},
  textCell(row,value){const cell={textContent:value,replaceChildren(...buttons){this.buttons=buttons;}};row.cells.push(cell);return cell;},
  actionButton:(label,action,id)=>({label,action,id}),showError:(box,error)=>{box.textContent=error.message;box.hidden=false;},errorBox:h.errorBox,
  updateLibraryOptions:options=>{h.options=options;},reloadAudit:async()=>{h.audits++;},reloadSessions:async()=>{h.sessions++;}
 });
 h.event=async(id,event='click',action='edit-owner',ownerID='owner')=>{
  for(const fn of get(id).handlers[event]||[])await fn({preventDefault(){},currentTarget:get(id),target:{closest:()=>({dataset:{action,id:ownerID}})}});
 };
 return h;
}
test('module load performs no DOM or authentication work',()=>{const window={};vm.runInNewContext(source,{window});assert.equal(typeof window.DeftaOwners.create,'function');});
test('filters, pagination, reset and idempotent init',async()=>{
 const h=setup(async url=>page(Number(new URL(url,'https://example.test').searchParams.get('offset'))));
 h.module.init();h.module.init();h.get('#owner-filters').elements.q.value=' A & B ';
 await h.event('#owners-next');await h.event('#owners-previous');await h.event('#owner-filters','submit');
 assert.deepEqual(h.calls.map(c=>new URL(c.url,'https://example.test').searchParams.get('offset')),['10','0','0']);
 assert.equal(new URL(h.calls[0].url,'https://example.test').searchParams.get('q'),'A & B');
});
test('active library options traverse pages and stop on empty shrinking page',async()=>{
 const h=setup(async()=>page(0,[owner],2));await h.module.reloadOptions();assert.equal(h.calls.length,2);assert.equal(h.options.length,2);
 const shrinking=setup(async()=>shrinking.calls.length===1?page(0,[owner],2):page(1,[],2));
 await shrinking.module.reloadOptions();assert.equal(shrinking.calls.length,2);assert.equal(shrinking.options.length,1);
 for(const c of shrinking.calls){const q=new URL(c.url,'https://example.test').searchParams;assert.equal(q.get('status'),'ACTIVE');assert.equal(q.get('libraryStatus'),'ACTIVE');}
});
for(const [status,actions] of [['ACTIVE',['edit-owner','reset-owner-password','disable-owner']],['LOCKED',['reset-owner-password','unlock-owner']],['DISABLED',['reset-owner-password','reactivate-owner']]])test(`actions for ${status}`,async()=>{
 const h=setup(async()=>page(0,[{...owner,status}]));await h.module.reload();
 assert.deepEqual(Array.from(h.get('#owners-body').rows[0].cells[4].buttons,b=>b.action),actions);
});
test('creation sends POST and update omits blank password',async()=>{
 const h=setup();h.module.init();const fields=h.get('#owner-form').elements;
 fields.username.value='new';fields.password.value='temporary-test';fields.libraryName.value='New';
 await h.event('#owner-form','submit');assert.equal(h.calls[0].options.method,'POST');
 assert.equal(JSON.parse(h.calls[0].options.body).password,'temporary-test');
 h.calls.length=0;fields.id.value='owner';fields.password.value='';
 await h.event('#owner-form','submit');assert.equal(h.calls[0].options.method,'PATCH');
 assert.equal(Object.hasOwn(JSON.parse(h.calls[0].options.body),'password'),false);
 assert.equal(h.get('#owner-dialog').closed,2);
});
test('failed save leaves dialog open and shows error',async()=>{
 const h=setup(async()=>{throw new Error('Refus');});h.module.init();await h.event('#owner-form','submit');
 assert.equal(h.get('#owner-dialog').closed,0);assert.equal(h.get('#owner-form-error').textContent,'Refus');assert.equal(h.calls.length,1);
});
test('password reset checks confirmation then refreshes audit and sessions',async()=>{
 const h=setup();h.module.init();const f=h.get('#owner-password-reset-form').elements;
 f.id.value='owner';f.password.value='temporary-test';f.confirmation.value='other';
 await h.event('#owner-password-reset-form','submit');assert.equal(h.calls.length,0);
 f.confirmation.value=f.password.value;await h.event('#owner-password-reset-form','submit');
 assert.equal(h.calls[0].url,'/api/admin/owners/owner/reset-password');assert.equal(h.audits,1);assert.equal(h.sessions,1);
});
for(const [action,suffix,method] of [['disable-owner','','DELETE'],['unlock-owner','/unlock','POST'],['reactivate-owner','/reactivate','POST']])test(action,async()=>{
 const h=setup();h.module.init();await h.module.reload();h.calls.length=0;
 await h.event('#owners-body','click',action);assert.equal(h.calls[0].url,'/api/admin/owners/owner'+suffix);assert.equal(h.calls[0].options.method,method);
 assert.equal(h.sessions,action==='disable-owner'?0:1);
});
test('cancelled and stale actions do not send mutations',async()=>{
 const h=setup();h.module.init();await h.module.reload();h.calls.length=0;h.confirmed=false;
 await h.event('#owners-body','click','disable-owner');h.confirmed=true;
 await h.event('#owners-body','click','unlock-owner','missing');assert.equal(h.calls.length,0);
});
test('dashboard loads module before auth and initializes only for root',()=>{
 const template=read('templates/admin.html');assert.ok(template.indexOf('/static/js/admin-owners.js')<template.indexOf('/static/js/admin-auth.js'));
 assert.doesNotMatch(read('templates/login.html'),/admin-owners/);
 assert.match(read('static/js/admin-auth.js'),/if \(isRoot\) \{ owners.init\(\)/);
});
