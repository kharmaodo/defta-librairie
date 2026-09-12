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
      username: `payment-${suffix}`,
      email: `payment-${suffix}@example.test`,
      password: 'Browser-Payment-Temp-2026!',
      library: {name: `Paiements ${suffix}`, description: 'Test navigateur isolé'}
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

async function displayedAmount(locator) {
  return Number((await locator.textContent()).replace(/[^0-9-]/g, ''));
}

async function expectAmount(locator, expected) {
  await expect.poll(() => displayedAmount(locator)).toBe(expected);
}

async function addPayment(page, {method, amount, reference = ''}) {
  await page.locator('#add-payment-button').click();
  const form = page.locator('#payment-form');
  await expect(form).toBeVisible();
  await form.locator('[name=method]').selectOption(method);
  await form.locator('[name=amount]').fill(String(amount));
  if (reference) await form.locator('[name=externalReference]').fill(reference);
  await form.locator('button[type=submit]').click();
  await expect(form).not.toBeVisible();
}

test('cash, mobile money and card payments update the remaining balance', async ({page, request}) => {
  const library = await createLibrary(request);
  const suffix = randomUUID().slice(0, 8);
  const title = `Livre paiement ${suffix}`;
  const customer = `Client paiement ${suffix}`;
  const register = `Caisse paiement ${suffix}`;
  await loginRoot(page);

  await page.locator('#add-book-button').click();
  const bookForm = page.locator('#book-form');
  await bookForm.locator('[name=libraryId]').selectOption(library.id);
  await bookForm.locator('[name=title]').fill(title);
  await bookForm.locator('[name=price]').fill('6000');
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
  await inventoryForm.locator('[name=quantity]').fill('1');
  await inventoryForm.locator('[name=reason]').fill('Stock du parcours paiement');
  await inventoryForm.locator('button[type=submit]').click();
  await expect(inventoryForm).not.toBeVisible();

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

  await page.locator('#sale-filters [name=libraryId]').selectOption(library.id);
  await page.locator('#sale-filters button[type=submit]').click();
  const sale = page.locator('#sales-body tr').filter({hasText: customer});
  await expect(sale).toHaveCount(1);
  page.once('dialog', dialog => dialog.accept());
  await sale.getByRole('button', {name: 'Confirmer'}).click();
  await expect(sale).toContainText('Confirmée');

  await page.locator('#add-cash-register-button').click();
  const registerForm = page.locator('#cash-register-form');
  await expect(registerForm).toBeVisible();
  await registerForm.locator('[name=libraryId]').selectOption(library.id);
  await registerForm.locator('[name=name]').fill(register);
  await registerForm.locator('button[type=submit]').click();
  await expect(registerForm).not.toBeVisible();
  await expect(page.locator('#cash-registers-body tr').filter({hasText: register})).toHaveCount(1);

  const paymentFilter = page.locator('#payment-sale-filter');
  await paymentFilter.locator('[name=libraryId]').selectOption(library.id);
  const saleOption = paymentFilter.locator('[name=saleId] option').filter({hasText: customer});
  await expect(saleOption).toHaveCount(1);
  await paymentFilter.locator('[name=saleId]').selectOption(await saleOption.getAttribute('value'));
  await paymentFilter.locator('button[type=submit]').click();

  const balance = page.locator('#payment-balance');
  await expect(balance).toBeVisible();
  await expect(balance.locator('[data-payment-status]')).toHaveText('Non payée');
  await expectAmount(balance.locator('[data-payment-remaining]'), 6000);

  await addPayment(page, {method: 'CASH', amount: 1000});
  await expect(balance.locator('[data-payment-status]')).toHaveText('Partiellement payée');
  await expectAmount(balance.locator('[data-payment-paid]'), 1000);
  await expectAmount(balance.locator('[data-payment-remaining]'), 5000);

  await addPayment(page, {method: 'MOBILE_MONEY', amount: 2000, reference: `MM-${suffix}`});
  await expectAmount(balance.locator('[data-payment-paid]'), 3000);
  await expectAmount(balance.locator('[data-payment-remaining]'), 3000);

  await addPayment(page, {method: 'CARD', amount: 3000, reference: `CARD-${suffix}`});
  await expect(balance.locator('[data-payment-status]')).toHaveText('Payée');
  await expectAmount(balance.locator('[data-payment-paid]'), 6000);
  await expectAmount(balance.locator('[data-payment-remaining]'), 0);
  await expect(page.locator('#add-payment-button')).toBeDisabled();
  await expect(page.locator('#payments-body tr')).toHaveCount(3);
});
