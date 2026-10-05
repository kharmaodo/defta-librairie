const {test, expect} = require('@playwright/test');
const {randomUUID} = require('node:crypto');
let query;

test.beforeAll(async ({request}) => {
  const suffix = randomUUID().slice(0, 8);
  query = `responsive${suffix}`;
  const login = await request.post('/api/auth/login', {data: {username: 'browser-root', password: 'Browser-Root-Only-2026!'}});
  expect(login.status()).toBe(200);
  const {accessToken} = await login.json();
  const headers = {Authorization: `Bearer ${accessToken}`};
  const owner = await request.post('/api/admin/owners', {headers, data: {
    username: `responsive-${suffix}`, email: `responsive-${suffix}@example.test`,
    password: 'Browser-Responsive-Only-2026!', library: {name: `Responsive ${suffix}`}
  }});
  expect(owner.status()).toBe(201);
  const {library} = await owner.json();
  // A real second page and long Arabic/Latin metadata, through existing APIs.
  for (let i = 0; i < 31; i++) {
    const created = await request.post('/api/manage/books', {headers, data: {
      libraryId: library.id, title: `${query} كتاب ${'عنوان'.repeat(25)} ${i}`,
      auteur: 'AuteurSansEspace'.repeat(8), price: 1000
    }});
    expect(created.status()).toBe(201);
  }
});

async function contained(page) {
  const layout = await page.evaluate(() => ({
    width: innerWidth, document: document.documentElement.scrollWidth,
    boxes: [...document.querySelectorAll('.book-card,.search-form,.pagination,.results-toolbar')]
      .filter(el => el.getClientRects().length)
      .map(el => ({left: el.getBoundingClientRect().left, right: el.getBoundingClientRect().right}))
  }));
  expect(layout.document).toBeLessThanOrEqual(layout.width + 1);
  for (const box of layout.boxes) {
    expect(box.left).toBeGreaterThanOrEqual(-1);
    expect(box.right).toBeLessThanOrEqual(layout.width + 1);
  }
}

for (const [width, height] of [[320,700],[390,844],[768,1024],[1024,768],[1440,900],[812,375]]) {
  test(`catalogue reflows and keeps search, views and pagination at ${width}x${height}`, async ({page}) => {
    await page.setViewportSize({width,height});
    await page.goto('/');
    await contained(page);
    await page.locator('#search-input').fill(query);
    await page.locator('#search-btn').click();
    await expect(page.locator('.book-card')).toHaveCount(30);
    await contained(page);
    const cover = page.locator('.card-cover').first();
    const box = await cover.boundingBox();
    expect(box.height).toBeGreaterThan(0);
    expect(box.height / box.width).toBeLessThan(1.5);
    await expect(page.locator('.public-book-cover').first()).toHaveAttribute('src', /book-cover-placeholder/);
    await page.locator('button[data-view=table]').click();
    await expect(page.locator('#books-table-view')).toBeVisible();
    await contained(page);
    await page.locator('#books-table-view').focus();
    await expect(page.locator('#books-table-view')).toBeFocused();
    await page.reload();
    await expect(page.locator('#books-table-view')).toBeVisible();
    await page.locator('button[data-view=card]').click();
    await page.locator('[rel=next]').click();
    await expect(page.locator('.book-card')).toHaveCount(1);
    await contained(page);
    await page.locator('[rel=prev]').click();
    await expect(page.locator('.book-card')).toHaveCount(30);
    await page.locator('.clear-search').click();
    await expect(page.locator('.welcome-panel')).toBeVisible();
    await contained(page);
  });
}
