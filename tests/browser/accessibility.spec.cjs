const {test, expect} = require('@playwright/test');

const root = {username: 'browser-root', password: 'Browser-Root-Only-2026!'};

async function loginRoot(page) {
  await page.goto('/login');
  await page.locator('#login-form [name=username]').fill(root.username);
  await page.locator('#login-form [name=password]').fill(root.password);
  await page.locator('#login-form button[type=submit]').click();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.locator('#role-badge')).toHaveText('SUPER ADMIN ROOT');
}

test('keyboard navigation keeps dialog focus and announces errors', async ({page}) => {
  await loginRoot(page);

  const skipLink = page.getByRole('link', {name: 'Aller au contenu principal'});
  await page.keyboard.press('Tab');
  await expect(skipLink).toBeFocused();
  await expect(skipLink).toBeVisible();
  await page.keyboard.press('Enter');
  await expect(page.locator('#dashboard-main')).toBeFocused();

  const addBook = page.locator('#add-book-button');
  await addBook.focus();
  const indicator = await addBook.evaluate(element => {
    const style = getComputedStyle(element);
    return {style: style.outlineStyle, width: style.outlineWidth};
  });
  expect(indicator).toEqual({style: 'solid', width: '3px'});

  await page.keyboard.press('Enter');
  const dialog = page.locator('#book-dialog');
  await expect(dialog).toBeVisible();
  await expect(dialog).toHaveAccessibleName('Nouveau livre');
  await expect(dialog.locator(':focus')).toHaveCount(1);

  for (let index = 0; index < 20; index += 1) {
    await page.keyboard.press(index % 5 === 0 ? 'Shift+Tab' : 'Tab');
    expect(await page.evaluate(() => {
      const active = document.activeElement;
      return active && document.querySelector('#book-dialog').contains(active);
    })).toBe(true);
  }

  await page.keyboard.press('Escape');
  await expect(dialog).not.toBeVisible();
  await expect(addBook).toBeFocused();

  const exportPanel = page.locator('#csv-exports-panel');
  const exportForm = exportPanel.locator('form');
  await expect(exportForm.locator('button[type=submit]')).toBeEnabled();
  await exportForm.locator('[name=kind]').selectOption('stocks');
  await exportForm.locator('[name=libraryId]').selectOption('');
  await exportForm.locator('button[type=submit]').focus();
  await page.keyboard.press('Enter');

  const error = exportPanel.locator('[data-error]');
  await expect(error).toBeVisible();
  await expect(error).toHaveAttribute('role', 'alert');
  await expect(error).toHaveText('Choisissez une librairie.');
});
