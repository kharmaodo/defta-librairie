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

test('admin dashboard keeps its thematic structure and stable section targets', async ({page}) => {
  await loginRoot(page);

  const header = page.locator('header.topbar');
  await expect(header.getByRole('link', {name: /Defta/})).toBeVisible();
  await expect(header.getByRole('button', {name: 'Mot de passe'})).toBeVisible();
  await expect(header.getByRole('button', {name: 'Se déconnecter'})).toBeVisible();

  const navigation = page.locator('[data-dashboard-nav]');
  const expectedGroups = [
    'Vue d’ensemble',
    'Catalogue',
    'Commerce',
    'Approvisionnement',
    'Administration'
  ];

  const groupButtons = navigation.locator('[data-dashboard-nav-group] > button');
  await expect(groupButtons).toHaveCount(expectedGroups.length);

  for (const name of expectedGroups) {
    const button = navigation.getByRole('button', {name});
    await expect(button).toHaveAttribute('aria-expanded', /^(true|false)$/);
    const controlledID = await button.getAttribute('aria-controls');
    await expect(page.locator(`#${controlledID}`)).toHaveCount(1);
  }

  const links = navigation.locator('[data-dashboard-nav-link]');
  await expect(links).toHaveCount(19);

  for (const link of await links.all()) {
    const targetID = await link.getAttribute('href');
    expect(targetID).toMatch(/^#[a-z][a-z0-9-]+$/);
    await expect(page.locator(targetID)).toHaveCount(1);
  }

  await expect(page.locator('#owners-section')).toBeVisible();
  await navigation.getByRole('button', {name: 'Administration'}).click();
  await expect(navigation.getByRole('link', {name: 'Propriétaires'})).toBeVisible();
  await expect(page.locator('footer.admin-footer'))
    .toHaveText(/^Defta Librairie · \S+ · \S+$/);
});
