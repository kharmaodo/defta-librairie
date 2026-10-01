const {test, expect} = require('@playwright/test');

test('relational tags reload without the removed suggestions datalist', async ({page}) => {
  await page.goto('/login');
  await page.setContent(`<select id="tag-library"><option value="">Choisir</option><option value="library-1">Librairie</option></select><div id="tags-list"></div><p id="error" hidden></p>`);
  await page.addScriptTag({url: '/static/js/admin-tags.js'});
  const failures = [];
  page.on('pageerror', error => failures.push(error.message));
  await page.evaluate(async () => {
    window.tagsModule = window.DeftaTags.create({
      apiFetch: async () => ({results: [{id: 'tag-1', name: 'Fiqh'}]}),
      showError: (box, error) => {box.textContent = error.message; box.hidden = false;},
      errorBox: document.querySelector('#error'),
      isRoot: () => true, reloadAudit: async () => {}
    });
    await window.tagsModule.reload();
  });
  await expect(page.locator('#tags-list')).toHaveText('Aucun tag défini');
  await expect(page.locator('#tag-suggestions')).toHaveCount(0);
  await page.evaluate(async () => {
    document.querySelector('#tag-library').value = 'library-1';
    await window.tagsModule.reload();
  });
  await expect(page.locator('#tags-list')).toContainText('Fiqh');
  await expect(page.getByRole('button', {name: 'Supprimer Fiqh'})).toBeVisible();
  expect(failures).toEqual([]);
});
