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
    data: {username: `supplier-return-${suffix}`, email: `supplier-return-${suffix}@example.test`, password: 'Browser-Supplier-Return-2026!', library: {name: `Retour fournisseur ${suffix}`, description: 'Test navigateur isolé'}}
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

test('shipped supplier return removes received stock and preserves valuation', async ({page, request}) => {
  test.setTimeout(60_000);
  const library = await createLibrary(request);
  const suffix = randomUUID().slice(0, 8);
  const title = `Livre retour fournisseur ${suffix}`;
  const supplier = `Fournisseur retour ${suffix}`;
  const reason = `Deux exemplaires endommagés ${suffix}`;
  await loginRoot(page);

  const auditBefore = await page.evaluate(async () =>
    (await window.DeftaHTTP.json('/api/audit-logs?action=SHIP_SUPPLIER_RETURN&offset=0&limit=100')).total);

  await page.locator('#add-book-button').click();
  const bookForm = page.locator('#book-form');
  await bookForm.locator('[name=libraryId]').selectOption(library.id);
  await bookForm.locator('[name=title]').fill(title);
  await bookForm.locator('[name=price]').fill('2500');
  await bookForm.locator('[name=volume]').fill('0');
  await bookForm.locator('button[type=submit]').click();
  await expect(bookForm).not.toBeVisible();

  await page.locator('#add-supplier-button').click();
  const supplierForm = page.locator('#supplier-form');
  await supplierForm.locator('[name=libraryId]').selectOption(library.id);
  await supplierForm.locator('[name=name]').fill(supplier);
  await supplierForm.locator('[name=contactName]').fill('Contact retour fournisseur');
  await supplierForm.locator('button[type=submit]').click();
  await expect(supplierForm).not.toBeVisible();
  await expect(page.locator('#suppliers-body tr').filter({hasText: supplier})).toHaveCount(1);

  await page.locator('#add-purchase-button').click();
  const purchaseForm = page.locator('#purchase-form');
  await expect(purchaseForm).toBeVisible();
  await purchaseForm.locator('[name=libraryId]').selectOption(library.id);
  await purchaseForm.locator('[name=supplierId]').selectOption({label: supplier});
  const purchaseLine = page.locator('#purchase-lines .purchase-line');
  const bookOption = purchaseLine.locator('[name=bookId] option').filter({hasText: title});
  await expect(bookOption).toHaveCount(1);
  await purchaseLine.locator('[name=bookId]').selectOption(await bookOption.getAttribute('value'));
  await purchaseLine.locator('[name=quantity]').fill('5');
  await purchaseLine.locator('[name=unitCost]').fill('1200');
  await purchaseForm.locator('button[type=submit]').click();
  await expect(purchaseForm).not.toBeVisible();

  await page.locator('#purchase-filters [name=libraryId]').selectOption(library.id);
  await page.locator('#purchase-filters [name=status]').selectOption('DRAFT');
  await page.locator('#purchase-filters button[type=submit]').click();
  const purchase = page.locator('#purchases-body tr').filter({hasText: 'DRAFT'});
  await expect(purchase).toHaveCount(1);
  page.once('dialog', dialog => dialog.accept());
  await Promise.all([page.waitForNavigation(), purchase.getByRole('button', {name: 'Réceptionner'}).click()]);
  await expect(page.locator('#role-badge')).toHaveText('SUPER ADMIN ROOT');

  await page.locator('#inventory-filters [name=libraryId]').selectOption(library.id);
  await page.locator('#inventory-filters button[type=submit]').click();
  const inventory = page.locator('#inventory-body tr').filter({hasText: title});
  await expect(inventory).toHaveCount(1);
  await expect(inventory.locator('td').nth(1)).toHaveText('5');

  const panel = page.locator('#supplier-returns-panel');
  await panel.locator('[data-filters] [name=libraryId]').selectOption(library.id);
  await panel.locator('[data-filters] button').click();
  await panel.locator('[data-new]').click();
  const editor = panel.locator('[data-editor]');
  await expect(editor).toBeVisible();
  await editor.locator('[name=libraryId]').selectOption(library.id);
  const purchaseOption = editor.locator('[name=purchaseId] option').filter({hasText: /A-/});
  await expect(purchaseOption).toHaveCount(1);
  await editor.locator('[name=purchaseId]').selectOption(await purchaseOption.getAttribute('value'));
  const returnLine = editor.locator('[data-line]');
  await expect(returnLine).toHaveCount(1);
  await returnLine.fill('2');
  await editor.locator('[name=reason]').fill(reason);
  await editor.locator('[name=supplierReference]').fill(`RF-EXT-${suffix}`);
  await editor.locator('[data-save]').click();
  await expect(editor).not.toBeVisible();

  const draft = panel.locator('tbody tr').filter({hasText: reason}).filter({hasText: 'Brouillon'});
  await expect(draft).toHaveCount(1);
  page.once('dialog', dialog => dialog.accept());
  await draft.getByRole('button', {name: 'Expédier'}).click();
  const shipped = panel.locator('tbody tr').filter({hasText: reason}).filter({hasText: 'Expédié'});
  await expect(shipped).toHaveCount(1);
  await expect(inventory.locator('td').nth(1)).toHaveText('3');

  await shipped.getByRole('button', {name: 'Détails'}).click();
  await expect(editor).toBeVisible();
  await expect(editor.locator('[data-costs]')).toBeVisible();
  const cost = editor.locator('[data-cost-rows] tr');
  await expect(cost).toHaveCount(1);
  await expect(cost).toContainText(title);
  await expect(cost).toContainText('2 400');
  await editor.locator('[data-close]').click();

  await expect.poll(async () => page.evaluate(async before =>
    (await window.DeftaHTTP.json('/api/audit-logs?action=SHIP_SUPPLIER_RETURN&offset=0&limit=100')).total - before, auditBefore)).toBe(1);
});
