(() => {
  'use strict';

  const panel = document.querySelector('#dashboard-summary');
  if (!panel) return;

  const form = panel.querySelector('[data-summary-filters]');
  const results = panel.querySelector('[data-summary-results]');
  const status = panel.querySelector('[data-summary-status]');
  const error = panel.querySelector('[data-summary-error]');
  const library = panel.querySelector('[data-summary-library]');
  const number = new Intl.NumberFormat('fr-FR', {maximumFractionDigits: 2});
  const api = url => window.DeftaHTTP.json(url);
  let busy = false;

  const today = new Date().toISOString().slice(0, 10);
  form.elements.from.value = `${today.slice(0, 8)}01`;
  form.elements.to.value = today;

  function setValue(name, value) {
    panel.querySelector(`[data-summary-value="${name}"]`).textContent = value;
  }

  async function populateLibraries() {
    if (form.elements.libraryId.options.length > 1) return;
    let offset = 0;
    do {
      const owners = await api(`/api/admin/owners?offset=${offset}&limit=100`);
      for (const owner of owners.results) {
        if (owner.library) {
          form.elements.libraryId.add(new Option(owner.library.name, owner.library.id));
        }
      }
      offset += owners.results.length;
      if (!owners.results.length || offset >= owners.total) break;
    } while (true);
  }

  async function load() {
    if (busy) return;
    busy = true;
    panel.setAttribute('aria-busy', 'true');
    results.hidden = true;
    error.hidden = true;
    status.textContent = 'Chargement de la synthèse…';
    const controls = [...form.elements];
    controls.forEach(control => { control.disabled = true; });

    try {
      const me = await api('/api/auth/me');
      const root = me.role === 'SUPER_ADMIN_ROOT';
      if (!root && me.role !== 'OWNER_LIBRARY') throw new Error('Profil non autorisé.');
      library.hidden = !root;
      if (root) await populateLibraries();

      const libraryID = root ? form.elements.libraryId.value : '';
      if (root && !libraryID) {
        status.textContent = 'Choisissez une librairie pour afficher sa synthèse.';
        return;
      }

      const start = new Date(`${form.elements.from.value}T00:00:00Z`);
      const end = new Date(`${form.elements.to.value}T00:00:00Z`);
      if (!Number.isFinite(start.getTime()) || !Number.isFinite(end.getTime()) || start > end) {
        throw new Error('Choisissez une période valide.');
      }
      end.setUTCDate(end.getUTCDate() + 1);

      const statisticsQuery = new URLSearchParams({
        from: start.toISOString(),
        to: end.toISOString(),
      });
      if (libraryID) statisticsQuery.set('libraryId', libraryID);

      const alertQuery = kind => {
        const query = new URLSearchParams({offset: '0', limit: '1', kind});
        if (libraryID) query.set('libraryId', libraryID);
        return query;
      };

      const [statistics, outOfStock, lowStock, drafts] = await Promise.all([
        api(`/api/manage/statistics?${statisticsQuery}`),
        api(`/api/manage/alerts?${alertQuery('OUT_OF_STOCK')}`),
        api(`/api/manage/alerts?${alertQuery('LOW_STOCK')}`),
        api(`/api/manage/alerts?${alertQuery('DRAFT_PURCHASE')}`),
      ]);

      const totals = [outOfStock.total, lowStock.total, drafts.total];
      if (!Number.isFinite(statistics.netSales) ||
          (statistics.netMargin !== null && !Number.isFinite(statistics.netMargin)) ||
          !totals.every(total => Number.isSafeInteger(total) && total >= 0)) {
        throw new Error('Indicateurs incomplets dans la réponse du serveur.');
      }

      setValue('netSales', `${number.format(statistics.netSales)} F CFA`);
      setValue('netMargin', statistics.netMargin === null
        ? 'Indisponible'
        : `${number.format(statistics.netMargin)} F CFA`);
      panel.querySelector('[data-summary-margin-note]').textContent =
        statistics.netMargin === null ? 'Coût historique manquant' : 'Période sélectionnée';
      setValue('stockAlerts', number.format(outOfStock.total + lowStock.total));
      setValue('draftPurchases', number.format(drafts.total));
      results.hidden = false;
      status.textContent = 'Synthèse actualisée.';
    } catch (cause) {
      error.textContent = cause.message;
      error.hidden = false;
      status.textContent = '';
    } finally {
      busy = false;
      controls.forEach(control => { control.disabled = false; });
      panel.setAttribute('aria-busy', 'false');
    }
  }

  form.addEventListener('submit', event => {
    event.preventDefault();
    load();
  });
  form.addEventListener('change', () => {
    results.hidden = true;
    status.textContent = 'Filtres modifiés : actualisez la synthèse.';
  });
  load();
})();
