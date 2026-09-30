const {test, expect} = require('@playwright/test');

const pixel = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLttAAAAABJRU5ErkJggg==', 'base64');
test('cover import UI uses the shared session, uploads, follows progress and commits the selected candidate', async ({page}) => {
  await page.addInitScript(() => sessionStorage.setItem('defta.accessToken', 'browser-test-token'));
  await page.route('**/api/auth/me', route => route.fulfill({json:{id:'root-test',role:'SUPER_ADMIN_ROOT',libraryId:null,passwordChangeRequired:false}}));
  let jobStatus = 'REVIEW_REQUIRED';
  let uploaded = false;
  let accepted = false;
  let key = '';
  const history = Array.from({length: 15}, (_, index) => ({
    id:`older-${index}`, libraryId:'library-a', createdAt:'2026-09-27T10:00:00Z', totalFiles:1,
    jobs:[{id:`old-job-${index}`,status:'FAILED',contentType:'image/png'}]
  }));
  await page.route('**/api/admin/owners?**', route => route.fulfill({json:{results:[{username:'owner-a',library:{id:'library-a',name:'Librairie A',status:'ACTIVE'}}],total:1}}));
  await page.route('**/api/manage/cover-imports**', async route => {
    const request = route.request();
    const url = new URL(request.url());
    if (url.pathname === '/api/manage/cover-imports' && request.method() === 'GET') {
      return route.fulfill({json:{results: uploaded ? [{id:'batch-1',libraryId:'library-a',createdAt:'2026-09-28T10:00:00Z',totalFiles:2,jobs:[{id:'job-1',status:jobStatus,contentType:'image/png'},{id:'job-2',status:'FAILED',contentType:'image/png'}]}, ...history] : [],total:uploaded ? 16 : 0}});
    }
    if (url.pathname === '/api/manage/cover-imports' && request.method() === 'POST') {
      key = request.headers()['idempotency-key'];
      expect(request.postDataBuffer().includes(pixel)).toBe(true);
      uploaded = true;
      return route.fulfill({status:202,json:{id:'batch-1',status:'PENDING'}});
    }
    if (url.pathname === '/api/manage/cover-imports/job-1/source') {
      return route.fulfill({body:pixel,contentType:'image/png'});
    }
    if (url.pathname === '/api/manage/cover-imports/job-1' && request.method() === 'GET') {
      return route.fulfill({json:{id:'job-1',libraryId:'library-a',status:jobStatus,nsfwDecision:'SAFE',candidates:[{bookId:12,rank:1,title:'Le livre voulu',ftsScore:-1.2}]}});
    }
    if (url.pathname === '/api/manage/cover-imports/job-1/decision') {
      const body = request.postDataJSON();
      expect(body).toEqual({libraryId:'library-a',action:'ACCEPT',bookId:12});
      accepted = true;
      jobStatus = 'READY';
      return route.fulfill({status:202,json:{id:'job-1',status:'READY'}});
    }
    throw new Error(`Unexpected import request: ${request.method()} ${url.pathname}`);
  });

  await page.goto('/admin/cover-imports');
  await expect(page.getByRole('heading',{name:'Importer et revoir les couvertures'})).toBeVisible();
  await page.locator('#cover-files').setInputFiles({name:'couverture.png',mimeType:'image/png',buffer:pixel});
  await page.getByRole('button',{name:'Importer les couvertures'}).click();
  await expect(page.getByText('Rattachement à revoir')).toBeVisible();
  expect(key).toMatch(/^[0-9a-f-]{36}$/);
  await page.getByRole('button',{name:'Revoir'}).click();
  await expect(page.getByRole('heading',{name:'Revue de l’image'})).toBeInViewport();
  await expect(page.getByRole('heading',{name:'Revue de l’image'})).toBeFocused();
  const reviewPanel = page.getByRole('region',{name:'Revue de l’image'});
  const selectedRow = page.locator('.jobs > li').filter({has:reviewPanel});
  await expect(selectedRow).toHaveCount(1);
  await expect(selectedRow.locator(':scope > span > strong')).toHaveText('Image 1');
  await expect(selectedRow.locator('xpath=following-sibling::li[1]').locator('strong')).toHaveText('Image 2');
  await expect(page.getByAltText('Couverture importée à examiner')).toBeVisible();
  await page.getByRole('radio',{name:/Le livre voulu/}).check();
  await page.getByRole('button',{name:'Rattacher au livre choisi'}).click();
  await expect(page.getByText('Rattachement validé')).toBeVisible();
  expect(accepted).toBe(true);
});

test('cover import UI redirects an expired session to the existing login', async ({page}) => {
  await page.route('**/api/auth/refresh', route => route.fulfill({status:401,json:{error:'invalid_refresh_token'}}));
  await page.goto('/admin/cover-imports');
  await expect(page).toHaveURL(/\/login$/);
});

test('cover import UI reports a review loading failure inside the review panel', async ({page}) => {
  await page.addInitScript(() => sessionStorage.setItem('defta.accessToken', 'browser-test-token'));
  await page.route('**/api/auth/me', route => route.fulfill({json:{id:'owner-test',role:'OWNER_LIBRARY',libraryId:'library-a',passwordChangeRequired:false}}));
  await page.route('**/api/manage/cover-imports**', route => {
    const pathname = new URL(route.request().url()).pathname;
    if (pathname === '/api/manage/cover-imports') {
      return route.fulfill({json:{results:[{id:'batch-1',libraryId:'library-a',createdAt:'2026-09-28T10:00:00Z',totalFiles:1,jobs:[{id:'job-1',status:'REVIEW_REQUIRED',contentType:'image/png'}]}],total:1}});
    }
    return route.fulfill({status:503,json:{error:'cover_import_review_unavailable'}});
  });
  await page.goto('/admin/cover-imports');
  await page.getByRole('button',{name:'Revoir'}).click();
  const panel = page.getByRole('region',{name:'Revue de l’image'});
  await expect(panel.getByRole('alert')).toBeVisible();
  await expect(panel.getByText('Chargement de la revue…')).toHaveCount(0);
});


test('cover import UI explains an import quota instead of suggesting a service outage', async ({page}) => {
  await page.addInitScript(() => sessionStorage.setItem('defta.accessToken', 'browser-test-token'));
  await page.route('**/api/auth/me', route => route.fulfill({json:{id:'owner-test',role:'OWNER_LIBRARY',libraryId:'library-a',passwordChangeRequired:false}}));
  await page.route('**/api/manage/cover-imports**', route => {
    if (route.request().method() === 'POST') {
      return route.fulfill({status:429,json:{error:'cover_import_quota_exceeded',message:'Cover import quota exceeded'}});
    }
    return route.fulfill({json:{results:[],total:0}});
  });
  await page.goto('/admin/cover-imports');
  await page.locator('#cover-files').setInputFiles({name:'couverture.png',mimeType:'image/png',buffer:pixel});
  await page.getByRole('button',{name:'Importer les couvertures'}).click();
  await expect(page.getByRole('alert')).toContainText('Limite d’import atteinte');
  await expect(page.getByRole('alert')).toContainText('Terminez la revue');
  await expect(page.getByText('Service temporairement indisponible. Réessayez plus tard.')).toHaveCount(0);
});
