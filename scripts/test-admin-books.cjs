const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const read=name=>fs.readFileSync(path.join(__dirname,'..',name),'utf8');
const source=read('static/js/admin-books.js');
const book={id:12,title:'Livre',price:1000,version:3,libraryId:'library'};
const page=(offset=0,results=[book],total=11)=>({offset,results,total,limit:10});
function setup(api) {
 const nodes=new Map(),h={calls:[],root:false,stocks:0,tags:0,confirmed:true};
 const get=id=>{
  if(!nodes.has(id))nodes.set(id,{handlers:{},elements:{},rows:[],closed:0,opened:0,
   addEventListener(event,fn){(this.handlers[event]||=[]).push(fn);},
   replaceChildren(){this.rows=[];},insertRow(){const row={cells:[]};this.rows.push(row);return row;},
   reset(){for(const f of Object.values(this.elements))f.value='';},close(){this.closed++;},showModal(){this.opened++;}
  });return nodes.get(id);
 };
 for(const name of ['id','version','title','auteur','editeur','price','volume','status','categorie','tags','coverUrl','libraryId'])get('#book-form').elements[name]={value:''};
 get('#book-search-form').elements.q={value:''};
 const window={confirm:()=>h.confirmed};vm.runInNewContext(source,{window,document:{querySelector:get},URLSearchParams,Intl});
 h.get=get;h.errorBox={};
 h.module=window.DeftaBooks.create({
  apiFetch:async(url,options)=>{h.calls.push({url,options});return api?api(url,options):page();},
  textCell(row,value){const cell={textContent:value,replaceChildren(...buttons){this.buttons=buttons;}};row.cells.push(cell);return cell;},
  actionButton:(label,action,id)=>({label,action,id}),formatDate:value=>value,
  showError:(box,error)=>{box.textContent=error.message;box.hidden=false;},errorBox:h.errorBox,isRoot:()=>h.root,
  reloadInventory:async()=>{h.stocks++;},reloadTags:async()=>{h.tags++;},renderTags:()=>{}
 });
 h.event=async(id,event='click',action='edit-book',bookID='12')=>{
  for(const fn of get(id).handlers[event]||[])await fn({preventDefault(){},currentTarget:get(id),target:{closest:()=>({dataset:{action,id:bookID}})}});
 };return h;
}
test('module load is inert',()=>{const window={};vm.runInNewContext(source,{window});assert.equal(typeof window.DeftaBooks.create,'function');});
test('search trims and encodes input, resets page; init is idempotent',async()=>{
 const h=setup(async url=>page(Number(new URL(url,'https://example.test').searchParams.get('offset'))));h.module.init();h.module.init();
 await h.event('#books-next');await h.event('#books-previous');h.get('#book-search-form').elements.q.value=' A & B ';
 await h.event('#book-search-form','submit');
 assert.deepEqual(h.calls.map(c=>new URL(c.url,'https://example.test').searchParams.get('offset')),['10','0','0']);
 assert.equal(new URL(h.calls[2].url,'https://example.test').searchParams.get('q'),'A & B');
});
test('empty last page falls back',async()=>{
 const h=setup(async url=>page(Number(new URL(url,'https://example.test').searchParams.get('offset')),[],0));h.module.init();await h.event('#books-next');
 assert.equal(h.calls.length,2);assert.equal(h.get('#books-next').disabled,true);
});
test('owner creation excludes libraryId and sends numeric fields',async()=>{
 const h=setup();h.module.init();const f=h.get('#book-form').elements;f.price.value='1000';f.volume.value='2';f.libraryId.value='foreign';
 await h.event('#book-form','submit');const body=JSON.parse(h.calls[0].options.body);
 assert.equal(h.calls[0].options.method,'POST');assert.equal(body.price,1000);assert.equal(body.volume,2);assert.equal(Object.hasOwn(body,'libraryId'),false);
 assert.equal(Object.hasOwn(body,'version'),false);assert.equal(h.stocks,1);assert.equal(h.get('#book-dialog').closed,1);
});
test('root creation requires destination and uses current role',async()=>{
 const h=setup();h.module.init();h.root=true;await h.event('#book-form','submit');assert.equal(h.calls.length,0);
 h.get('#book-form').elements.libraryId.value='library';await h.event('#book-form','submit');
 assert.equal(JSON.parse(h.calls[0].options.body).libraryId,'library');
});
test('editing keeps version and locks library field',async()=>{
 const h=setup();h.module.init();await h.module.reload();await h.event('#books-body');const f=h.get('#book-form').elements;
 assert.equal(f.version.value,3);assert.equal(f.libraryId.disabled,true);h.calls.length=0;
 await h.event('#book-form','submit');assert.equal(h.calls[0].options.method,'PUT');assert.equal(h.calls[0].url,'/api/manage/books/12');
 assert.equal(JSON.parse(h.calls[0].options.body).version,3);
});
test('failed mutation keeps dialog open and does not refresh stock',async()=>{
 const h=setup(async()=>{throw new Error('Conflit de version');});h.module.init();await h.event('#book-form','submit');
 assert.equal(h.get('#book-dialog').closed,0);assert.equal(h.stocks,0);assert.equal(h.calls.length,1);assert.match(h.get('#book-form-error').textContent,/Conflit/);
});
test('delete refreshes stock; cancelled or stale delete does nothing',async()=>{
 const h=setup();h.module.init();await h.module.reload();h.calls.length=0;h.confirmed=false;
 await h.event('#books-body','click','delete-book');assert.equal(h.calls.length,0);h.confirmed=true;
 await h.event('#books-body','click','delete-book','999');assert.equal(h.calls.length,0);
 await h.event('#books-body','click','delete-book');assert.equal(h.calls[0].options.method,'DELETE');assert.equal(h.stocks,1);
});
test('history uses book id and renders server content as text',async()=>{
 const entry={createdAt:'date',action:'UPDATE',oldValues:'<script>old</script>',newValues:'new'};
 const h=setup(async url=>url.includes('/history')?{results:[entry]}:page());h.module.init();await h.module.reload();
 await h.event('#books-body','click','history-book');assert.equal(h.calls[1].url,'/api/manage/books/12/history?offset=0&limit=100');
 assert.equal(h.get('#book-history-body').rows[0].cells[3].textContent,entry.oldValues);assert.equal(h.get('#book-history-dialog').opened,1);
});
test('root library change refreshes tags; owner does not',async()=>{
 const h=setup();h.module.init();h.get('#book-form [name=libraryId]').value='library';
 await h.event('#book-form [name=libraryId]','change');assert.equal(h.tags,0);h.root=true;
 await h.event('#book-form [name=libraryId]','change');assert.equal(h.tags,1);assert.equal(h.get('#tag-library').value,'library');
});
test('dashboard integrates module and retains independent sales catalogue',()=>{
 const template=read('templates/admin.html');assert.ok(template.indexOf('/static/js/admin-books.js')<template.indexOf('/static/js/admin-auth.js'));
 assert.doesNotMatch(read('templates/login.html'),/admin-books/);
 const auth=read('static/js/admin-auth.js');assert.match(auth,/const reloadBooks = \(\) => books.reload\(\)/);assert.match(read('static/js/admin-sales.js'),/async function loadSaleBooks\(/);
});
