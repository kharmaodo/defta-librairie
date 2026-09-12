const {test, expect} = require('@playwright/test');
const {randomUUID} = require('node:crypto');

const root = {username: 'browser-root', password: 'Browser-Root-Only-2026!'};

async function createLibrary(request) {
  const login = await request.post('/api/auth/login', {data: root});
  expect(login.status()).toBe(200);
  const {accessToken} = await login.json();
  const suffix = randomUUID().slice(0, 8);
  const created = await request.post('/api/admin/owners', {
    headers: {Authorization: `Bearer ${accessToken}`},
    data: {
      username: `commercial-${suffix}`,
      email: `commercial-${suffix}@example.test`,
      password: 'Browser-Commercial-Temp-2026!',
      library: {name: `Commercial ${suffix}`, description: 'Cycle navigateur isolé'}
    }
  });
  expect(created.status()).toBe(201);
  const account = await created.json();
  return {id: account.library.id, name: account.library.name};
}

async function loginRoot(page) {
  await page.goto('/login');
  await page.locator('#login-form [name=username]').fill(root.username);
  await page.locator('#login-form [name=password]').fill(root.password);
  await page.locator('#login-form button[type=submit]').click();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.locator('#role-badge')).toHaveText('SUPER ADMIN ROOT');
}

async function filterInventory(page, libraryID) {
  await page.locator('#inventory-filters [name=libraryId]').selectOption(libraryID);
  await page.locator('#inventory-filters button[type=submit]').click();
}

async function filterSales(page, libraryID) {
  await page.locator('#sale-filters [name=libraryId]').selectOption(libraryID);
  await page.locator('#sale-filters button[type=submit]').click();
}

test('book, stock and confirmed/cancelled sale restore inventory', async ({page, request}) => {
  const library = await createLibrary(request);
  const title = `Cycle commercial ${randomUUID().slice(0, 8)}`;
  await loginRoot(page);

  await page.locator('#add-book-button').click();
  const bookForm = page.locator('#book-form');
  await expect(bookForm).toBeVisible();
  await bookForm.locator('[name=libraryId]').selectOption(library.id);
  await bookForm.locator('[name=title]').fill(title);
  await bookForm.locator('[name=auteur]').fill('Auteur navigateur');
  await bookForm.locator('[name=price]').fill('1000');
  await bookForm.locator('[name=volume]').fill('0');
  await bookForm.locator('button[type=submit]').click();
  await expect(bookForm).not.toBeVisible();
  await expect(page.locator('#books-body tr').filter({hasText: title})).toHaveCount(1);

  await filterInventory(page, library.id);
  let inventoryRow = page.locator('#inventory-body tr').filter({hasText: title});
  await expect(inventoryRow).toHaveCount(1);
  await expect(inventoryRow.locator('td').nth(1)).toHaveText('0');
  await inventoryRow.getByRole('button', {name: 'Mouvement'}).click();
  const inventoryForm = page.locator('#inventory-form');
  await inventoryForm.locator('[name=operation]').selectOption('ENTRY');
  await inventoryForm.locator('[name=quantity]').fill('7');
  await inventoryForm.locator('[name=reason]').fill('Stock initial du test navigateur');
  await inventoryForm.locator('button[type=submit]').click();
  await expect(inventoryForm).not.toBeVisible();
  inventoryRow = page.locator('#inventory-body tr').filter({hasText: title});
  await expect(inventoryRow.locator('td').nth(1)).toHaveText('7');

  await page.locator('#add-sale-button').click();
  const saleForm = page.locator('#sale-form');
  await expect(saleForm).toBeVisible();
  await saleForm.locator('[name=libraryId]').selectOption(library.id);
  const saleLine = page.locator('#sale-lines .sale-line');
  await expect(saleLine).toHaveCount(1);
  const bookOption = saleLine.locator('[name=bookId] option').filter({hasText: title});
  await expect(bookOption).toHaveCount(1);
  await saleLine.locator('[name=bookId]').selectOption(await bookOption.getAttribute('value'));
  await saleLine.locator('[name=quantity]').fill('2');
  await saleForm.locator('[name=customerName]').fill('Client navigateur');
  await saleForm.locator('button[type=submit]').click();
  await expect(saleForm).not.toBeVisible();

  await filterSales(page, library.id);
  let saleRow = page.locator('#sales-body tr').filter({hasText: 'Client navigateur'});
  await expect(saleRow).toHaveCount(1);
  await expect(saleRow).toContainText('Brouillon');

  page.once('dialog', dialog => dialog.accept());
  await saleRow.getByRole('button', {name: 'Confirmer'}).click();
  saleRow = page.locator('#sales-body tr').filter({hasText: 'Client navigateur'});
  await expect(saleRow).toContainText('Confirmée');
  inventoryRow = page.locator('#inventory-body tr').filter({hasText: title});
  await expect(inventoryRow.locator('td').nth(1)).toHaveText('5');

  page.once('dialog', dialog => dialog.accept());
  await saleRow.getByRole('button', {name: 'Annuler'}).click();
  saleRow = page.locator('#sales-body tr').filter({hasText: 'Client navigateur'});
  await expect(saleRow).toContainText('Annulée');
  inventoryRow = page.locator('#inventory-body tr').filter({hasText: title});
  await expect(inventoryRow.locator('td').nth(1)).toHaveText('7');
});
