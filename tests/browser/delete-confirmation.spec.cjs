const {test, expect} = require('@playwright/test');

const root = {username: 'browser-root', password: 'Browser-Root-Only-2026!'};

async function login(page) {
  await page.goto('/login');
  await page.locator('#login-form [name=username]').fill(root.username);
  await page.locator('#login-form [name=password]').fill(root.password);
  await page.locator('#login-form button[type=submit]').click();
  await expect(page).toHaveURL(/\/admin$/);
}

async function openConfirmation(page, expected = 'Livre Exact') {
  await page.evaluate(value => {
    const trigger = document.querySelector('#add-book-button');
    window.DeftaDeleteConfirmation.run({
      trigger,
      expected: value,
      subject: `le livre « ${value} »`,
      execute: async () => { document.body.dataset.deleteTestExecuted = 'true'; }
    });
  }, expected);
}

test('destructive confirmation requires exact text and resets accessibly', async ({page}) => {
  await login(page);
  const trigger = page.locator('#add-book-button');
  const dialog = page.locator('#delete-confirmation-dialog');
  const input = dialog.locator('[name=confirmation]');
  const submit = dialog.locator('[data-delete-confirm]');

  await openConfirmation(page);
  await expect(dialog).toBeVisible();
  await expect(input).toBeFocused();
  await expect(dialog.locator('[data-delete-expected]')).toHaveText('Livre Exact');
  await expect(submit).toBeDisabled();

  await input.fill('Livre Exact ');
  await expect(submit).toBeDisabled();
  await input.fill('livre Exact');
  await expect(submit).toBeDisabled();
  await input.fill('Livre Exact');
  await expect(submit).toBeEnabled();

  await page.keyboard.press('Escape');
  await expect(dialog).not.toBeVisible();
  await expect(trigger).toBeFocused();

  await openConfirmation(page);
  await expect(input).toHaveValue('');
  await expect(submit).toBeDisabled();
  await input.fill('Livre Exact');
  await submit.click();

  await expect(dialog).not.toBeVisible();
  await expect(trigger).toBeFocused();
  await expect(page.locator('body')).toHaveAttribute('data-delete-test-executed', 'true');
});
