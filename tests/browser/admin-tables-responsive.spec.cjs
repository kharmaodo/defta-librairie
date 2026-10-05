const {test, expect} = require('@playwright/test');
const {randomUUID} = require('node:crypto');
let title;
test.beforeAll(async ({request}) => {
  const suffix = randomUUID().slice(0,8);
  title = `Tableau-${suffix} عنوان ${'LongSansEspace'.repeat(12)}`;
  const login = await request.post('/api/auth/login',{data:{username:'browser-root',password:'Browser-Root-Only-2026!'}});
  expect(login.status()).toBe(200);
  const {accessToken} = await login.json();
  const headers = {Authorization:`Bearer ${accessToken}`};
  const created = await request.post('/api/admin/owners',{headers,data:{username:`tables-${suffix}`,email:`tables-${suffix}@example.test`,password:'Browser-Tables-Only-2026!',library:{name:`Tables ${suffix}`}}});
  expect(created.status()).toBe(201);
  const {library} = await created.json();
  const book = await request.post('/api/manage/books',{headers,data:{libraryId:library.id,title,auteur:'AuteurLong'.repeat(15),price:1000}});
  expect(book.status()).toBe(201);
});
for (const [width,height] of [[320,700],[390,844],[768,1024],[812,375]]) {
  test(`admin tables keep native columns and usable actions at ${width}x${height}`,async({page})=>{
    await page.setViewportSize({width,height});
    await page.goto('/login');
    await page.locator('[name=username]').fill('browser-root');
    await page.locator('[name=password]').fill('Browser-Root-Only-2026!');
    await page.locator('#login-form button[type=submit]').click();
    await expect(page).toHaveURL(/\/admin$/);
    const search = page.locator('#book-search-form');
    await search.locator('[name=q]').fill(title.split(' ')[0]);
    await search.locator('button[type=submit]').click();
    const row = page.locator('#books-body tr').filter({hasText:title});
    await expect(row).toHaveCount(1);
    const region = page.locator('#books-panel .table-wrap');
    await expect(region).toHaveAttribute('role','region');
    await expect(region).toHaveAttribute('tabindex','0');
    await expect(region).toHaveClass(/table-overflows/);
    await expect(region.locator('table')).toHaveClass(/table-with-actions/);
    await expect(region.locator('th')).toHaveCount(6);
    await expect(row.locator('td')).toHaveCount(6);
    await expect(page.locator('#books-panel .table-scroll-hint')).toBeVisible();
    await region.scrollIntoViewIfNeeded();
    const limits = await region.boundingBox();
    for (const name of ['Historique','Modifier','Supprimer']) {
      const button = row.getByRole('button',{name,exact:true});
      const box = await button.boundingBox();
      expect(box.height).toBeGreaterThanOrEqual(44);
      expect(box.x).toBeGreaterThanOrEqual(limits.x-1);
      expect(box.x+box.width).toBeLessThanOrEqual(limits.x+limits.width+1);
    }
    await region.focus();
    const before = await region.evaluate(el=>el.scrollLeft);
    await page.keyboard.press('ArrowRight');
    await expect.poll(()=>region.evaluate(el=>el.scrollLeft)).toBeGreaterThan(before);
    await row.getByRole('button',{name:'Modifier',exact:true}).click();
    await expect(page.locator('#book-dialog')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.locator('#book-dialog')).toBeHidden();
    await search.locator('[name=q]').fill(`absent-${randomUUID()}`);
    await search.locator('button[type=submit]').click();
    await expect(page.locator('#books-body .empty')).toContainText('Aucun livre');
    await expect(page.locator('#books-body .table-empty-message')).toBeVisible();
    expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(width+1);
  });
}
