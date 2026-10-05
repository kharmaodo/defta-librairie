const {test, expect} = require('@playwright/test');
const pixel = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLttAAAAABJRU5ErkJggg==', 'base64');
const longTitle = `عنوان ${'LivreSansEspace'.repeat(18)}`;
async function fits(page, width) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
}
for (const [width,height,role] of [[320,700,'OWNER_LIBRARY'],[390,844,'SUPER_ADMIN_ROOT'],[768,1024,'OWNER_LIBRARY'],[812,375,'SUPER_ADMIN_ROOT']]) {
  test(`cover import UI reflows through upload and manual review at ${width}x${height}`, async ({page}) => {
    let uploaded=false, accepted=false, dismissed=false, status='REVIEW_REQUIRED';
    const scope = role === 'SUPER_ADMIN_ROOT' ? 'library-a' : '';
    await page.addInitScript(() => sessionStorage.setItem('defta.accessToken','browser-test-token'));
    await page.route('**/api/auth/me', route => route.fulfill({json:{id:'responsive-user',role,libraryId:role === 'OWNER_LIBRARY' ? 'library-a' : null,passwordChangeRequired:false}}));
    await page.route('**/api/admin/owners?**', route => route.fulfill({json:{results:[{username:'owner',library:{id:'library-a',name:longTitle,status:'ACTIVE'}}],total:1}}));
    await page.route('**/api/manage/books/22/cover?**', route => route.fulfill({body:pixel,contentType:'image/png'}));
    await page.route('**/api/manage/cover-imports**', route => {
      const request=route.request(), path=new URL(request.url()).pathname;
      if (path === '/api/manage/cover-imports' && request.method()==='POST') {
        expect(request.headers()['idempotency-key']).toMatch(/^[0-9a-f-]{36}$/);
        expect(request.postDataBuffer().includes(pixel)).toBe(true);
        uploaded=true; return route.fulfill({status:202,json:{id:'batch',status:'PENDING'}});
      }
      if (path === '/api/manage/cover-imports') return route.fulfill({json:{results:uploaded ? [{id:'batch',libraryId:'library-a',createdAt:'2026-10-05T10:00:00Z',totalFiles:2,jobs:[{id:'job',status,contentType:'image/png'},{id:'failed',status:'FAILED',contentType:'image/png'}]}] : []}});
      if (path.endsWith('/source')) return route.fulfill({body:pixel,contentType:'image/png'});
      if (path.endsWith('/candidate-search')) {
        expect(request.postDataJSON()).toEqual({libraryId:scope,query:'Guide'});
        return route.fulfill({json:{results:[...(!dismissed ? [{bookId:21,title:'Proposition incorrecte',author:longTitle}] : []),{bookId:22,title:longTitle,author:longTitle,hasActiveCover:true},{bookId:23,title:'Autre proposition',author:'Auteur'}]}});
      }
      if (path.endsWith('/candidate-dismiss')) {expect(request.postDataJSON()).toEqual({libraryId:scope,bookId:21}); dismissed=true; return route.fulfill({json:{status:'DISMISSED'}});}
      if (path.endsWith('/decision')) {expect(request.postDataJSON()).toEqual({libraryId:scope,action:'ACCEPT',bookId:22}); accepted=true; status='READY'; return route.fulfill({status:202,json:{id:'job',status}});}
      return route.fulfill({json:{id:'job',libraryId:'library-a',status,nsfwDecision:'SAFE',candidates:[]}});
    });
    await page.setViewportSize({width,height});
    await page.goto('/admin/cover-imports');
    await expect(page.getByRole('heading',{name:'Importer et revoir les couvertures'})).toBeVisible();
    await fits(page,width);
    await page.locator('#cover-files').setInputFiles({name:'couverture.png',mimeType:'image/png',buffer:pixel});
    const upload=page.getByRole('button',{name:'Importer les couvertures'});
    expect((await upload.boundingBox()).height).toBeGreaterThanOrEqual(44);
    await upload.click();
    await expect(page.getByText('Rattachement à revoir')).toBeVisible();
    await page.getByRole('button',{name:'Revoir',exact:true}).click();
    const panel=page.getByRole('region',{name:'Revue de l’image'});
    await expect(panel.getByRole('heading',{name:'Revue de l’image'})).toBeFocused();
    await expect(panel.getByAltText('Couverture importée à examiner')).toBeVisible();
    await panel.getByLabel('Rechercher un livre par titre, auteur ou ISBN').fill('Guide');
    await panel.getByRole('button',{name:'Rechercher',exact:true}).click();
    const results=panel.getByRole('group',{name:'Résultats de recherche manuelle'});
    await expect(results.locator('.candidate')).toHaveCount(3);
    await fits(page,width);
    await results.locator('.candidate').filter({hasText:'Proposition incorrecte'}).getByRole('button',{name:'Rejeter la solution'}).click();
    await expect(results.locator('.candidate')).toHaveCount(2);
    const resolve=results.locator('.candidate').filter({hasText:longTitle}).getByRole('button',{name:'Résoudre',exact:true});
    expect((await resolve.boundingBox()).height).toBeGreaterThanOrEqual(44);
    await resolve.click();
    const dialog=page.getByRole('dialog');
    await expect(dialog).toContainText('Sa couverture actuelle sera remplacée');
    const box=await dialog.boundingBox();
    expect(box.x).toBeGreaterThanOrEqual(0); expect(box.y).toBeGreaterThanOrEqual(0);
    expect(box.x+box.width).toBeLessThanOrEqual(width); expect(box.y+box.height).toBeLessThanOrEqual(height);
    expect(await dialog.evaluate(el => el.scrollWidth)).toBeLessThanOrEqual(await dialog.evaluate(el => el.clientWidth));
    const cancel=dialog.getByRole('button',{name:'Annuler',exact:true});
    await cancel.focus(); await page.keyboard.press('Shift+Tab');
    await expect(dialog.getByRole('button',{name:'Confirmer',exact:true})).toBeFocused();
    await page.keyboard.press('Tab'); await expect(cancel).toBeFocused();
    await cancel.click(); expect(accepted).toBe(false);
    await resolve.click(); await page.keyboard.press('Escape');
    await expect(dialog).not.toBeVisible(); expect(accepted).toBe(false);
    await resolve.click(); const confirm=dialog.getByRole('button',{name:'Confirmer',exact:true});
    await confirm.scrollIntoViewIfNeeded(); await expect(confirm).toBeInViewport();
    await confirm.click(); await expect(page.getByText('Rattachement validé',{exact:true})).toBeVisible();
    expect(accepted).toBe(true); await fits(page,width);
  });
}

test('cover import UI root quarantine review keeps moderation actions usable on mobile', async ({page}) => {
  let approved=false;
  await page.setViewportSize({width:390,height:844});
  await page.addInitScript(() => sessionStorage.setItem('defta.accessToken','browser-test-token'));
  await page.route('**/api/auth/me',route=>route.fulfill({json:{id:'root',role:'SUPER_ADMIN_ROOT',libraryId:null,passwordChangeRequired:false}}));
  await page.route('**/api/admin/owners?**',route=>route.fulfill({json:{results:[{username:'owner',library:{id:'library-a',name:'Librairie',status:'ACTIVE'}}],total:1}}));
  await page.route('**/api/manage/cover-imports**',route=>{
    const path=new URL(route.request().url()).pathname;
    if(path.endsWith('/source')) return route.fulfill({body:pixel,contentType:'image/png'});
    if(path.endsWith('/quarantine-decision')) {expect(route.request().postDataJSON()).toEqual({libraryId:'library-a',decision:'APPROVE'});approved=true;return route.fulfill({json:{status:'OCR_PENDING'}});}
    if(path==='/api/manage/cover-imports') return route.fulfill({json:{results:[{id:'batch',libraryId:'library-a',createdAt:'2026-10-05T10:00:00Z',totalFiles:1,jobs:[{id:'quarantined',status:approved?'OCR_PENDING':'QUARANTINED',contentType:'image/png'}]}]}});
    return route.fulfill({json:{id:'quarantined',libraryId:'library-a',status:'QUARANTINED',candidates:[]}});
  });
  await page.goto('/admin/cover-imports'); await page.getByRole('button',{name:'Revoir',exact:true}).click();
  const approve=page.getByRole('button',{name:'Autoriser l’OCR'});
  await expect(approve).toBeVisible(); expect((await approve.boundingBox()).height).toBeGreaterThanOrEqual(44);
  await fits(page,390); await approve.click(); await expect(page.getByText('OCR en attente',{exact:true})).toBeVisible();expect(approved).toBe(true);
});
