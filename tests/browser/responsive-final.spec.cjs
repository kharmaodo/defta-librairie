const {test,expect}=require('@playwright/test');
const {readFileSync}=require('node:fs');const {resolve}=require('node:path');const {randomUUID,randomBytes}=require('node:crypto');
const fixture=readFileSync(resolve(__dirname,'../../scripts/browser-test-server.cjs'),'utf8');
const value=key=>fixture.match(new RegExp(`${key}:'([^']+)'`))[1];
let query;
test.beforeAll(async({request})=>{
  query=`final${randomUUID().slice(0,8)}`;
  const login=await request.post('/api/auth/login',{data:{username:value('DEFTA_ROOT_USERNAME'),password:value('DEFTA_ROOT_PASSWORD')}});expect(login.status()).toBe(200);
  const headers={Authorization:`Bearer ${(await login.json()).accessToken}`};
  const owner=await request.post('/api/admin/owners',{headers,data:{username:query,email:`${query}@example.test`,password:`Aa!${randomBytes(24).toString('hex')}`,library:{name:query}}});expect(owner.status()).toBe(201);
  const {library}=await owner.json();
  for(let i=0;i<31;i++)expect((await request.post('/api/manage/books',{headers,data:{libraryId:library.id,title:`${query} كتاب ${i}`,price:1000}})).status()).toBe(201);
});
async function contained(page){expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual((await page.viewportSize()).width+1);}
async function login(page){await page.goto('/login');await page.locator('[name=username]').fill(value('DEFTA_ROOT_USERNAME'));await page.locator('[name=password]').fill(value('DEFTA_ROOT_PASSWORD'));await page.locator('#login-form button[type=submit]').click();await expect(page.locator('#role-badge')).toHaveText('SUPER ADMIN ROOT');}
test.describe('native public catalogue fallback',()=>{
  test.use({javaScriptEnabled:false});
  test('search and pagination stay functional without JavaScript',async({page})=>{
    await page.setViewportSize({width:390,height:844});await page.goto('/');
    await page.locator('#search-input').fill(query);await page.locator('#search-btn').click();await expect(page.locator('.book-card')).toHaveCount(30);
    await page.locator('[rel=next]').click();await expect(page).toHaveURL(/page=2/);await expect(page.locator('.book-card')).toHaveCount(1);
    await page.locator('[rel=prev]').click();await expect(page.locator('.book-card')).toHaveCount(30);
    await page.locator('.clear-search').click();await expect(page.locator('.welcome-panel')).toBeVisible();
  });
});
for(const width of [320,1440]){
  test(`public and admin reflow with 200 percent root text at ${width}px`,async({page})=>{
    await page.setViewportSize({width,height:900});await page.emulateMedia({reducedMotion:'reduce'});
    await page.goto(`/?q=${query}`);await expect(page.locator('.book-card')).toHaveCount(30);
    await page.evaluate(()=>{document.documentElement.style.fontSize='32px';});
    expect(await page.evaluate(()=>getComputedStyle(document.documentElement).fontSize)).toBe('32px');await contained(page);
    await login(page);await page.evaluate(()=>{document.documentElement.style.fontSize='32px';});await contained(page);
    await page.locator('#add-book-button').click();const dialog=page.locator('#book-dialog');await expect(dialog).toBeVisible();
    const geometry=await dialog.evaluate(el=>({scroll:el.scrollWidth,client:el.clientWidth,right:el.getBoundingClientRect().right}));
    expect(geometry.scroll).toBeLessThanOrEqual(geometry.client+1);expect(geometry.right).toBeLessThanOrEqual(width+1);
    await page.keyboard.press('Escape');await expect(dialog).not.toBeVisible();
  });
}
test('public and admin remain contained on a 1920px desktop',async({page})=>{
  await page.setViewportSize({width:1920,height:1080});await page.goto(`/?q=${query}`);await expect(page.locator('.book-card')).toHaveCount(30);await contained(page);
  await login(page);await expect(page.locator('[data-dashboard-menu-toggle]')).toBeHidden();await expect(page.locator('[data-dashboard-nav]')).toBeVisible();await contained(page);
});
