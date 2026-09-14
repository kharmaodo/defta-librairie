const {test, expect} = require('@playwright/test');

const root = {username: 'browser-root', password: 'Browser-Root-Only-2026!'};
const widths = [390, 768, 1024, 1440];

async function login(page) {
  await page.goto('/login');
  await page.locator('#login-form [name=username]').fill(root.username);
  await page.locator('#login-form [name=password]').fill(root.password);
  await page.locator('#login-form button[type=submit]').click();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.locator('#role-badge')).toHaveText('SUPER ADMIN ROOT');
}

for (const width of widths) {
  test(`dashboard remains usable at ${width}px`, async ({page}) => {
    await page.setViewportSize({width, height: 900});
    await login(page);

    await expect(page.locator('#dashboard-summary')).toBeVisible();
    const layout = await page.evaluate(() => ({
      viewport: window.innerWidth,
      documentWidth: document.documentElement.scrollWidth,
      summaryRight: document.querySelector('#dashboard-summary').getBoundingClientRect().right,
      tableOverflow: getComputedStyle(document.querySelector('.table-wrap')).overflowX,
    }));
    expect(layout.documentWidth).toBeLessThanOrEqual(layout.viewport + 1);
    expect(layout.summaryRight).toBeLessThanOrEqual(layout.viewport + 1);
    expect(layout.tableOverflow).toBe('auto');

    const menuToggle = page.locator('[data-dashboard-menu-toggle]');
    if (width <= 1100) {
      await expect(menuToggle).toBeVisible();
      await menuToggle.click();
      await expect(menuToggle).toHaveAttribute('aria-expanded', 'true');
      await page.keyboard.press('Escape');
      await expect(menuToggle).toHaveAttribute('aria-expanded', 'false');
      await expect(menuToggle).toBeFocused();
    } else {
      await expect(menuToggle).toBeHidden();
      await expect(page.locator('[data-dashboard-nav]')).toBeVisible();
    }
  });
}
