const {test, expect} = require('@playwright/test');

for (const [width,height] of [[320,700],[390,844],[768,1024],[812,375]]) {
  test(`admin navigation isolates focus and keeps actions usable at ${width}x${height}`, async ({page}) => {
    await page.setViewportSize({width,height});
    await page.emulateMedia({reducedMotion:'reduce'});
    await page.goto('/login');
    await page.locator('[name=username]').fill('browser-root');
    await page.locator('[name=password]').fill('Browser-Root-Only-2026!');
    await page.locator('#login-form button[type=submit]').click();
    await expect(page).toHaveURL(/\/admin$/);
    const navigation = page.locator('[data-dashboard-nav]');
    const toggle = page.locator('[data-dashboard-menu-toggle]');
    const close = page.locator('[data-dashboard-menu-close]');
    await expect(navigation).toHaveAttribute('inert','');
    for (const id of ['change-password-button','logout-button']) {
      const action = page.locator(`#${id}`);
      await expect(action).toBeVisible();
      const box = await action.boundingBox();
      expect(box.x).toBeGreaterThanOrEqual(0);
      expect(box.x+box.width).toBeLessThanOrEqual(width+1);
      expect(box.height).toBeGreaterThanOrEqual(44);
    }
    await toggle.click();
    await expect(close).toBeFocused();
    await expect(navigation).toHaveAttribute('aria-modal','true');
    await expect(page.locator('#dashboard-main')).toHaveAttribute('inert','');
    const navBox = await navigation.boundingBox();
    expect(navBox.y).toBe(0);
    expect(navBox.height).toBeLessThanOrEqual(height+1);
    await page.keyboard.press('Shift+Tab');
    expect(await page.evaluate(()=>document.querySelector('[data-dashboard-nav]').contains(document.activeElement))).toBe(true);
    await page.keyboard.press('Tab');
    await expect(close).toBeFocused();
    await navigation.getByRole('button',{name:'Catalogue',exact:true}).click();
    await navigation.getByRole('link',{name:'Livres',exact:true}).click();
    await expect(page.locator('#books-panel')).toBeFocused();
    await expect(page.locator('#dashboard-main')).not.toHaveAttribute('inert','');
    const target = await page.locator('#books-panel').boundingBox();
    expect(target.y).toBeGreaterThanOrEqual(-1);
    await toggle.scrollIntoViewIfNeeded();
    await toggle.click();
    await close.click();
    await expect(toggle).toBeFocused();
    await toggle.click();
    await page.keyboard.press('Escape');
    await expect(toggle).toBeFocused();
    await toggle.click();
    await page.setViewportSize({width:1440,height:900});
    await expect(navigation).not.toHaveAttribute('inert','');
    await expect(navigation).not.toHaveAttribute('aria-modal','true');
    await expect(page.locator('#dashboard-main')).not.toHaveAttribute('inert','');
    await expect(close).toBeHidden();
    await page.setViewportSize({width,height});
    await expect(navigation).toHaveAttribute('inert','');
    expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(width+1);
  });
}
