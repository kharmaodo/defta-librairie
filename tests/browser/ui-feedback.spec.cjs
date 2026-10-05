const {test,expect}=require('@playwright/test');
const {readFileSync}=require('node:fs');const {resolve}=require('node:path');const {randomUUID,randomBytes}=require('node:crypto');
const fixture=readFileSync(resolve(__dirname,'../../scripts/browser-test-server.cjs'),'utf8');
const value=key=>fixture.match(new RegExp(`${key}:'([^']+)'`))[1];
let libraryId, customerName;
test.beforeAll(async({request})=>{
  customerName=`Feedback ${randomUUID().slice(0,8)}`;
  const login=await request.post('/api/auth/login',{data:{username:value('DEFTA_ROOT_USERNAME'),password:value('DEFTA_ROOT_PASSWORD')}});expect(login.status()).toBe(200);
  const headers={Authorization:`Bearer ${(await login.json()).accessToken}`},suffix=randomUUID().slice(0,8);
  const owner=await request.post('/api/admin/owners',{headers,data:{username:`feedback-${suffix}`,email:`feedback-${suffix}@example.test`,password:`Aa!${randomBytes(24).toString('hex')}`,library:{name:`Feedback ${suffix}`}}});expect(owner.status()).toBe(201);libraryId=(await owner.json()).library.id;
});
async function login(page){await page.goto('/login');await page.locator('[name=username]').fill(value('DEFTA_ROOT_USERNAME'));await page.locator('[name=password]').fill(value('DEFTA_ROOT_PASSWORD'));await page.locator('#login-form button[type=submit]').click();await expect(page.locator('#role-badge')).toHaveText('SUPER ADMIN ROOT');}
test('public catalogue announces a pending request without blocking the mobile layout',async({page})=>{
  let release;const gate=new Promise(resolve=>{release=resolve;});
  await page.setViewportSize({width:320,height:700});
  await page.route('**/?q=feedback*',async route=>{const response=await route.fetch();await gate;await route.fulfill({response});});
  await page.goto('/');await page.locator('#search-input').fill('feedback');await page.locator('#search-btn').click();
  const status=page.locator('.ui-feedback');await expect(status).toBeVisible();await expect(status).toHaveAttribute('role','status');await expect(page.locator('main')).toHaveAttribute('aria-busy','true');
  const box=await status.boundingBox();expect(box.x).toBeGreaterThanOrEqual(0);expect(box.x+box.width).toBeLessThanOrEqual(320);
  expect(await status.evaluate(el=>getComputedStyle(el).pointerEvents)).toBe('none');
  release();await expect(page.locator('.empty-state')).toBeVisible();await expect(status).not.toBeVisible();await expect(page.locator('main')).not.toHaveAttribute('aria-busy','true');
});
test('admin form blocks repeat submission, preserves fields after failure and permits a deliberate retry',async({page})=>{
  let release,calls=0;const gate=new Promise(resolve=>{release=resolve;});
  await login(page);
  await page.route('**/api/manage/customers',async route=>{
    if(route.request().method()!=='POST')return route.continue();
    calls++;if(calls===1){await gate;return route.fulfill({status:503,json:{error:'unavailable',message:'private diagnostic'}});}return route.continue();
  });
  await page.locator('#add-customer-button').click();const form=page.locator('#customer-form');
  await form.locator('[name=libraryId]').selectOption(libraryId);await form.locator('[name=name]').fill(customerName);
  const save=form.locator('button[type=submit]');await save.click();await expect(save).toBeDisabled();await expect(form).toHaveAttribute('aria-busy','true');
  await expect(form.getByRole('status')).toHaveText('Enregistrement en cours…');
  await form.evaluate(el=>el.dispatchEvent(new Event('submit',{bubbles:true,cancelable:true})));expect(calls).toBe(1);
  release();await expect(form.getByRole('alert')).toContainText('Service temporairement indisponible');await expect(form.getByRole('alert')).not.toContainText('private diagnostic');
  await expect(save).toBeEnabled();await expect(form).not.toHaveAttribute('aria-busy','true');await expect(form.locator('[name=name]')).toHaveValue(customerName);
  await save.click();await expect(page.locator('#customer-dialog')).not.toBeVisible();await expect(page.locator('#customers-body')).toContainText(customerName);expect(calls).toBe(2);
});
test('shared read feedback ends only after concurrent coalesced requests settle',async({page})=>{
  let one,two,calls=0;const first=new Promise(resolve=>{one=resolve;}),second=new Promise(resolve=>{two=resolve;});
  await login(page);await expect(page.locator('.ui-feedback')).not.toBeVisible();
  await page.route('**/api/manage/books?feedback=*',async route=>{calls++;await(new URL(route.request().url()).searchParams.get('feedback')==='one'?first:second);return route.fulfill({json:{results:[]}});});
  await page.evaluate(()=>{window.feedbackReads=[window.DeftaHTTP.json('/api/manage/books?feedback=one'),window.DeftaHTTP.json('/api/manage/books?feedback=one'),window.DeftaHTTP.json('/api/manage/books?feedback=two')];});
  const status=page.locator('.ui-feedback');await expect(status).toBeVisible();expect(calls).toBe(2);
  one();await page.evaluate(()=>window.feedbackReads[0]);await expect(status).toBeVisible();
  two();await page.evaluate(()=>Promise.all(window.feedbackReads));await expect(status).not.toBeVisible();
});
