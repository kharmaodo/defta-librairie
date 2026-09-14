const {test}=require('node:test'),assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const source=fs.readFileSync(path.join(__dirname,'../static/js/admin-http.js'),'utf8');
const response=(status,body)=>new Response(status===204?null:JSON.stringify(body),{status,headers:{'Content-Type':'application/json'}});
function setup(fetch,token='old'){
 const values=new Map(token?[['defta.accessToken',token]]:[]),window={location:{origin:'https://example.test'}};
 vm.runInNewContext(source,{window,URL,Headers,SyntaxError,fetch,sessionStorage:{getItem:k=>values.get(k),setItem:(k,v)=>values.set(k,v),removeItem:k=>values.delete(k)}});
 window.DeftaHTTP.enableSessionRefresh();return {api:window.DeftaHTTP,values};
}
test('parallel expired requests share refresh and retry with new token',async()=>{
 let refreshes=0;const h=setup(async(url,options)=>{
  if(url.endsWith('/refresh')){refreshes++;return response(200,{accessToken:'new'});}
  return options.headers.get('Authorization')==='Bearer old'?response(401,{error:'unauthorized'}):response(200,{ok:true});
 });
 await Promise.all([h.api.json('/api/a'),h.api.json('/api/b')]);assert.equal(refreshes,1);assert.equal(h.values.get('defta.accessToken'),'new');
});
test('second 401 is final, write body preserved',async()=>{
 let refreshes=0,writes=0;const h=setup(async(url,options)=>{
  if(url.endsWith('/refresh')){refreshes++;return response(200,{accessToken:'new'});}
  writes++;assert.equal(options.body,'{"version":3}');assert.equal(options.method,'POST');return response(401,{error:'unauthorized'});
 });await assert.rejects(h.api.json('/api/sales',{method:'POST',body:'{"version":3}'}),e=>e.status===401);assert.equal(refreshes,1);assert.equal(writes,2);
});
test('missing access token renews before request',async()=>{
 const calls=[];const h=setup(async url=>{calls.push(url);return response(200,url.endsWith('/refresh')?{accessToken:'new'}:{ok:true});},'');
 await h.api.json('/api/me');assert.ok(calls[0].endsWith('/refresh'));assert.equal(calls.length,2);
});
test('logout invalidates pending renewal',async()=>{
 let finish;const h=setup(()=>new Promise(resolve=>{finish=resolve;}));const pending=h.api.refreshSession();h.api.clearSession();finish(response(200,{accessToken:'new'}));
 await assert.rejects(pending,e=>e.status===401);assert.equal(h.values.has('defta.accessToken'),false);
});
test('403 and network failures never renew or replay',async()=>{
 for(const status of [403,409,500]){let calls=0;const h=setup(async()=>{calls++;return response(status,{error:'forbidden'});});await assert.rejects(h.api.json('/api/write',{method:'POST'}));assert.equal(calls,1);}
 let calls=0;const h=setup(async()=>{calls++;throw new TypeError('network');});await assert.rejects(h.api.json('/api/write',{method:'POST'}));assert.equal(calls,1);
});
test('auth endpoints use cookie header without bearer and no automatic refresh',async()=>{
 let calls=0;const h=setup(async(url,options)=>{calls++;assert.equal(options.headers['X-Defta-Session'],'cookie');assert.equal(options.headers.Authorization,undefined);return response(401,{error:'invalid_credentials'});});
 await assert.rejects(h.api.authJSON('login',{username:'test',password:'test'}),/Identifiant/);assert.equal(calls,1);await assert.rejects(h.api.authJSON('../other'));assert.equal(calls,1);
});
test('abort while renewing prevents replay',async()=>{
 let finish,calls=0;const controller=new AbortController();const h=setup(async url=>{calls++;if(url.endsWith('/refresh'))return new Promise(resolve=>{finish=resolve;});return response(401,{error:'unauthorized'});});
 const pending=h.api.json('/api/a',{signal:controller.signal});while(!finish)await new Promise(resolve=>setImmediate(resolve));controller.abort();finish(response(200,{accessToken:'new'}));await assert.rejects(pending,e=>e.name==='AbortError');assert.equal(calls,2);
});
test('malformed refresh does not overwrite access token',async()=>{
 const h=setup(async()=>response(200,{}));await assert.rejects(h.api.refreshSession(),/invalide/);assert.equal(h.values.get('defta.accessToken'),'old');
});
