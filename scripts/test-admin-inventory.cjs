const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const read=name=>fs.readFileSync(path.join(__dirname,'..',name),'utf8');
const source=read('static/js/admin-inventory.js');
const book={bookId:12,title:'Livre',quantity:5,lowStockThreshold:2,version:3,stockStatus:'LOW_STOCK'};
const page=(offset=0,results=[book],total=11)=>({offset,results,total,limit:10});
function setup(api) {
 const nodes=new Map(),h={calls:[],root:false,audits:0,confirmed:true};
 const get=id=>{
  if(!nodes.has(id))nodes.set(id,{handlers:{},elements:{},rows:[],closed:0,opened:0,
   addEventListener(event,fn){(this.handlers[event]||=[]).push(fn);},
   replaceChildren(){this.rows=[];},insertRow(){const row={cells:[]};this.rows.push(row);return row;},
   reset(){for(const f of Object.values(this.elements))f.value='';},close(){this.closed++;},showModal(){this.opened++;}
  });return nodes.get(id);
 };
 for(const name of ['bookId','version','operation','quantity','reason'])get('#inventory-form').elements[name]={value:''};
 for(const name of ['status','libraryId'])get('#inventory-filters').elements[name]={value:''};
 const window={confirm:()=>h.confirmed};vm.runInNewContext(source,{window,document:{querySelector:get},URLSearchParams,Intl});
 h.get=get;h.errorBox={};
 h.module=window.DeftaInventory.create({
  apiFetch:async(url,options)=>{h.calls.push({url,options});return api?api(url,options):page();},
  textCell(row,value){const cell={classList:{add(){}},textContent:value,replaceChildren(...buttons){this.buttons=buttons;}};row.cells.push(cell);return cell;},
  actionButton:(label,action,id)=>({label,action,id}),formatDate:value=>value,
  showError:(box,error)=>{box.textContent=error.message;box.hidden=false;},errorBox:h.errorBox,isRoot:()=>h.root,
  reloadAudit:async()=>{h.audits++;}
 });
 h.event=async(id,event='click',action='edit-inventory',bookID='12')=>{
  for(const fn of get(id).handlers[event]||[])await fn({preventDefault(){},currentTarget:get(id),target:{closest:()=>({dataset:{action,id:bookID}})}});
 };return h;
}
test('load is inert',()=>{const window={};vm.runInNewContext(source,{window});assert.equal(typeof window.DeftaInventory.create,'function');});
test('root filters include library; owner filters do not',async()=>{
 const h=setup();h.get('#inventory-filters').elements.libraryId.value='library';h.get('#inventory-filters').elements.status.value='LOW_STOCK';
 await h.module.reload();assert.equal(new URL(h.calls[0].url,'https://example.test').searchParams.has('libraryId'),false);
 h.root=true;await h.module.reload();const q=new URL(h.calls[1].url,'https://example.test').searchParams;
 assert.equal(q.get('libraryId'),'library');assert.equal(q.get('status'),'LOW_STOCK');
});
test('pagination resets on filter submit and init is idempotent',async()=>{
 const h=setup(async url=>page(Number(new URL(url,'https://example.test').searchParams.get('offset'))));h.module.init();h.module.init();
 await h.event('#inventory-next');await h.event('#inventory-previous');await h.event('#inventory-filters','submit');
 assert.deepEqual(h.calls.map(c=>new URL(c.url,'https://example.test').searchParams.get('offset')),['10','0','0']);
});
for(const [operation,suffix,method] of [['ENTRY','/entries','POST'],['EXIT','/exits','POST'],['ADJUSTMENT','','PUT'],['THRESHOLD','/threshold','PATCH']])test(operation,async()=>{
 const h=setup();h.module.init();await h.module.reload();await h.event('#inventory-body');
 const f=h.get('#inventory-form').elements;assert.equal(f.version.value,3);f.operation.value=operation;f.quantity.value='2';f.reason.value=' motif ';
 h.calls.length=0;await h.event('#inventory-form','submit');
 assert.equal(h.calls[0].url,'/api/manage/books/12/inventory'+suffix);assert.equal(h.calls[0].options.method,method);
 const body=JSON.parse(h.calls[0].options.body);assert.equal(body.version,3);
 assert.equal(body[operation==='THRESHOLD'?'lowStockThreshold':'quantity'],2);
 if(operation!=='THRESHOLD')assert.equal(body.reason,'motif');
 assert.equal(h.audits,1);assert.equal(h.get('#inventory-dialog').closed,1);
});
test('invalid quantity or missing adjustment reason sends no mutation',async()=>{
 const h=setup();h.module.init();const f=h.get('#inventory-form').elements;
 for(const op of ['ENTRY','EXIT','ADJUSTMENT']){f.operation.value=op;f.quantity.value='0';f.reason.value='';await h.event('#inventory-form','submit');}
 assert.equal(h.calls.length,0);
});
test('server refusal keeps form open without refresh',async()=>{
 const h=setup(async()=>{throw new Error('Stock insuffisant');});h.module.init();const f=h.get('#inventory-form').elements;f.operation.value='EXIT';f.quantity.value='20';
 await h.event('#inventory-form','submit');assert.equal(h.calls.length,1);assert.equal(h.audits,0);assert.equal(h.get('#inventory-dialog').closed,0);assert.match(h.get('#inventory-form-error').textContent,/insuffisant/);
});
test('history preserves signed movement and reason as text',async()=>{
 const h=setup(async url=>url.includes('/movements')?{results:[{quantityDelta:2,quantityBefore:3,quantityAfter:5,reason:'<b>texte</b>'}]}:page());
 h.module.init();await h.module.reload();await h.event('#inventory-body','click','history-inventory');
 assert.equal(h.calls[1].url,'/api/manage/books/12/inventory/movements?offset=0&limit=100');
 const cells=h.get('#inventory-history-body').rows[0].cells;assert.equal(cells[2].textContent,'+2');assert.equal(cells[5].textContent,'<b>texte</b>');
});
test('dashboard retains stock refresh hook',()=>{
 const auth=read('static/js/admin-auth.js');assert.match(auth,/const reloadInventory = \(\) => inventory.reload\(\)/);
 const template=read('templates/admin.html');assert.ok(template.indexOf('/static/js/admin-inventory.js')<template.indexOf('/static/js/admin-auth.js'));
 assert.doesNotMatch(read('templates/login.html'),/admin-inventory/);
});
