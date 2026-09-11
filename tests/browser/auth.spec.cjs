const {test, expect} = require('@playwright/test');
const {randomUUID} = require('node:crypto');
const root = {username:'browser-root',password:'Browser-Root-Only-2026!'};
async function login(page,credentials=root) {
  await page.goto('/login');
  await page.locator('#login-form [name=username]').fill(credentials.username);
  await page.locator('#login-form [name=password]').fill(credentials.password);
  await page.locator('#login-form button[type=submit]').click();
}
async function owner(request) {
  const auth = await request.post('/api/auth/login',{data:root});
  expect(auth.status()).toBe(200);
  const {accessToken} = await auth.json();
  const username = `browser-${randomUUID().slice(0,8)}`;
  const credentials = {username,password:'Browser-Owner-Temp-2026!'};
  const created = await request.post('/api/admin/owners',{
    headers:{Authorization:`Bearer ${accessToken}`},
    data:{...credentials,email:`${username}@example.test`,library:{name:`Librairie ${username}`,description:'Recette isolée'}}
  });
  expect(created.status()).toBe(201);
  return credentials;
}
test('invalid login displays an accessible error',async({page})=>{
  await login(page,{username:'unknown-browser',password:'Wrong-Test-Password!'});
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.locator('#login-error')).toBeVisible();
  await expect(page.locator('#login-error')).toHaveAttribute('role','alert');
  await expect(page.locator('#login-error')).toContainText('Identifiant ou mot de passe incorrect');
});
test('root dashboard and logout invalidate cookie session',async({page})=>{
  await login(page);await expect(page).toHaveURL(/\/admin$/);
  await expect(page.locator('#role-badge')).toHaveText('SUPER ADMIN ROOT');
  await expect(page.locator('#owners-section')).toBeVisible();
  await page.locator('#logout-button').click();await expect(page).toHaveURL(/\/login$/);
  expect(await page.evaluate(()=>sessionStorage.getItem('defta.accessToken'))).toBeNull();
  const refresh = await page.request.post('/api/auth/refresh',{headers:{'X-Defta-Session':'cookie'}});
  expect(refresh.status()).toBe(400);
  expect((await refresh.json()).error).toBe('invalid_request');
  await page.goto('/admin');await expect(page).toHaveURL(/\/login$/);
});
test('mandatory password change, owner scope and forbidden root API',async({page,request})=>{
  const credentials=await owner(request);
  await login(page,credentials);await expect(page).toHaveURL(/\/admin$/);
  const dialog=page.locator('#password-dialog');await expect(dialog).toBeVisible();
  await page.keyboard.press('Escape');await expect(dialog).toBeVisible();
  const form=page.locator('#password-form');
  await form.locator('[name=currentPassword]').fill(credentials.password);
  const password='Browser-Owner-Changed-2026!';
  await form.locator('[name=newPassword]').fill(password);
  await form.locator('[name=confirmation]').fill(password);
  await form.locator('button[type=submit]').click();await expect(page.locator('#login-notice')).toContainText('Mot de passe modifié');
  await login(page,{username:credentials.username,password});
  await expect(page.locator('#role-badge')).toHaveText('PROPRIÉTAIRE');
  await expect(dialog).not.toBeVisible();await expect(page.locator('#owners-section')).not.toBeVisible();
  const status=await page.evaluate(async()=>{
    const response=await fetch('/api/admin/owners',{headers:{Authorization:`Bearer ${sessionStorage.getItem('defta.accessToken')}`}});
    return response.status;
  });
  expect(status).toBe(403);
});
test('parallel protected requests renew an invalid access token once',async({page})=>{
  await login(page);await expect(page.locator('#owners-page-label')).toContainText('Page');
  await expect(page.locator('#role-badge')).toHaveText('SUPER ADMIN ROOT');
  // Wait for the initial modules before deliberately invalidating their shared token.
  await page.waitForLoadState('networkidle');
  const result=await page.evaluate(async()=>{
    sessionStorage.setItem('defta.accessToken','expired-test-token');
    let renewals=0;
    const original=window.fetch;
    window.fetch=(url,options)=>{if(String(url).endsWith('/api/auth/refresh'))renewals++;return original(url,options);};
    try {
      const users=await Promise.all([window.DeftaHTTP.json('/api/auth/me'),window.DeftaHTTP.json('/api/auth/me')]);
      return {renewals,roles:users.map(user=>user.role)};
    } finally {window.fetch=original;}
  });
  expect(result).toEqual({renewals:1,roles:['SUPER_ADMIN_ROOT','SUPER_ADMIN_ROOT']});
});
