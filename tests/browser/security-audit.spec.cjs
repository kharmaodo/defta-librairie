const {test,expect}=require('@playwright/test');
const {randomUUID}=require('node:crypto');
const contract = require('../../static/openapi.json');
const publicPaths = new Set(['/api/health/live', '/api/health/ready', '/api/auth/login', '/api/auth/refresh', '/api/auth/logout', '/api/books', '/api/books/{id}/cover']);
const protectedRoutes = Object.entries(contract.paths).flatMap(([path, operations]) =>
 publicPaths.has(path) ? [] : Object.keys(operations)
  .filter(method => ['get','post','put','patch','delete','head'].includes(method))
  .map(method => [method.toUpperCase(), path.replace(/\{[^}]+\}/g, '1')])
);

test('every documented protected API rejects anonymous access',async({request})=>{
  for(const [method,path] of protectedRoutes){
    const response=await request.fetch(path,{method});
    expect(response.status(),`${method} ${path}`).toBe(401);
  }
});
test('strict CSP, no static listing and signed browser CSRF',async({page,request})=>{
  const response=await page.goto('/login');
  const policy=response.headers()['content-security-policy'];
  expect(policy).toContain("script-src 'self'");expect(policy).not.toContain('unsafe-inline');expect(policy).not.toContain('google');
  const cookie=(await page.context().cookies()).find(c=>c.name==='defta_csrf');expect(cookie).toBeTruthy();
  for(const path of ['/static/','/static/js/','/static/.env','/static/cover-imports/index.html']){expect((await request.get(path)).status()).toBe(404);}
  const denied=await page.request.post('/api/auth/login',{headers:{'X-Defta-Session':'cookie'},data:{username:'x',password:'x'}});expect(denied.status()).toBe(403);
  const allowed=await page.request.post('/api/auth/login',{headers:{'X-Defta-Session':'cookie','X-Defta-CSRF':cookie.value},data:{username:'x',password:'x'}});expect(allowed.status()).toBe(401);
});

test('public search escapes an HTML attribute payload and keeps local resources', async ({page,request}) => {
 const payload = '\"><img src=x onerror=window.deftaXSS=1>';
 const login = await request.post('/api/auth/login', {data:{username:'browser-root',password:'Browser-Root-Only-2026!'}});
 expect(login.status()).toBe(200);
 const {accessToken} = await login.json();
 const headers = {Authorization:`Bearer ${accessToken}`};
 const suffix = randomUUID().slice(0,8);
 const owner = await request.post('/api/admin/owners', {headers,data:{
  username:`security-${suffix}`,email:`security-${suffix}@example.test`,password:'Browser-Security-2026!',
  library:{name:`Security ${suffix}`,description:'Catalogue XSS regression'}
 }});
 expect(owner.status()).toBe(201);
 const account = await owner.json();
 const book = await request.post('/api/manage/books', {headers,data:{
  title:payload,auteur:'Distinct author',price:100,volume:0,status:'MODIFIED',libraryId:account.library.id
 }});
 expect(book.status()).toBe(201);

 const response = await page.goto('/?q=' + encodeURIComponent(payload));
 expect(response.status()).toBe(200);
 await expect(page.locator('#search-input')).toHaveValue(payload);
 await expect(page.locator('.book-card .title')).toHaveText(payload);
 await expect(page.locator('.book-card .author')).toHaveText('Distinct author');
 await expect(page.locator('body')).not.toContainText('MODIFIED');
 const publicResponse = await request.get('/api/books?q=' + encodeURIComponent(payload));
 expect(publicResponse.status()).toBe(200);
 const {results} = await publicResponse.json();
 expect(results).toHaveLength(1);
 expect(results[0].auteur).toBe('Distinct author');
 expect(results[0]).not.toHaveProperty('status');
 expect(results[0]).not.toHaveProperty('score');
 expect(await page.evaluate(() => window.deftaXSS)).toBeUndefined();
 await expect(page.locator('img[onerror]')).toHaveCount(0);
});
