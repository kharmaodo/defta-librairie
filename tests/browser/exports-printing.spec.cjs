const {test, expect} = require('@playwright/test');
const {randomUUID} = require('node:crypto');
const {readFile} = require('node:fs/promises');

const root = {username: 'browser-root', password: 'Browser-Root-Only-2026!'};

async function createLibrary(request) {
  const login = await request.post('/api/auth/login', {data: root});
  expect(login.status()).toBe(200);
  const {accessToken} = await login.json();
  const suffix = randomUUID().slice(0, 8);
  const response = await request.post('/api/admin/owners', {
    headers: {Authorization: `Bearer ${accessToken}`},
    data: {
      username: `exports-${suffix}`,
      email: `exports-${suffix}@example.test`,
      password: 'Browser-Exports-Temp-2026!',
      library: {name: `Exports ${suffix}`, description: 'Exports et impressions navigateur'}
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

async function downloadExport(page, libraryID, kind, needle) {
  const panel = page.locator('#csv-exports-panel');
  const form = panel.locator('form');
  await form.locator('[name=kind]').selectOption(kind);
  if (kind !== 'audit') await form.locator('[name=libraryId]').selectOption(libraryID);
  const [download] = await Promise.all([
    page.waitForEvent('download'),
    form.locator('button[type=submit]').click()
  ]);
  expect(download.suggestedFilename()).toBe(`${kind}.csv`);
  const path = await download.path();
  expect(path).not.toBeNull();
  const contents = await readFile(path, 'utf8');
  expect(contents).toContain(needle);
  await expect(panel.locator('[data-notice]')).toContainText('Téléchargement lancé');
}

test('CSV exports download scoped data and sale/purchase receipts print', async ({page, request}) => {
  const library = await createLibrary(request);
  const suffix = randomUUID().slice(0, 8);
  const title = `Livre export ${suffix}`;
  const supplier = `Fournisseur export ${suffix}`;
  const customer = `Client export ${suffix}`;
  await loginRoot(page);

  await page.locator('#add-book-button').click();
  const bookForm = page.locator('#book-form');
  await bookForm.locator('[name=libraryId]').selectOption(library.id);
  await bookForm.locator('[name=title]').fill(title);
  await bookForm.locator('[name=price]').fill('2500');
  await bookForm.locator('[name=volume]').fill('0');
  await bookForm.locator('button[type=submit]').click();
  await expect(bookForm).not.toBeVisible();

  await page.locator('#inventory-filters [name=libraryId]').selectOption(library.id);
  await page.locator('#inventory-filters button[type=submit]').click();
  const inventory = page.locator('#inventory-body tr').filter({hasText: title});
  await expect(inventory).toHaveCount(1);
  await inventory.getByRole('button', {name: 'Mouvement'}).click();
  const inventoryForm = page.locator('#inventory-form');
  await inventoryForm.locator('[name=operation]').selectOption('ENTRY');
  await inventoryForm.locator('[name=quantity]').fill('3');
  await inventoryForm.locator('[name=reason]').fill('Stock pour exports navigateur');
  await inventoryForm.locator('button[type=submit]').click();
  await expect(inventoryForm).not.toBeVisible();

  await page.locator('#add-supplier-button').click();
  const supplierForm = page.locator('#supplier-form');
  await supplierForm.locator('[name=libraryId]').selectOption(library.id);
  await supplierForm.locator('[name=name]').fill(supplier);
  await supplierForm.locator('[name=contactName]').fill('Contact export navigateur');
  await supplierForm.locator('button[type=submit]').click();
  await expect(supplierForm).not.toBeVisible();

  await page.locator('#add-purchase-button').click();
  const purchaseForm = page.locator('#purchase-form');
  await purchaseForm.locator('[name=libraryId]').selectOption(library.id);
  await purchaseForm.locator('[name=supplierId]').selectOption({label: supplier});
  const purchaseLine = page.locator('#purchase-lines .purchase-line');
  const purchaseBook = purchaseLine.locator('[name=bookId] option').filter({hasText: title});
  await expect(purchaseBook).toHaveCount(1);
  await purchaseLine.locator('[name=bookId]').selectOption(await purchaseBook.getAttribute('value'));
  await purchaseLine.locator('[name=quantity]').fill('2');
  await purchaseLine.locator('[name=unitCost]').fill('1200');
  await purchaseForm.locator('button[type=submit]').click();
  await expect(purchaseForm).not.toBeVisible();
  await page.locator('#purchase-filters [name=libraryId]').selectOption(library.id);
  await page.locator('#purchase-filters button[type=submit]').click();
  const purchaseRow = page.locator('#purchases-body tr').filter({hasText: supplier});
  await expect(purchaseRow).toHaveCount(1);
  const purchaseReference = (await purchaseRow.locator('td').first().textContent()).trim();

  await page.locator('#add-sale-button').click();
  const saleForm = page.locator('#sale-form');
  await saleForm.locator('[name=libraryId]').selectOption(library.id);
  await saleForm.locator('[name=customerName]').fill(customer);
  const saleLine = page.locator('#sale-lines .sale-line');
  const saleBook = saleLine.locator('[name=bookId] option').filter({hasText: title});
  await expect(saleBook).toHaveCount(1);
  await saleLine.locator('[name=bookId]').selectOption(await saleBook.getAttribute('value'));
  await saleLine.locator('[name=quantity]').fill('1');
  await saleForm.locator('button[type=submit]').click();
  await expect(saleForm).not.toBeVisible();
  await page.locator('#sale-filters [name=libraryId]').selectOption(library.id);
  await page.locator('#sale-filters button[type=submit]').click();
  const saleRow = page.locator('#sales-body tr').filter({hasText: customer});
  await expect(saleRow).toHaveCount(1);
  const saleReference = (await saleRow.locator('td').first().textContent()).trim();

  const exportButton = page.locator('#csv-exports-panel button[type=submit]');
  await expect(exportButton).toBeEnabled();
  await downloadExport(page, library.id, 'stocks', title);
  await downloadExport(page, library.id, 'sales', saleReference);
  await downloadExport(page, library.id, 'purchases', purchaseReference);
  await downloadExport(page, library.id, 'suppliers', supplier);
  await downloadExport(page, library.id, 'audit', 'CREATE');

  await page.evaluate(() => {
    window.__browserPrintCalls = 0;
    window.print = () => {
      window.__browserPrintCalls += 1;
      window.dispatchEvent(new Event('afterprint'));
    };
  });

  await saleRow.getByRole('button', {name: 'Détails'}).click();
  const saleDialog = page.locator('#sale-detail-dialog');
  await expect(saleDialog).toBeVisible();
  await expect(saleDialog.locator('#sale-detail-reference')).toHaveText(saleReference);
  await saleDialog.locator('#print-sale-button').click();
  await expect.poll(() => page.evaluate(() => window.__browserPrintCalls)).toBe(1);
  await expect(page.locator('body')).not.toHaveClass(/printing-sale/);
  await saleDialog.locator('[data-close="sale-detail-dialog"]').click();

  await purchaseRow.getByRole('button', {name: 'Détails'}).click();
  const purchaseDialog = page.locator('#purchase-detail-dialog');
  await expect(purchaseDialog).toBeVisible();
  await expect(purchaseDialog.locator('#purchase-detail-reference')).toHaveText(purchaseReference);
  await purchaseDialog.locator('#print-purchase-button').click();
  await expect.poll(() => page.evaluate(() => window.__browserPrintCalls)).toBe(2);
  await expect(page.locator('body')).not.toHaveClass(/printing-purchase/);
  await purchaseDialog.locator('[data-procurement-close="purchase-detail-dialog"]').click();
});
