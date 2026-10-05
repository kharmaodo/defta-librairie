const {test, expect} = require('@playwright/test');
const {readFileSync} = require('node:fs');
const {resolve} = require('node:path');
// Reuse the disposable server's public test fixture; never read application config.
const fixture = readFileSync(resolve(__dirname, '../../scripts/browser-test-server.cjs'), 'utf8');
const fixtureValue = key => fixture.match(new RegExp(`${key}:'([^']+)'`))[1];
for (const [width, height] of [[320,700], [390,844], [768,1024], [812,375]]) {
  test(`admin forms fit and retain native interactions at ${width}x${height}`, async ({page}) => {
    await page.setViewportSize({width, height});
    await page.goto('/login');
    await page.locator('[name=username]').fill(fixtureValue('DEFTA_ROOT_USERNAME'));
    await page.locator('[name=password]').fill(fixtureValue('DEFTA_ROOT_PASSWORD'));
    await page.locator('#login-form button[type=submit]').click();
    await expect(page).toHaveURL(/\/admin$/);
    for (const [opener, id] of [['add-book-button','book-dialog'], ['add-customer-button','customer-dialog'], ['add-supplier-button','supplier-dialog'], ['add-purchase-button','purchase-dialog']]) {
      const button = page.locator(`#${opener}`);
      await button.click();
      const dialog = page.locator(`#${id}`);
      await expect(dialog).toBeVisible();
      const geometry = await dialog.evaluate(element => {
        const box = element.getBoundingClientRect();
        return {x:box.x, y:box.y, right:box.right, bottom:box.bottom, client:element.clientWidth, scroll:element.scrollWidth};
      });
      expect(geometry.x).toBeGreaterThanOrEqual(0);
      expect(geometry.y).toBeGreaterThanOrEqual(0);
      expect(geometry.right).toBeLessThanOrEqual(width);
      expect(geometry.bottom).toBeLessThanOrEqual(height);
      expect(geometry.scroll).toBeLessThanOrEqual(geometry.client + 1);
      if (id === 'book-dialog') {
        const title = dialog.locator('[name=title]');
        await title.fill('عنوان Livre responsive');
        await expect(title).toHaveValue('عنوان Livre responsive');
        await dialog.locator('[name=price]').fill('-1');
        await dialog.locator('button[type=submit]').click();
        expect(await dialog.locator('[name=price]').evaluate(el => el.validity.rangeUnderflow)).toBe(true);
        await expect(dialog).toBeVisible();
      }
      if (id === 'purchase-dialog') {
        await dialog.locator('#add-purchase-line-button').click();
        await expect(dialog.locator('.purchase-line')).toHaveCount(2);
        await dialog.locator('.remove-purchase-line').last().click();
        await expect(dialog.locator('.purchase-line')).toHaveCount(1);
      }
      const cancel = dialog.locator('.modal-actions button[type=button]').first();
      await cancel.scrollIntoViewIfNeeded();
      await expect(cancel).toBeInViewport();
      expect((await cancel.boundingBox()).height).toBeGreaterThanOrEqual(44);
      await cancel.click();
      await expect(dialog).not.toBeVisible();
      await expect(button).toBeFocused();
    }
    await page.locator('#add-book-button').click();
    const dialog = page.locator('#book-dialog');
    for (let i=0; i<20; i++) {
      await page.keyboard.press(i % 5 === 0 ? 'Shift+Tab' : 'Tab');
      expect(await dialog.evaluate(el => el.contains(document.activeElement))).toBe(true);
    }
    await page.keyboard.press('Escape');
    await expect(dialog).not.toBeVisible();
    await expect(page.locator('#add-book-button')).toBeFocused();
  });
}
