(() => {
  "use strict";
  const root = document.querySelector('#supplier-returns-panel');
  if (!root) return;
  root.innerHTML = `
    <div class="panel-heading purchase-heading"><h2>Retours fournisseurs</h2>
      <button class="button primary" data-new type="button">Nouveau retour</button></div>
    <form class="filters entity-form" data-filters>
      <label>État<select name="status"><option value="">Tous</option><option>DRAFT</option><option>SHIPPED</option><option>CANCELLED</option></select></label>
      <label data-root hidden>Librairie<select name="libraryId"></select></label>
      <button class="button ghost">Actualiser</button>
    </form>
    <p class="alert" role="alert" data-error hidden></p>
    <p class="scope-note">Expédier retire les quantités du stock. Un retour expédié reste conservé dans l’historique.</p>
    <div class="table-wrap"><table><thead><tr><th scope="col">Référence</th><th scope="col">Motif</th><th scope="col">Total</th><th scope="col">État</th><th scope="col">Actions</th></tr></thead><tbody></tbody></table></div>
    <div class="pagination"><button class="button ghost" data-prev>Précédent</button><span data-page></span><button class="button ghost" data-next>Suivant</button></div>
    <dialog class="modal sale-modal" aria-labelledby="supplier-return-form-title"><form class="entity-form" data-editor>
      <h2 data-title id="supplier-return-form-title">Nouveau retour fournisseur</h2>
      <div class="form-grid">
        <label data-root hidden>Librairie<select name="libraryId"></select></label>
        <label class="full">Achat réceptionné<select name="purchaseId" required></select></label>
        <label class="full">Motif<textarea name="reason" minlength="3" maxlength="1000" required></textarea></label>
        <label class="full">Référence fournisseur<input name="supplierReference" maxlength="160"></label>
      </div>
      <p class="hint" data-quantity-hint>Quantité zéro : article exclu du retour. Le serveur contrôle les quantités déjà réservées.</p>
      <div data-lines class="form-grid"></div>
      <section data-costs hidden aria-label="Valorisation du retour expédié">
        <h3>Valorisation à l’expédition</h3>
        <p class="hint">Montants en F CFA. Écart = montant fournisseur − valeur du stock sorti. Un écart positif signifie que le montant fournisseur est supérieur à la valeur du stock sorti.</p>
        <div class="table-wrap" tabindex="0" role="region" aria-label="Coûts par livre, défilement horizontal">
          <table><thead><tr><th scope="col">Livre</th><th scope="col">Quantité</th><th scope="col">Montant fournisseur</th><th scope="col">CMP figé</th><th scope="col">Valeur du stock sorti</th><th scope="col">Écart</th></tr></thead><tbody data-cost-rows></tbody></table>
        </div>
        <p class="hint">« Indisponible » indique un coût inconnu. Ces montants historiques ne représentent pas un remboursement reçu.</p>
      </section>
      <p class="alert" role="alert" data-form-error hidden></p>
      <div class="modal-actions"><button type="button" class="button ghost" data-close>Fermer</button><button class="button primary" data-save>Enregistrer</button></div>
    </form></dialog>`;
  const $ = (s) => root.querySelector(s);
  const form = $('[data-editor]'), filters = $('[data-filters]'), dialog = $('dialog');
  let isRoot = false, offset = 0, editing = null, busy = false, purchaseGeneration = 0;
  const money = n => new Intl.NumberFormat('fr-FR', {maximumFractionDigits:2}).format(n);
  const states = {DRAFT:'Brouillon', SHIPPED:'Expédié', CANCELLED:'Annulé'};
  async function api(path, options = {}) {
    const headers = new Headers(options.headers);
    headers.set('Authorization', `Bearer ${sessionStorage.getItem('defta.accessToken') || ''}`);
    const response = await fetch(path, {...options, headers});
    const data = (response.headers.get('content-type') || '').includes('json') ? await response.json() : null;
    if (!response.ok) throw new Error(response.status === 401 ? 'Session expirée : reconnectez-vous.' : data?.message || `Erreur HTTP ${response.status}`);
    return data;
  }
  function error(e) { const box = dialog.open ? $('[data-form-error]') : $('[data-error]'); box.textContent = e.message; box.hidden = false; }
  async function run(task) {
    if (busy) return;
    busy = true;
    root.querySelectorAll('button, select, input, textarea').forEach(b => { b.dataset.disabled = String(b.disabled); b.disabled = true; });
    try { await task(); } catch(e) { error(e); }
    finally {
      busy = false;
      root.querySelectorAll('button, select, input, textarea').forEach(b => { b.disabled = b.dataset.disabled === 'true'; });
      if (dialog.open) {
        form.elements.purchaseId.disabled=Boolean(editing);
        form.elements.libraryId.disabled=Boolean(editing);
        const readonly=Boolean(editing && editing.status!=='DRAFT');
        form.elements.reason.disabled=readonly;form.elements.supplierReference.disabled=readonly;
        root.querySelectorAll('[data-line]').forEach(i=>i.disabled=readonly);
      }
      updatePager();
    }
  }
  let total = 0;
  function updatePager() { $('[data-prev]').disabled = busy || offset === 0; $('[data-next]').disabled = busy || offset + 10 >= total; }
  function option(value, text) { return new Option(text, value); }
  function renderLines(purchase, selected = null, readonly = false) {
    $('[data-lines]').replaceChildren();
    for (const line of purchase.lines) {
      const label = document.createElement('label');
      label.textContent = `${line.title} — ${money(line.unitCost)} / exemplaire`;
      const input = document.createElement('input'); input.type='number'; input.min='0'; input.step='1'; input.max=String(line.quantity);
      input.dataset.line = line.purchaseLineId || line.id;
      input.value = String(selected ? selected.find(l => l.purchaseLineId === input.dataset.line)?.quantity || 0 : 0);
      input.disabled=readonly; label.append(input); $('[data-lines]').append(label);
    }
  }
  function renderCosts(value) {
    const rows = $('[data-cost-rows]');
    rows.replaceChildren();
    const amount = n => Number.isFinite(n) ? `${money(n)} F CFA` : 'Indisponible';
    for (const line of value.lines) {
      const row = rows.insertRow();
      const variance = Number.isFinite(line.costVariance)
        ? `${line.costVariance > 0 ? '+' : ''}${amount(line.costVariance)}` : 'Indisponible';
      for (const text of [line.title, line.quantity, amount(line.lineTotal), amount(line.unitCostSnapshot), amount(line.inventoryCost), variance]) {
        row.insertCell().textContent = text;
      }
    }
    $('[data-costs]').hidden = false;
  }
  async function loadPurchases() {
    const generation = ++purchaseGeneration;
    form.elements.purchaseId.replaceChildren(option('', 'Choisir un achat'));
    $('[data-lines]').replaceChildren();
    const library = isRoot ? form.elements.libraryId.value : '';
    if (isRoot && !library) return;
    let start = 0;
    do {
      const q = new URLSearchParams({status:'RECEIVED',offset:String(start),limit:'100'});
      if (library) q.set('libraryId', library);
      const data = await api(`/api/manage/purchases?${q}`);
      if (generation !== purchaseGeneration) return;
      for (const p of data.results) form.elements.purchaseId.append(option(p.id,p.reference));
      start += data.results.length;
      if (!data.results.length || start >= data.total) break;
    } while (true);
  }
  async function list() {
    const q = new URLSearchParams({offset:String(offset),limit:'10'});
    if (filters.elements.status.value) q.set('status',filters.elements.status.value);
    if (isRoot && filters.elements.libraryId.value) q.set('libraryId',filters.elements.libraryId.value);
    const data = await api(`/api/manage/supplier-returns?${q}`); total=data.total;
    $('tbody').replaceChildren();
    for (const item of data.results) {
      const row = $('tbody').insertRow();
      for (const text of [item.reference,item.reason,money(item.totalAmount),states[item.status] || item.status]) row.insertCell().textContent=text;
      const cell=row.insertCell();
      for (const [action,label] of (item.status==='DRAFT' ? [['edit','Modifier'],['ship','Expédier'],['cancel','Annuler']] : [['view','Détails']])) {
        const b=document.createElement('button'); b.type='button';b.className='row-button';b.textContent=label;
        b.onclick=()=>run(async()=>{
          if (action==='edit' || action==='view') return open(item.id);
          if (!window.confirm(`${label} ${item.reference} ?${action==='ship' ? ' Les quantités seront retirées du stock.' : ''}`)) return;
          await api(`/api/manage/supplier-returns/${encodeURIComponent(item.id)}/${action}`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({version:item.version})});
          await list();
        }); cell.append(b);
      }
    }
    if (!data.results.length) { const c=$('tbody').insertRow().insertCell();c.colSpan=5;c.textContent='Aucun retour fournisseur.'; }
    $('[data-page]').textContent=`Page ${Math.floor(offset/10)+1} — ${total} retour(s)`; updatePager();
    $('[data-error]').hidden=true;
  }
  async function open(id = '') {
    $('[data-costs]').hidden=true; $('[data-cost-rows]').replaceChildren();
    $('[data-quantity-hint]').hidden=false;
    editing=null; form.reset(); $('[data-lines]').replaceChildren(); $('[data-form-error]').hidden=true;
    form.elements.purchaseId.disabled=false; form.elements.libraryId.disabled=false;
    form.elements.reason.disabled=false; form.elements.supplierReference.disabled=false; $('[data-save]').hidden=false;
    $('[data-title]').textContent='Nouveau retour fournisseur';
    if (id) {
      editing=await api(`/api/manage/supplier-returns/${encodeURIComponent(id)}`);
      const readonly=editing.status!=='DRAFT';
      $('[data-title]').textContent=editing.reference;
      form.elements.libraryId.value=editing.libraryId;form.elements.libraryId.disabled=true;
      form.elements.purchaseId.replaceChildren(option(editing.purchaseId,editing.purchaseId));form.elements.purchaseId.disabled=true;
      form.elements.reason.value=editing.reason;form.elements.supplierReference.value=editing.supplierReference || '';
      form.elements.reason.disabled=readonly;form.elements.supplierReference.disabled=readonly; $('[data-save]').hidden=readonly;
      $('[data-quantity-hint]').hidden=readonly;
      if (editing.status === 'SHIPPED') renderCosts(editing);
      else if (readonly) renderLines(editing, editing.lines, true);
      else renderLines(await api(`/api/manage/purchases/${encodeURIComponent(editing.purchaseId)}`),editing.lines);
    } else {
      form.elements.libraryId.value=filters.elements.libraryId.value || form.elements.libraryId.options[0]?.value || '';
      await loadPurchases();
    }
    dialog.showModal();
  }
  $('[data-close]').onclick=()=>dialog.close();
  $('[data-new]').onclick=()=>run(()=>open());
  filters.onsubmit=e=>{e.preventDefault();run(async()=>{offset=0;await list();});};
  $('[data-prev]').onclick=()=>run(async()=>{offset=Math.max(0,offset-10);await list();});
  $('[data-next]').onclick=()=>run(async()=>{offset+=10;await list();});
  form.elements.libraryId.onchange=()=>run(loadPurchases);
  form.elements.purchaseId.onchange=()=>run(async()=>{
    $('[data-lines]').replaceChildren();
    const id=form.elements.purchaseId.value;
    if (id) {const p=await api(`/api/manage/purchases/${encodeURIComponent(id)}`);if(form.elements.purchaseId.value===id)renderLines(p);}
  });
  form.onsubmit=e=>{e.preventDefault();run(async()=>{
    const lines=[...root.querySelectorAll('[data-line]')].map(i=>({purchaseLineId:i.dataset.line,quantity:Number(i.value)})).filter(l=>l.quantity>0);
    if (!lines.length || lines.some(l=>!Number.isSafeInteger(l.quantity))) throw new Error('Choisissez au moins une quantité entière positive.');
    const data={purchaseId:form.elements.purchaseId.value,reason:form.elements.reason.value.trim(),supplierReference:form.elements.supplierReference.value.trim(),lines};
    if (isRoot && !editing) data.libraryId=form.elements.libraryId.value;
    if (editing) data.version=editing.version;
    await api(`/api/manage/supplier-returns${editing ? '/'+encodeURIComponent(editing.id) : ''}`,{method:editing?'PUT':'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(data)});
    dialog.close();await list();
  });};
  run(async()=>{
    const me=await api('/api/auth/me'); isRoot=me.role==='SUPER_ADMIN_ROOT';
    if (isRoot) {
      root.querySelectorAll('[data-root]').forEach(l=>l.hidden=false);
      filters.elements.libraryId.append(option('','Toutes'));
      let start=0;
      do {
        const data=await api(`/api/admin/owners?offset=${start}&limit=100`);
        for(const owner of data.results) if(owner.library){const l=owner.library;filters.elements.libraryId.append(option(l.id,l.name));if(l.status==='ACTIVE')form.elements.libraryId.append(option(l.id,l.name));}
        start+=data.results.length;if(!data.results.length || start>=data.total)break;
      } while(true);
    }
    await list();
  });
})();
