const {test, expect} = require('@playwright/test');
const {randomUUID} = require('node:crypto');

test('book editing persists categories with a primary category and scoped relational tags', async ({page, request}) => {
  const suffix = randomUUID().slice(0, 8);
  const login = await request.post('/api/auth/login', {data: {username: 'browser-root', password: 'Browser-Root-Only-2026!'}});
  expect(login.status()).toBe(200);
  const {accessToken} = await login.json();
  const headers = {Authorization: `Bearer ${accessToken}`};
  const ownerResponse = await request.post('/api/admin/owners', {headers, data: {
    username: `taxonomy-${suffix}`, email: `taxonomy-${suffix}@example.test`,
    password: 'Browser-Taxonomy-Only-2026!', library: {name: `Taxonomy ${suffix}`}
  }});
  expect(ownerResponse.status()).toBe(201);
  const {library} = await ownerResponse.json();
  const tagResponse = await request.post('/api/manage/tags', {headers, data: {libraryId: library.id, name: `Tag ${suffix}`}});
  expect(tagResponse.status()).toBe(201);
  const tag = await tagResponse.json();
  const categoriesResponse = await request.get('/api/manage/categories', {headers});
  expect(categoriesResponse.status()).toBe(200);
  const categoriesPayload = await categoriesResponse.json();
  const categories = Array.isArray(categoriesPayload) ? categoriesPayload : categoriesPayload.results;
  expect(categories.length).toBeGreaterThan(0);
  const category = categories[0];
  const title = `Livre taxonomie ${suffix}`;
  const createdResponse = await request.post('/api/manage/books', {headers, data: {libraryId: library.id, title, price: 1000}});
  expect(createdResponse.status()).toBe(201);
  const created = await createdResponse.json();

  await page.goto('/login');
  await page.locator('#login-form [name=username]').fill('browser-root');
  await page.locator('#login-form [name=password]').fill('Browser-Root-Only-2026!');
  await page.locator('#login-form button[type=submit]').click();
  await expect(page).toHaveURL(/\/admin$/);
  await page.locator('#book-search-form [name=q]').fill(title);
  await page.locator('#book-search-form button[type=submit]').click();
  const row = page.locator('#books-body tr').filter({hasText: title});
  await expect(row).toHaveCount(1);
  await row.getByRole('button', {name: 'Modifier'}).click();
  const form = page.locator('#book-form');
  await expect(form.locator(`[name=categoryIds] option[value="${category.id}"]`)).toHaveCount(1);
  await expect(form.locator(`[name=tagIds] option[value="${tag.id}"]`)).toHaveCount(1);
  await form.locator('[name=categoryIds]').selectOption(String(category.id));
  await form.locator('[name=tagIds]').selectOption(tag.id);
  const [updatedResponse] = await Promise.all([
    page.waitForResponse(response => response.url().endsWith(`/api/manage/books/${created.id}`) && response.request().method() === 'PUT'),
    form.locator('button[type=submit]').click()
  ]);
  expect(updatedResponse.status()).toBe(200);
  const updated = await updatedResponse.json();
  expect(updated.categoryIds).toEqual([category.id]);
  expect(updated.primaryCategoryId).toBe(category.id);
  expect(updated.tagIds).toEqual([tag.id]);
  await expect(form).not.toBeVisible();
  await row.getByRole('button', {name: 'Modifier'}).click();
  await expect(form.locator('[name=categoryIds]')).toHaveValues([String(category.id)]);
  await expect(form.locator('[name=tagIds]')).toHaveValues([tag.id]);
  await form.locator('[name=categoryIds]').selectOption([]);
  await form.locator('[name=tagIds]').selectOption([]);
  const [clearedResponse] = await Promise.all([
    page.waitForResponse(response => response.url().endsWith(`/api/manage/books/${created.id}`) && response.request().method() === 'PUT'),
    form.locator('button[type=submit]').click()
  ]);
  expect(clearedResponse.status()).toBe(200);
  const cleared = await clearedResponse.json();
  expect(cleared.categoryIds || []).toEqual([]);
  expect(cleared.primaryCategoryId).toBeNull();
  expect(cleared.tagIds || []).toEqual([]);
});
