(() => {
  'use strict';
  const dialog = document.querySelector('#customer-history-dialog');
  if (!dialog) return;
  const $ = selector => dialog.querySelector(selector);
  const form = $('[data-history-filters]');
  const results = $('[data-history-results]'), error = $('[data-history-error]'), notice = $('[data-history-notice]');
  const money = new Intl.NumberFormat('fr-FR', {style:'currency', currency:'XOF', maximumFractionDigits:2});
  const date = new Intl.DateTimeFormat('fr-FR', {dateStyle:'short', timeStyle:'short', timeZone:'UTC'});
  let customerID = '', offset = 0, total = 0, generation = 0, controller;
  const limit = 10;
  function invalidate(message = '') {
    generation++; controller?.abort(); results.hidden = true; error.hidden = true;
    notice.textContent = message; dialog.setAttribute('aria-busy','false');
  }
  async function load() {
    invalidate('Chargement…');
    const current = generation;
    controller = new AbortController();
    dialog.setAttribute('aria-busy','true');
    try {
      const query = new URLSearchParams({offset:String(offset),limit:String(limit)});
      if (form.elements.status.value) query.set('status', form.elements.status.value);
      const from = form.elements.from.value, to = form.elements.to.value;
      if (from && to && from > to) throw new Error('La date de début doit précéder ou égaler la date de fin.');
      if (from) query.set('from',new Date(from+'T00:00:00Z').toISOString());
      if (to) { const end = new Date(to+'T00:00:00Z'); end.setUTCDate(end.getUTCDate()+1); query.set('to',end.toISOString()); }
      const token = sessionStorage.getItem('defta.accessToken');
      if (!token) throw new Error('Connectez-vous pour consulter cet historique.');
      const response = await fetch(`/api/manage/customers/${encodeURIComponent(customerID)}/sales?${query}`, {headers:{Authorization:`Bearer ${token}`},cache:'no-store',signal:controller.signal});
      if (!response.ok) throw new Error(response.status===401 ? 'Session expirée : reconnectez-vous.' : response.status===404 ? 'Client introuvable ou inaccessible.' : `Historique indisponible (HTTP ${response.status}).`);
      const data = await response.json();
      if (current !== generation || !dialog.open) return;
      if (!Array.isArray(data.results) || !Number.isSafeInteger(data.total) || data.total<0) throw new Error('Réponse inattendue du serveur.');
      total=data.total;
      const body=$('[data-history-rows]'); body.replaceChildren();
      for (const sale of data.results) {
        const row=body.insertRow();
        const timestamp=sale.status==='DRAFT' ? sale.createdAt : sale.confirmedAt;
        const parsed=timestamp ? new Date(timestamp) : null;
        const label={CONFIRMED:'Confirmée',CANCELLED:'Annulée',DRAFT:'Brouillon — non finalisé'}[sale.status] || sale.status;
        for (const text of [sale.reference,parsed && Number.isFinite(parsed.getTime()) ? date.format(parsed) : 'Date inconnue',label,Number.isFinite(sale.totalAmount) ? money.format(sale.totalAmount) : 'Indisponible']) row.insertCell().textContent=text;
      }
      if (!data.results.length) { const cell=body.insertRow().insertCell();cell.colSpan=4;cell.textContent='Aucune vente pour ces filtres.'; }
      $('[data-history-page]').textContent=`Page ${Math.floor(offset/limit)+1} · ${total} vente(s)`;
      $('[data-history-prev]').disabled=offset===0; $('[data-history-next]').disabled=offset+limit>=total;
      notice.textContent='Historique actualisé.'; results.hidden=false;
    } catch (e) {
      if (current !== generation || e.name==='AbortError') return;
      error.textContent=e.message;error.hidden=false;notice.textContent='';
    } finally { if (current===generation) dialog.setAttribute('aria-busy','false'); }
  }
  window.addEventListener('customer-history-open', event => {
    customerID=event.detail.id;offset=0;form.reset();
    $('#customer-history-title').textContent=`Historique · ${event.detail.name}`;
    if (!dialog.open) dialog.showModal();load();
  });
  $('[data-history-close]').onclick=()=>dialog.close();
  dialog.addEventListener('close',()=>invalidate());
  form.addEventListener('change',()=>{offset=0;invalidate('Filtres modifiés : cliquez sur Actualiser.');});
  form.onsubmit=event=>{event.preventDefault();offset=0;load();};
  $('[data-history-prev]').onclick=()=>{offset=Math.max(0,offset-limit);load();};
  $('[data-history-next]').onclick=()=>{if(offset+limit<total){offset+=limit;load();}};
})();
