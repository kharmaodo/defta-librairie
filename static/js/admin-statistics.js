(() => {
  'use strict';
  const panel = document.querySelector('#commercial-statistics-panel');
  if (!panel) return;
  const form = panel.querySelector('form');
  const output = panel.querySelector('[data-results]');
  const notice = panel.querySelector('[data-notice]');
  const error = panel.querySelector('[data-error]');
  let busy = false;
  const number = new Intl.NumberFormat('fr-FR', {maximumFractionDigits: 2});
  const today = new Date().toISOString().slice(0, 10);
  form.elements.from.value = today.slice(0, 8) + '01';
  form.elements.to.value = today;

  const api = (url, options) => window.DeftaHTTP.json(url, options);

  function renderSupplierCosts(data) {
    const unknown = data.supplierReturnUnknownCostLines > 0 ||
      data.supplierReturnInventoryCost === null || data.supplierReturnCostVariance === null;
    panel.querySelector('[data-value="supplierReturnInventoryCost"]').textContent = unknown
      ? 'Indisponible' : number.format(data.supplierReturnInventoryCost) + ' F CFA';
    panel.querySelector('[data-value="supplierReturnCostVariance"]').textContent = unknown
      ? 'Indisponible' : (data.supplierReturnCostVariance > 0 ? '+' : '') + number.format(data.supplierReturnCostVariance) + ' F CFA';
    panel.querySelector('[data-supplier-cost-notice]').textContent = unknown
      ? `Valorisation et écart indisponibles : ${data.supplierReturnUnknownCostLines} ligne(s) sans coût figé. Partie connue du coût : ${number.format(data.supplierReturnKnownCost)} F CFA ; ce montant est partiel.`
      : 'Valorisation complète des retours expédiés sur la période. Ces montants ne représentent pas des remboursements reçus.';
  }

  async function load() {
    if (busy) return;
    busy = true;
    output.hidden = true;
    notice.textContent = 'Chargement…';
    error.hidden = true;
    panel.setAttribute('aria-busy', 'true');
    const controls = [...form.querySelectorAll('input,select,button')];
    controls.forEach(control => { control.disabled = true; });
    try {
      const me = await api('/api/auth/me');
      const isRoot = me.role === 'SUPER_ADMIN_ROOT';
      if (!isRoot && me.role !== 'OWNER_LIBRARY') throw new Error('Profil non autorisé.');
      form.querySelector('[data-library]').hidden = !isRoot;
      if (isRoot && form.elements.libraryId.options.length === 1) {
        let offset = 0;
        const options = [];
        do {
          const owners = await api(`/api/admin/owners?offset=${offset}&limit=100`);
          for (const owner of owners.results) {
            if (owner.library) options.push(new Option(owner.library.name, owner.library.id));
          }
          offset += owners.results.length;
          if (!owners.results.length || offset >= owners.total) break;
        } while (true);
        form.elements.libraryId.append(...options);
      }
      if (isRoot && !form.elements.libraryId.value) {
        notice.textContent = 'Choisissez une librairie puis cliquez sur Actualiser.';
        return;
      }
      const start = new Date(`${form.elements.from.value}T00:00:00Z`);
      const end = new Date(`${form.elements.to.value}T00:00:00Z`);
      if (!Number.isFinite(start.getTime()) || !Number.isFinite(end.getTime()) || start > end) {
        throw new Error('Choisissez une période valide : la date de début doit précéder ou égaler la date de fin.');
      }
      end.setUTCDate(end.getUTCDate() + 1);
      const query = new URLSearchParams({from: start.toISOString(), to: end.toISOString()});
      if (isRoot) query.set('libraryId', form.elements.libraryId.value);
      const data = await api(`/api/manage/statistics?${query}`);
      const fields = ['grossSales', 'cancellations', 'customerReturns', 'netSales', 'receivedPurchases', 'supplierReturns', 'netPurchases'];
      if (fields.some(key => !Number.isFinite(data[key])) ||
          !Number.isSafeInteger(data.unknownCostEvents) || data.unknownCostEvents < 0 ||
          !Number.isSafeInteger(data.supplierReturnUnknownCostLines) || data.supplierReturnUnknownCostLines < 0 ||
          !Number.isFinite(data.supplierReturnKnownCost) ||
          (data.supplierReturnInventoryCost !== null && !Number.isFinite(data.supplierReturnInventoryCost)) ||
          (data.supplierReturnCostVariance !== null && !Number.isFinite(data.supplierReturnCostVariance)) ||
          (data.netMargin !== null && !Number.isFinite(data.netMargin))) {
        throw new Error('Indicateurs incomplets dans la réponse du serveur.');
      }
      fields.forEach(key => { panel.querySelector(`[data-value="${key}"]`).textContent = number.format(data[key]) + ' F CFA'; });
      const unknown = data.unknownCostEvents > 0 || data.netMargin === null;
      panel.querySelector('[data-value="netMargin"]').textContent = unknown ? 'Indisponible' : number.format(data.netMargin) + ' F CFA';
      notice.textContent = unknown
        ? `Marge indisponible : coût historique manquant pour ${data.unknownCostEvents} ligne(s) d’événement. Les ventes restent consultables.`
        : 'Indicateurs actualisés. Les montants décrivent l’activité commerciale de la période.';
      renderSupplierCosts(data);
      output.hidden = false;
    } catch (e) {
      notice.textContent = '';
      error.textContent = e.message;
      error.hidden = false;
    } finally {
      busy = false;
      controls.forEach(control => { control.disabled = false; });
      panel.setAttribute('aria-busy', 'false');
    }
  }
  form.addEventListener('submit', event => { event.preventDefault(); load(); });
  form.addEventListener('change', () => {
    output.hidden = true;
    notice.textContent = 'Filtres modifiés : cliquez sur Actualiser.';
  });
  load();
})();
