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

test('mobile dark dashboard remains keyboard-usable with reduced motion', async ({page}) => {
  await page.setViewportSize({width: 390, height: 844});
  await page.emulateMedia({colorScheme: 'dark', reducedMotion: 'reduce'});
  await page.addInitScript(() => localStorage.removeItem('defta.adminTheme'));
  await loginRoot(page);

  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await expect(page.locator('[data-theme-toggle]')).toHaveAttribute('aria-pressed', 'true');

  const menuToggle = page.locator('[data-dashboard-menu-toggle]');
  await menuToggle.focus();
  await page.keyboard.press('Enter');
  await expect(menuToggle).toHaveAttribute('aria-expanded', 'true');

  const navigation = page.locator('[data-dashboard-nav]');
  await expect(navigation).toBeInViewport();
  const commerce = navigation.getByRole('button', {name: 'Commerce'});
  await commerce.focus();
  await page.keyboard.press('Enter');
  await expect(commerce).toHaveAttribute('aria-expanded', 'true');

  const sales = navigation.getByRole('link', {name: 'Ventes'});
  await sales.focus();
  await expect(sales).toBeFocused();

  const motion = await navigation.evaluate(element => {
    const duration = getComputedStyle(element).transitionDuration;
    return Math.max(...duration.split(',').map(value => {
      const trimmed = value.trim();
      return Number.parseFloat(trimmed) * (trimmed.endsWith('ms') ? 1 : 1000);
    }));
  });
  expect(motion).toBeLessThanOrEqual(0.01);

  await page.keyboard.press('Escape');
  await expect(navigation).not.toBeInViewport();
  await expect(menuToggle).toBeFocused();

  const dimensions = await page.evaluate(() => ({
    viewport: window.innerWidth,
    document: document.documentElement.scrollWidth,
  }));
  expect(dimensions.document).toBeLessThanOrEqual(dimensions.viewport + 1);
});
