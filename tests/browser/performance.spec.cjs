const {test, expect} = require('@playwright/test');

const root = {username: 'browser-root', password: 'Browser-Root-Only-2026!'};

test('initial dashboard coalesces concurrent profile reads', async ({page}) => {
  let profileReads = 0;
  page.on('request', request => {
    const url = new URL(request.url());
    if (request.method() === 'GET' && url.pathname === '/api/auth/me') {
      profileReads++;
    }
  });

  await page.goto('/login');
  await page.locator('#login-form [name=username]').fill(root.username);
  await page.locator('#login-form [name=password]').fill(root.password);
  await page.locator('#login-form button[type=submit]').click();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.locator('#role-badge')).toHaveText('SUPER ADMIN ROOT');
  await page.waitForLoadState('networkidle');

  expect(profileReads).toBe(1);

  const assets = await page.evaluate(() => {
    const resources = performance.getEntriesByType('resource')
      .map(entry => new URL(entry.name).pathname);
    return {
      adminScripts: resources.filter(path => /^\/static\/js\/admin-.*\.js$/.test(path)).length,
      stylesheets: resources.filter(path => path === '/static/css/admin.css').length,
    };
  });
  expect(assets.adminScripts).toBeLessThanOrEqual(22);
  expect(assets.stylesheets).toBe(1);
});
