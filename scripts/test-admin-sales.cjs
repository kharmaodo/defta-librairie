const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const read=name=>fs.readFileSync(path.join(__dirname,'..',name),'utf8');
const source=read('static/js/admin-sales.js');
const book={id:'sale',reference:'V1',version:3,status:'DRAFT',lines:[{bookId:12,quantity:2}],totalAmount:2000};
const page=(offset=0,results=[book],total=11)=>({offset,results,total,limit:10});
function setup(api) {
 const nodes=new Map(),h={calls:[],root:false,audits:0,stocks:0,confirmed:true};
 const get=id=>{
  if(!nodes.has(id))nodes.set(id,{handlers:{},elements:{},rows:[],closed:0,opened:0,
   addEventListener(event,fn){(this.handlers[event]||=[]).push(fn);},
   replaceChildren(){this.rows=[];},insertRow(){const row={cells:[]};this.rows.push(row);return row;},
   reset(){for(const f of Object.values(this.elements))f.value='';},close(){this.closed++;},showModal(){this.opened++;}
  });return nodes.get(id);
 };
 for(const name of ['id','version','customerId','customerName','libraryId'])get('#sale-form').elements[name]={value:''};
 for(const name of ['status','libraryId','from','to'])get('#sale-filters').elements[name]={value:''};
 const window={confirm:()=>h.confirmed};vm.runInNewContext(source,{window,document:{querySelector:get,querySelectorAll:()=>h.lines||[]},URLSearchParams,Intl});
 h.get=get;h.errorBox={};
 h.module=window.DeftaSales.create({
  apiFetch:async(url,options)=>{h.calls.push({url,options});return api?api(url,options):page();},
  textCell(row,value){const cell={classList:{add(){}},textContent:value,replaceChildren(...buttons){this.buttons=buttons;}};row.cells.push(cell);return cell;},
  actionButton:(label,action,id)=>({label,action,id}),formatDate:value=>value,
  showError:(box,error)=>{box.textContent=error.message;box.hidden=false;},errorBox:h.errorBox,isRoot:()=>h.root,
  reloadAudit:async()=>{h.audits++;},reloadInventory:async()=>{h.stocks++;}
 });
 h.event=async(id,event='click',action='confirm-sale',bookID='sale')=>{
  h.button={dataset:{action,id:bookID}};
  for(const fn of get(id).handlers[event]||[])await fn({preventDefault(){},currentTarget:get(id),target:{closest:()=>h.button}});
 };return h;
}
test('module load is inert',()=>{const window={};vm.runInNewContext(source,{window});assert.equal(typeof window.DeftaSales.create,'function');});
test('filters preserve role scope and pagination resets',async()=>{
 const h=setup();h.module.init();h.module.init();h.get('#sale-filters').elements.libraryId.value='library';
 await h.event('#sales-next');assert.equal(new URL(h.calls[0].url,'https://example.test').searchParams.has('libraryId'),false);
 h.root=true;await h.event('#sale-filters','submit');const q=new URL(h.calls[1].url,'https://example.test').searchParams;
 assert.equal(q.get('offset'),'0');assert.equal(q.get('libraryId'),'library');assert.equal(h.calls.length,2);
});
for(const action of ['confirm','cancel'])test(action+' sends version and refreshes stock/audit',async()=>{
 const h=setup();h.module.init();await h.module.reload();h.calls.length=0;
 await h.event('#sales-body','click',action+'-sale');assert.equal(h.calls[0].url,'/api/manage/sales/sale/'+action);
 assert.equal(h.calls[0].options.method,'POST');assert.deepEqual(JSON.parse(h.calls[0].options.body),{version:3});
 assert.equal(h.stocks,1);assert.equal(h.audits,1);assert.equal(h.button.disabled,false);
});
test('failed transition preserves original error without repeating mutation',async()=>{
 const h=setup(async(url,options)=>{if(options)throw new Error('Stock insuffisant');if(h.calls.length>2)throw new Error('Lecture impossible');return page();});
 h.module.init();await h.module.reload();await h.event('#sales-body');
 assert.equal(h.calls.filter(c=>c.options).length,1);assert.equal(h.stocks,0);assert.equal(h.audits,0);
 assert.equal(h.errorBox.textContent,'Stock insuffisant');assert.equal(h.button.disabled,false);
});
test('cancelled confirmation sends no mutation',async()=>{
 const h=setup();h.module.init();await h.module.reload();h.calls.length=0;h.confirmed=false;await h.event('#sales-body');assert.equal(h.calls.length,0);
});
test('draft deletion refreshes list and audit',async()=>{
 const h=setup();h.module.init();await h.module.reload();h.calls.length=0;await h.event('#sales-body','click','delete-sale');
 assert.equal(h.calls[0].options.method,'DELETE');assert.equal(h.audits,1);assert.equal(h.stocks,0);
});
const line=(id,quantity)=>({querySelector:selector=>({value:selector.includes('bookId')?String(id):String(quantity)})});
test('create payload and update version retain customer link',async()=>{
 const h=setup();h.module.init();h.lines=[line(12,2)];const f=h.get('#sale-form').elements;f.customerId.value='customer';f.customerName.value=' Client ';
 await h.event('#sale-form','submit');let payload=JSON.parse(h.calls[0].options.body);
 assert.equal(h.calls[0].options.method,'POST');assert.equal(payload.customerId,'customer');assert.deepEqual(payload.lines,[{bookId:12,quantity:2}]);assert.equal(payload.customerName,'Client');
 h.calls.length=0;f.id.value='sale';f.version.value='3';await h.event('#sale-form','submit');
 assert.equal(h.calls[0].options.method,'PUT');assert.equal(JSON.parse(h.calls[0].options.body).version,3);
});
test('empty, duplicate lines and missing root library are rejected',async()=>{
 const h=setup();h.module.init();await h.event('#sale-form','submit');h.lines=[line(12,1),line(12,2)];await h.event('#sale-form','submit');
 h.lines=[line(12,1)];h.root=true;await h.event('#sale-form','submit');assert.equal(h.calls.length,0);
});
test('failed save keeps dialog open',async()=>{
 const h=setup(async()=>{throw new Error('Conflit');});h.module.init();h.lines=[line(12,1)];await h.event('#sale-form','submit');
 assert.equal(h.get('#sale-dialog').closed,0);assert.equal(h.get('#sale-form-error').textContent,'Conflit');
});
test('module loaded before dashboard and excluded from login',()=>{
 const t=read('templates/admin.html');assert.ok(t.indexOf('/static/js/admin-sales.js')<t.indexOf('/static/js/admin-auth.js'));
 assert.doesNotMatch(read('templates/login.html'),/admin-sales/);
});
