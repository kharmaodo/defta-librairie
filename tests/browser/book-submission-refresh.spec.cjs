const {test, expect} = require('@playwright/test');

test('accepted book submission stays tracked after the creation dialog closes', async ({page}) => {
  await page.goto('/login');
  await page.setContent(`
    <dialog id="creation"><button type="button">Fermer</button></dialog>
    <p id="created" role="status"></p>
    <section id="book-submissions-panel"><button type="button">Actualiser</button>
      <p data-submission-notice></p><table><tbody></tbody></table></section>
    <dialog id="submission-retry-dialog"><form><button type="submit">Relancer</button>
      <p data-submission-retry-error hidden></p></form></dialog>
  `);
  await page.clock.install();
  await page.evaluate(() => {
    window.scenario = {results: [], reads: 0, created: 0};
    window.DeftaHTTP = {
      json: async () => ({role: 'OWNER_LIBRARY'}),
      request: async () => {
        window.scenario.reads++;
        return new Response(JSON.stringify({results: window.scenario.results}));
      }
    };
    document.addEventListener('defta:book-submission-created', event => {
      window.scenario.created++;
      document.querySelector('#created').textContent = `Livre ${event.detail.bookId} créé`;
    });
  });
  await page.addScriptTag({url: '/static/js/admin-book-submissions.js'});
  await page.evaluate(() => document.dispatchEvent(new Event('DOMContentLoaded')));
  await expect(page.locator('[data-submission-notice]')).toHaveText('Aucune soumission récente.');
  await page.evaluate(() => {
    const dialog = document.querySelector('#creation');
    dialog.showModal();
    window.scenario.results = [{id: 'submission-1', title: 'Livre arabe', moderationStatus: 'PENDING_SCAN'}];
    dialog.close();
    document.dispatchEvent(new CustomEvent('defta:book-submission-accepted', {detail: {id: 'submission-1'}}));
  });
  await expect(page.locator('#creation')).not.toBeVisible();
  await expect(page.locator('tbody')).toContainText('En attente d’analyse');
  await page.evaluate(() => {
    window.scenario.results[0].moderationStatus = 'APPROVED';
    window.scenario.results[0].createdBookId = 480;
  });
  await page.clock.fastForward(3000);
  await expect(page.locator('#created')).toHaveText('Livre 480 créé');
  await expect(page.locator('tbody')).toContainText('Approuvée');
  await expect(page.locator('tbody')).toContainText('480');
  const reads = await page.evaluate(() => window.scenario.reads);
  await page.clock.fastForward(9000);
  expect(await page.evaluate(() => window.scenario.reads)).toBe(reads);
  expect(await page.evaluate(() => window.scenario.created)).toBe(1);
});
