const {test, expect} = require('@playwright/test');

test('book cover upload, failure, retry and authenticated preview in the form', async ({page}) => {
  await page.goto('/login');
  await page.setContent(`
    <button id="add-book-button">Ajouter</button>
    <form id="book-search-form"><input name="q"><button type="submit">Chercher</button></form>
    <button id="books-previous"></button><button id="books-next"></button>
    <span id="book-total"></span><span id="books-page-label"></span>
    <table><tbody id="books-body"></tbody></table>
    <dialog id="book-dialog">
      <form id="book-form">
        <input name="id"><input name="version"><input name="title">
        <input name="auteur"><input name="editeur"><input name="price">
        <input name="volume"><input name="status"><input name="categorie">
        <input name="tags"><select name="publisherId"></select>
        <select name="categoryIds" multiple></select><select name="primaryCategoryId"></select>
        <select name="tagIds" multiple></select><input name="coverUrl"><input name="libraryId">
        <input name="cover" type="file" accept="image/jpeg,image/png">
        <p id="book-cover-status" role="status"></p>
        <button id="book-cover-retry" type="button" hidden>Relancer le traitement</button>
        <img id="book-cover-preview" alt="Couverture par défaut" src="/static/img/book-cover-placeholder.svg">
        <p id="book-form-error" hidden></p>
        <button type="submit">Enregistrer</button>
      </form>
      <h2 id="book-form-title"></h2>
    </dialog>
    <div id="tag-library"></div><p id="dashboard-error" hidden></p>
  `);
  await page.addScriptTag({url: '/static/js/admin-books.js'});
  await page.evaluate(() => {
    const book = {
      id: 42, version: 1, title: 'Livre Exact', auteur: 'Auteur', editeur: '',
      price: 10, volume: 0, status: 'AVAILABLE', categorie: '', tags: '',
      coverUrl: '', libraryId: 'library-1', publisherId: 1,
      categoryIds: [2], primaryCategoryId: 2, tagIds: ['tag-1']
    };
    window.coverScenario = {state: 'ABSENT', uploads: 0, retries: 0, previewReads: 0};
    window.DeftaHTTP = {
      request: async () => {
        window.coverScenario.previewReads++;
        if (window.coverScenario.state !== 'READY') throw Object.assign(new Error('Absent'), {status: 404});
        return new Response(new Blob([Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLttAAAAABJRU5ErkJggg=='), c => c.charCodeAt(0))], {type: 'image/png'}));
      }
    };
    const apiFetch = async (path, options = {}) => {
      const state = window.coverScenario;
      if (path.startsWith('/api/manage/books?')) return {results: [book], total: 1, offset: 0, limit: 10};
      if (path === '/api/manage/categories') return [{id: 2, name: 'Fiqh'}];
      if (path === '/api/manage/publishers') return [{id: 1, name: 'Dar al Fikr'}];
      if (path.startsWith('/api/manage/tags')) return [{id: 'tag-1', name: 'Fiqh'}];
      if (path.endsWith('/cover/status')) {
        if (state.state === 'ABSENT') throw Object.assign(new Error('Absent'), {status: 404});
        return {status: state.state, canRetry: state.state === 'FAILED'};
      }
      if (path.endsWith('/cover/retry')) {
        state.retries++;
        state.state = 'READY';
        return {status: 'PENDING', canRetry: false};
      }
      if (path.endsWith('/cover') && options.method === 'POST') {
        const file = options.body.get('cover');
        if (!(file instanceof File) || file.type !== 'image/png') throw new Error('Fichier incorrect');
        state.uploads++;
        state.state = 'FAILED';
        return {status: 'PENDING'};
      }
      if (path === '/api/manage/books/42' && options.method === 'PUT') {
        book.version++;
        return {...book};
      }
      throw new Error(`Unexpected request: ${path}`);
    };
    const textCell = (row, value) => {
      const cell = row.insertCell();
      cell.textContent = value;
      return cell;
    };
    const actionButton = (label, action, id) => {
      const button = document.createElement('button');
      button.type = 'button';
      button.textContent = label;
      button.dataset.action = action;
      button.dataset.id = id;
      return button;
    };
    const module = window.DeftaBooks.create({
      apiFetch, textCell, actionButton, formatDate: String,
      showError: (element, error) => {element.textContent = error.message; element.hidden = false;},
      errorBox: document.querySelector('#dashboard-error'), isRoot: () => false,
      reloadInventory: async () => {}, reloadTags: async () => {}, renderTags: () => {}
    });
    module.init();
    module.reload();
  });

  await page.locator('#books-body').getByRole('button', {name: 'Modifier'}).click();
  await expect(page.locator('#book-cover-status')).toHaveText('Aucune couverture');
  await expect(page.locator('#book-form [name=publisherId]')).toHaveValue('1');
  await expect(page.locator('#book-form [name=categoryIds] option:checked')).toHaveCount(1);
  await expect(page.locator('#book-form [name=tagIds] option:checked')).toHaveCount(1);
  await expect(page.locator('#book-cover-preview')).toHaveAttribute('alt', 'Couverture par défaut');
  await expect(page.locator('#books-body .book-cover-thumb')).toHaveAttribute('alt', 'Couverture par défaut');
  const pixel = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLttAAAAABJRU5ErkJggg==', 'base64');
  await page.locator('#book-form [name=cover]').setInputFiles({name: 'cover.png', mimeType: 'image/png', buffer: pixel});
  await page.locator('#book-form button[type=submit]').click();
  await expect(page.locator('#book-cover-status')).toContainText('échoué');
  await expect(page.locator('#book-cover-retry')).toBeVisible();
  await page.locator('#book-cover-retry').click();
  await expect(page.locator('#book-cover-status')).toContainText('disponible');
  await expect(page.locator('#book-cover-preview')).toHaveAttribute('alt', 'Couverture du livre');
  await expect(page.locator('#book-cover-preview')).toBeVisible();
  expect(await page.evaluate(() => window.coverScenario.uploads)).toBe(1);
  expect(await page.evaluate(() => window.coverScenario.retries)).toBe(1);
});
