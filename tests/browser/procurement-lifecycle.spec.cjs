const {test, expect} = require('@playwright/test');
const {randomUUID} = require('node:crypto');

const root = {username: 'browser-root', password: 'Browser-Root-Only-2026!'};

async function createLibrary(request) {
  const login = await request.post('/api/auth/login', {data: root});
  expect(login.status()).toBe(200);
  const {accessToken} = await login.json();
  const suffix = randomUUID().slice(0, 8);
  const response = await request.post('/api/admin/owners', {
    headers: {Authorization: `Bearer ${accessToken}`},
    data: {
      username: `procurement-${suffix}`,
      email: `procurement-${suffix}@example.test`,
      password: 'Browser-Procurement-Temp-2026!',
      library: {name: `Approvisionnement ${suffix}`, description: 'Test navigateur isolé'}
    }
  });
  expect(response.status()).toBe(201);
  return (await response.json()).library;
}

async function loginRoot(page) {
  await page.goto('/login');
  await page.locator('#login-form [name=username]').fill(root.username);
  await page.locator('#login-form [name=password]').fill(root.password);
  await page.locator('#login-form button[type=submit]').click();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.locator('#role-badge')).toHaveText('SUPER ADMIN ROOT');
}

async function createPurchase(page, {libraryID, supplierName, title, quantity, unitCost}) {
  await expect(page.locator('#suppliers-body tr').filter({hasText: supplierName})).toHaveCount(1);
  await page.locator('#add-purchase-button').click();
  const form = page.locator('#purchase-form');
  await expect(form).toBeVisible();
  await form.locator('[name=libraryId]').selectOption(libraryID);
  await form.locator('[name=supplierId]').selectOption({label: supplierName});
  const line = page.locator('#purchase-lines .purchase-line');
  await expect(line).toHaveCount(1);
  const option = line.locator('[name=bookId] option').filter({hasText: title});
  await expect(option).toHaveCount(1);
  await line.locator('[name=bookId]').selectOption(await option.getAttribute('value'));
  await line.locator('[name=quantity]').fill(String(quantity));
  await line.locator('[name=unitCost]').fill(String(unitCost));
  await form.locator('button[type=submit]').click();
  await expect(form).not.toBeVisible();

  await page.locator('#purchase-filters [name=libraryId]').selectOption(libraryID);
  await page.locator('#purchase-filters [name=status]').selectOption('DRAFT');
  await page.locator('#purchase-filters button[type=submit]').click();
  const row = page.locator('#purchases-body tr').filter({hasText: 'DRAFT'});
  await expect(row).toHaveCount(1);
  page.once('dialog', dialog => dialog.accept());
  await Promise.all([
    page.waitForNavigation(),
    row.getByRole('button', {name: 'Réceptionner'}).click()
  ]);
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.locator('#role-badge')).toHaveText('SUPER ADMIN ROOT');
}

test('two receipts update stock and weighted average cost', async ({page, request}) => {
  const library = await createLibrary(request);
  const suffix = randomUUID().slice(0, 8);
  const title = `Livre CMP ${suffix}`;
  const supplierName = `Fournisseur CMP ${suffix}`;
  await loginRoot(page);

  await page.locator('#add-book-button').click();
  const bookForm = page.locator('#book-form');
  await bookForm.locator('[name=libraryId]').selectOption(library.id);
  await bookForm.locator('[name=title]').fill(title);
  await bookForm.locator('[name=price]').fill('3000');
  await bookForm.locator('[name=volume]').fill('0');
  await bookForm.locator('button[type=submit]').click();
  await expect(bookForm).not.toBeVisible();

  await page.locator('#add-supplier-button').click();
  const supplierForm = page.locator('#supplier-form');
  await supplierForm.locator('[name=libraryId]').selectOption(library.id);
  await supplierForm.locator('[name=name]').fill(supplierName);
  await supplierForm.locator('[name=contactName]').fill('Contact navigateur');
  await supplierForm.locator('button[type=submit]').click();
  await expect(supplierForm).not.toBeVisible();
  await expect(page.locator('#suppliers-body tr').filter({hasText: supplierName})).toHaveCount(1);

  await createPurchase(page, {libraryID: library.id, supplierName, title, quantity: 4, unitCost: 1000});
  await createPurchase(page, {libraryID: library.id, supplierName, title, quantity: 6, unitCost: 2000});

  await page.locator('#inventory-filters [name=libraryId]').selectOption(library.id);
  await page.locator('#inventory-filters button[type=submit]').click();
  const inventory = page.locator('#inventory-body tr').filter({hasText: title});
  await expect(inventory).toHaveCount(1);
  await expect(inventory.locator('td').nth(1)).toHaveText('10');

  const customer = `Client CMP ${suffix}`;
  await page.locator('#add-sale-button').click();
  const saleForm = page.locator('#sale-form');
  await saleForm.locator('[name=libraryId]').selectOption(library.id);
  await saleForm.locator('[name=customerName]').fill(customer);
  const saleLine = page.locator('#sale-lines .sale-line');
  const bookOption = saleLine.locator('[name=bookId] option').filter({hasText: title});
  await expect(bookOption).toHaveCount(1);
  await saleLine.locator('[name=bookId]').selectOption(await bookOption.getAttribute('value'));
  await saleLine.locator('[name=quantity]').fill('1');
  await saleForm.locator('button[type=submit]').click();
  await expect(saleForm).not.toBeVisible();
  await page.waitForLoadState('networkidle');

  await page.locator('#sale-filters [name=libraryId]').selectOption(library.id);
  await page.locator('#sale-filters button[type=submit]').click();
  const sale = page.locator('#sales-body tr').filter({hasText: customer});
  await expect(sale).toHaveCount(1);
  page.once('dialog', dialog => dialog.accept());
  await sale.getByRole('button', {name: 'Confirmer'}).click();
  await expect(page.locator('#sales-body tr').filter({hasText: customer})).toContainText('Confirmée');

  const statistics = await page.evaluate(async libraryID => {
    const query = new URLSearchParams({
      libraryId: libraryID,
      from: '2000-01-01T00:00:00Z',
      to: '2100-01-01T00:00:00Z'
    });
    return window.DeftaHTTP.json(`/api/manage/statistics?${query}`);
  }, library.id);
  expect(statistics.receivedPurchases).toBe(16000);
  expect(statistics.knownCost).toBe(1600);
  expect(statistics.netMargin).toBe(1400);
  expect(statistics.unknownCostEvents).toBe(0);
});
