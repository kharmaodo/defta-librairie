const {test}=require('node:test'),assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const read=n=>fs.readFileSync(path.join(__dirname,'..',n),'utf8');
const source=read('static/js/admin-tags.js');
function setup(api) {
 const node=()=>({children:[],handlers:{},dataset:{},elements:{},value:'',append(...items){this.children.push(...items);},replaceChildren(...items){this.children=items;},setAttribute(k,v){this[k]=v;},addEventListener(e,f){(this.handlers[e]||=[]).push(f);}});
 const nodes=new Map(),get=id=>{if(!nodes.has(id))nodes.set(id,node());return nodes.get(id);};
 const h={calls:[],audits:0,root:false,confirmed:true,get};
 get('#tag-form').elements={name:{value:''},libraryId:{value:''}};
 const window={confirm:()=>h.confirmed};vm.runInNewContext(source,{window,document:{querySelector:get,createElement:node,createTextNode:text=>({textContent:text})},URLSearchParams});
 h.errorBox={};h.module=window.DeftaTags.create({apiFetch:async(url,options)=>{h.calls.push({url,options});return api?api(url,options):{results:[{id:'tag',name:'Roman'}]};},isRoot:()=>h.root,reloadAudit:async()=>{h.audits++;},errorBox:h.errorBox,showError:(box,e)=>{box.textContent=e.message;box.hidden=false;}});
 h.event=async(id,event='click',tag='tag')=>{for(const fn of get(id).handlers[event]||[])await fn({preventDefault(){},currentTarget:get(id),target:{closest:()=>({dataset:{id:tag}})}});};return h;
}
test('load is inert',()=>{const window={};vm.runInNewContext(source,{window});assert.equal(typeof window.DeftaTags.create,'function');});
test('root requires selected library for listing and clears suggestions',async()=>{
 const h=setup();h.root=true;await h.module.reload();assert.equal(h.calls.length,0);assert.equal(h.get('#tag-suggestions').children.length,0);
 h.get('#tag-library').value='A & B';await h.module.reload();assert.equal(new URL(h.calls[0].url,'https://example.test').searchParams.get('libraryId'),'A & B');
});
test('owner listing omits supplied library',async()=>{
 const h=setup();h.get('#tag-library').value='foreign';await h.module.reload();assert.equal(new URL(h.calls[0].url,'https://example.test').searchParams.has('libraryId'),false);
});
test('render preserves text and accessible remove name',()=>{
 const h=setup();h.module.render({results:[{id:'tag',name:'<b>Roman</b>'}]});const chip=h.get('#tags-list').children[0];
 assert.equal(chip.children[0].textContent,'<b>Roman</b>');assert.equal(chip.children[1]['aria-label'],'Supprimer <b>Roman</b>');
 assert.equal(h.get('#tag-suggestions').children[0].value,'<b>Roman</b>');
});
test('create trims name, refreshes audit; init once',async()=>{
 const h=setup();h.module.init();h.module.init();h.get('#tag-form').elements.name.value=' Roman ';
 await h.event('#tag-form','submit');assert.equal(h.calls[0].options.method,'POST');assert.deepEqual(JSON.parse(h.calls[0].options.body),{name:'Roman'});assert.equal(h.audits,1);assert.equal(h.calls.length,2);
});
test('root creation requires library',async()=>{
 const h=setup();h.root=true;h.module.init();await h.event('#tag-form','submit');assert.equal(h.calls.length,0);
 h.get('#tag-form').elements.libraryId.value='library';await h.event('#tag-form','submit');assert.equal(JSON.parse(h.calls[0].options.body).libraryId,'library');
});
test('failed creation retains name',async()=>{
 const h=setup(async()=>{throw new Error('Refus');});h.module.init();h.get('#tag-form').elements.name.value='Roman';await h.event('#tag-form','submit');assert.equal(h.get('#tag-form').elements.name.value,'Roman');assert.equal(h.audits,0);assert.equal(h.errorBox.textContent,'Refus');
});
test('cancelled or stale removal does nothing; valid removal refreshes',async()=>{
 const h=setup();h.module.init();h.module.render({results:[{id:'tag',name:'Roman'}]});h.confirmed=false;await h.event('#tags-list');h.confirmed=true;await h.event('#tags-list','click','missing');assert.equal(h.calls.length,0);
 await h.event('#tags-list');assert.equal(h.calls[0].url,'/api/manage/tags/tag');assert.equal(h.calls[0].options.method,'DELETE');assert.equal(h.audits,1);
});
test('dashboard keeps books callbacks and excludes script from login',()=>{
 assert.match(read('static/js/admin-auth.js'),/const renderTags = payload => tags.render\(payload\)/);
 assert.doesNotMatch(read('templates/login.html'),/admin-tags/);
 const t=read('templates/admin.html');assert.ok(t.indexOf('/static/js/admin-tags.js')<t.indexOf('/static/js/admin-auth.js'));
});
