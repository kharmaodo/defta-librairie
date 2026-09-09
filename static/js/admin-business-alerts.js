(() => {
  'use strict';
  const panel=document.querySelector('#business-alerts-panel');
  if (!panel) return;
  const form=panel.querySelector('form'), output=panel.querySelector('[data-results]');
  const notice=panel.querySelector('[data-notice]'), error=panel.querySelector('[data-error]');
  const prev=panel.querySelector('[data-prev]'), next=panel.querySelector('[data-next]');
  const labels={OUT_OF_STOCK:'Rupture',LOW_STOCK:'Stock faible',DRAFT_PURCHASE:'Achat en brouillon',DISABLED_SUPPLIER:'Fournisseur désactivé'};
  let offset=0,total=0,busy=false;
  async function api(url) {
    const token=sessionStorage.getItem('defta.accessToken');
    if (!token) throw new Error('Connectez-vous pour consulter les alertes.');
    const response=await fetch(url,{cache:'no-store',headers:{Authorization:`Bearer ${token}`}});
    if (!response.ok) throw new Error(response.status===401 ? 'Session expirée : reconnectez-vous.' : `Alertes indisponibles (HTTP ${response.status}).`);
    return response.json();
  }
  async function load() {
    if (busy) return;
    busy=true;output.hidden=true;error.hidden=true;notice.textContent='Chargement…';panel.setAttribute('aria-busy','true');
    const controls=[...form.querySelectorAll('input,select,button')];controls.forEach(c=>{c.disabled=true;});
    try {
      const me=await api('/api/auth/me'), root=me.role==='SUPER_ADMIN_ROOT';
      if (!root && me.role!=='OWNER_LIBRARY') throw new Error('Profil non autorisé.');
      form.querySelector('[data-library]').hidden=!root;
      if (root && form.elements.libraryId.options.length===1) {
        const options=[];let start=0;
        while (true) {
          const owners=await api(`/api/admin/owners?offset=${start}&limit=100`);
          for (const owner of owners.results) if (owner.library) options.push(new Option(owner.library.name,owner.library.id));
          start+=owners.results.length;if (!owners.results.length || start>=owners.total) break;
        }
        form.elements.libraryId.append(...options);
      }
      if (root && !form.elements.libraryId.value) {notice.textContent='Choisissez une librairie puis actualisez.';return;}
      const q=new URLSearchParams({offset:String(offset),limit:'10',kind:form.elements.kind.value});
      if (root) q.set('libraryId',form.elements.libraryId.value);
      const data=await api(`/api/manage/alerts?${q}`);
      if (!Array.isArray(data.results) || !Number.isSafeInteger(data.total) || data.total<0) throw new Error('Réponse inattendue du serveur.');
      total=data.total;
      const body=panel.querySelector('tbody');body.replaceChildren();
      for (const alert of data.results) {const row=body.insertRow();for (const value of [labels[alert.kind] || alert.kind,alert.label,alert.detail]) row.insertCell().textContent=value;}
      if (!data.results.length) {const cell=body.insertRow().insertCell();cell.colSpan=3;cell.textContent='Aucune alerte pour ces filtres.';}
      prev.disabled=offset===0;next.disabled=offset+10>=total;
      panel.querySelector('[data-page]').textContent=`Page ${Math.floor(offset/10)+1} · ${total} alerte(s)`;
      notice.textContent='Situation actualisée à '+new Date().toLocaleTimeString('fr-FR');output.hidden=false;
    } catch(e) {error.textContent=e.message;error.hidden=false;notice.textContent='';}
    finally {busy=false;controls.forEach(c=>{c.disabled=false;});panel.setAttribute('aria-busy','false');}
  }
  form.addEventListener('change',()=>{offset=0;output.hidden=true;notice.textContent='Filtres modifiés : actualisez les alertes.';});
  form.onsubmit=e=>{e.preventDefault();offset=0;load();};
  prev.onclick=()=>{if(!busy){offset=Math.max(0,offset-10);load();}};
  next.onclick=()=>{if(!busy && offset+10<total){offset+=10;load();}};
  load();
})();
