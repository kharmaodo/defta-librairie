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
    data: {username: `return-${suffix}`, email: `return-${suffix}@example.test`, password: 'Browser-Return-Temp-2026!', library: {name: `Retours ${suffix}`, description: 'Test navigateur isolé'}}
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

async function amount(locator) {
  return Number((await locator.textContent()).replace(/[^0-9-]/g, ''));
}

async function expectAmount(locator, expected) {
  await expect.poll(() => amount(locator)).toBe(expected);
}

async function createReturn(page, {libraryID, customer, resolution}) {
  await page.locator('#add-return-button').click();
  const form = page.locator('#return-form');
  await expect(form).toBeVisible();
  await form.locator('[name=libraryId]').selectOption(libraryID);
  const option = form.locator('[name=saleId] option').filter({hasText: customer});
  await expect(option).toHaveCount(1);
  await form.locator('[name=saleId]').selectOption(await option.getAttribute('value'));
  const line = page.locator('#return-lines .return-line');
  await expect(line).toHaveCount(1);
  await line.locator('[name=quantity]').fill('1');
  await form.locator('[name=resolution]').selectOption(resolution);
  await form.locator('[name=reason]').fill(`Retour navigateur ${resolution}`);
  await form.locator('button[type=submit]').click();
  await expect(form).not.toBeVisible();

  const label = resolution === 'REFUND' ? 'Remboursement' : 'Avoir';
  const row = page.locator('#returns-body tr').filter({hasText: label}).filter({hasText: 'Brouillon'});
  await expect(row).toHaveCount(1);
  page.once('dialog', dialog => dialog.accept());
  await row.getByRole('button', {name: 'Finaliser'}).click();
  const completed = page.locator('#returns-body tr').filter({hasText: label}).filter({hasText: 'Finalisé'});
  await expect(completed).toHaveCount(1);
  return completed;
}

async function settleReturn(page, row, method) {
  await row.getByRole('button', {name: 'Règlements'}).click();
  const detail = page.locator('#return-detail-dialog');
  await expect(detail).toBeVisible();
  await expect(detail.locator('[data-return-balance-status]')).toHaveText('En attente');
  await expectAmount(detail.locator('[data-return-remaining]'), 1000);
  await detail.locator('#add-return-settlement-button').click();
  const form = page.locator('#return-settlement-form');
  await expect(form).toBeVisible();
  await form.locator('[name=method]').selectOption(method);
  await form.locator('button[type=submit]').click();
  await expect(form).not.toBeVisible();
  await expect(detail.locator('[data-return-balance-status]')).toHaveText('Soldé');
  await expectAmount(detail.locator('[data-return-remaining]'), 0);
  await expect(detail.locator('#return-settlements-body tr')).toHaveCount(1);
  await detail.locator('.modal-actions [data-return-close="return-detail-dialog"]').click();
}

test('refund and credit note restore stock and remain audited', async ({page, request}) => {
  const library = await createLibrary(request);
  const suffix = randomUUID().slice(0, 8);
  const title = `Livre retour ${suffix}`;
  const customer = `Client retour ${suffix}`;
  const register = `Caisse retour ${suffix}`;
  await loginRoot(page);

  const auditBefore = await page.evaluate(async () => {
    const completed = await window.DeftaHTTP.json('/api/audit-logs?action=COMPLETE_CUSTOMER_RETURN&offset=0&limit=100');
    const settled = await window.DeftaHTTP.json('/api/audit-logs?action=ISSUE_RETURN_SETTLEMENT&offset=0&limit=100');
    return {completed: completed.total, settled: settled.total};
  });

  await page.locator('#add-book-button').click();
  const bookForm = page.locator('#book-form');
  await bookForm.locator('[name=libraryId]').selectOption(library.id);
  await bookForm.locator('[name=title]').fill(title);
  await bookForm.locator('[name=price]').fill('1000');
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
  await inventoryForm.locator('[name=quantity]').fill('2');
  await inventoryForm.locator('[name=reason]').fill('Stock du parcours retour');
  await inventoryForm.locator('button[type=submit]').click();

  await page.locator('#add-sale-button').click();
  const saleForm = page.locator('#sale-form');
  await saleForm.locator('[name=libraryId]').selectOption(library.id);
  await saleForm.locator('[name=customerName]').fill(customer);
  const saleLine = page.locator('#sale-lines .sale-line');
  const bookOption = saleLine.locator('[name=bookId] option').filter({hasText: title});
  await expect(bookOption).toHaveCount(1);
  await saleLine.locator('[name=bookId]').selectOption(await bookOption.getAttribute('value'));
  await saleLine.locator('[name=quantity]').fill('2');
  await saleForm.locator('button[type=submit]').click();
  await page.locator('#sale-filters [name=libraryId]').selectOption(library.id);
  await page.locator('#sale-filters button[type=submit]').click();
  const sale = page.locator('#sales-body tr').filter({hasText: customer});
  page.once('dialog', dialog => dialog.accept());
  await sale.getByRole('button', {name: 'Confirmer'}).click();
  await expect(sale).toContainText('Confirmée');
  await expect(inventory.locator('td').nth(1)).toHaveText('0');

  await page.locator('#add-cash-register-button').click();
  const registerForm = page.locator('#cash-register-form');
  await registerForm.locator('[name=libraryId]').selectOption(library.id);
  await registerForm.locator('[name=name]').fill(register);
  await registerForm.locator('button[type=submit]').click();
  await page.locator('#payment-sale-filter [name=libraryId]').selectOption(library.id);
  const paymentSale = page.locator('#payment-sale-filter [name=saleId] option').filter({hasText: customer});
  await expect(paymentSale).toHaveCount(1);
  await page.locator('#payment-sale-filter [name=saleId]').selectOption(await paymentSale.getAttribute('value'));
  await page.locator('#payment-sale-filter button[type=submit]').click();
  await page.locator('#add-payment-button').click();
  const paymentForm = page.locator('#payment-form');
  await paymentForm.locator('[name=method]').selectOption('CASH');
  await paymentForm.locator('[name=amount]').fill('2000');
  await paymentForm.locator('button[type=submit]').click();
  await expect(page.locator('#payment-balance [data-payment-status]')).toHaveText('Payée');

  await page.locator('#return-filters [name=libraryId]').selectOption(library.id);
  await page.locator('#return-filters button[type=submit]').click();
  const refund = await createReturn(page, {libraryID: library.id, customer, resolution: 'REFUND'});
  await expect(inventory.locator('td').nth(1)).toHaveText('1');
  await settleReturn(page, refund, 'CASH');

  const credit = await createReturn(page, {libraryID: library.id, customer, resolution: 'CREDIT_NOTE'});
  await expect(inventory.locator('td').nth(1)).toHaveText('2');
  await settleReturn(page, credit, 'CREDIT_NOTE');

  await expect.poll(async () => page.evaluate(async before => {
    const completed = await window.DeftaHTTP.json('/api/audit-logs?action=COMPLETE_CUSTOMER_RETURN&offset=0&limit=100');
    const settled = await window.DeftaHTTP.json('/api/audit-logs?action=ISSUE_RETURN_SETTLEMENT&offset=0&limit=100');
    return completed.total - before.completed === 2 && settled.total - before.settled === 2;
  }, auditBefore)).toBe(true);
});
