const {test, expect} = require('@playwright/test');

const root = {username: 'browser-root', password: 'Browser-Root-Only-2026!'};

async function login(page) {
  await page.goto('/login');
  await page.locator('#login-form [name=username]').fill(root.username);
  await page.locator('#login-form [name=password]').fill(root.password);
  await page.locator('#login-form button[type=submit]').click();
  await expect(page).toHaveURL(/\/admin$/);
}

async function colors(control) {
  return control.evaluate(element => {
    const style = getComputedStyle(element);
    const rootStyle = getComputedStyle(document.documentElement);
    const resolveColor = name => {
      const probe = document.createElement('span');
      probe.style.color = rootStyle.getPropertyValue(name);
      document.body.append(probe);
      const value = getComputedStyle(probe).color;
      probe.remove();
      return value;
    };
    return {
      border: style.borderTopColor,
      background: style.backgroundColor,
      expectedBorder: resolveColor('--input-border-color'),
      expectedHover: resolveColor('--input-border-hover-color'),
      expectedDisabled: resolveColor('--input-disabled-background')
    };
  });
}

async function verifyTheme(page, theme) {
  await expect(page.locator('html')).toHaveAttribute('data-theme', theme);
  const control = page.locator('#book-form [name=title]');
  await expect(control).toBeVisible();

  let actual = await colors(control);
  expect(actual.border).toBe(actual.expectedBorder);
  expect(actual.border).not.toBe(actual.background);

  await control.hover();
  await expect.poll(async () => {
    const current = await colors(control);
    return current.border === current.expectedHover;
  }).toBe(true);

  await control.focus();
  await expect(control).toBeFocused();
  actual = await colors(control);
  expect(actual.border).not.toBe(actual.expectedBorder);

  await control.evaluate(element => { element.disabled = true; });
  await expect(control).toBeDisabled();
  await expect.poll(async () => {
    const current = await colors(control);
    return current.background === current.expectedDisabled;
  }).toBe(true);
  await control.evaluate(element => { element.disabled = false; });
}

test('form controls remain visible and expose their states in both themes', async ({page}) => {
  await page.addInitScript(() => localStorage.setItem('defta.adminTheme', 'light'));
  await login(page);
  await page.locator('#add-book-button').click();
  await verifyTheme(page, 'light');
  await page.locator('#book-dialog').getByRole('button', {name: 'Annuler', exact: true}).click();
  await expect(page.locator('#book-dialog')).not.toBeVisible();

  await page.locator('[data-theme-toggle]').click();
  await page.locator('#add-book-button').click();
  await verifyTheme(page, 'dark');
  await page.locator('#book-dialog').getByRole('button', {name: 'Annuler', exact: true}).click();
  await expect(page.locator('#book-dialog')).not.toBeVisible();
});
