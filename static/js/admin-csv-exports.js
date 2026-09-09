(() => {
  'use strict';
  const panel=document.querySelector('#csv-exports-panel');if(!panel)return;
  const form=panel.querySelector('form'),notice=panel.querySelector('[data-notice]'),error=panel.querySelector('[data-error]');
  let root=false,ready=false,busy=false;
  const statuses={stocks:[['OUT_OF_STOCK','Rupture'],['LOW_STOCK','Stock faible'],['IN_STOCK','Stock suffisant']],sales:[['DRAFT','Brouillon'],['CONFIRMED','Confirmée'],['CANCELLED','Annulée']],purchases:[['DRAFT','Brouillon'],['RECEIVED','Réceptionné'],['CANCELLED','Annulé']],suppliers:[['ACTIVE','Actif'],['DISABLED','Désactivé']],audit:[]};
  function update(){
    const kind=form.elements.kind.value,audit=kind==='audit';
    form.elements.status.replaceChildren(new Option('Tous',''),...statuses[kind].map(([v,l])=>new Option(l,v)));
    form.elements.status.disabled=audit;
    panel.querySelector('[data-library]').hidden=!root||audit;
    panel.querySelectorAll('[data-audit]').forEach(e=>{e.hidden=!audit;});
    panel.querySelector('[data-hint]').textContent=audit ? (root?'Audit global. ':'Vos propres événements uniquement. ')+'Dates des événements UTC ; recherche sur identifiant de ressource. Les instantanés JSON et adresses IP ne sont pas exportés.' : `Dates UTC de ${kind==='stocks'?'dernière modification du stock':'création'} ; recherche sur ${kind==='stocks'?'titre':kind==='suppliers'?'nom':'référence'}. Les montants des ventes et achats sont bruts, avant retours. Un coût inconnu reste vide.`;
  }
  async function request(url){
    const token=sessionStorage.getItem('defta.accessToken');if(!token)throw new Error('Connectez-vous pour exporter.');
    const response=await fetch(url,{cache:'no-store',headers:{Authorization:`Bearer ${token}`}});
    if(!response.ok)throw new Error(response.status===422?'Plus de 10 000 lignes : affinez les filtres.':response.status===401?'Session expirée : reconnectez-vous.':`Export indisponible (HTTP ${response.status}).`);
    return response;
  }
  form.elements.kind.onchange=()=>{update();notice.textContent='';error.hidden=true;};
  form.onsubmit=async e=>{
    e.preventDefault();if(busy||!ready)return;busy=true;error.hidden=true;notice.textContent='Préparation du fichier…';panel.setAttribute('aria-busy','true');
    const controls=[...form.querySelectorAll('input,select,button')];controls.forEach(c=>{c.disabled=true;});
    try{
      const kind=form.elements.kind.value,q=new URLSearchParams();
      if(kind!=='audit' && root){if(!form.elements.libraryId.value)throw new Error('Choisissez une librairie.');q.set('libraryId',form.elements.libraryId.value);}
      for(const key of (kind==='audit'?['q','action','resourceType']:['q','status']))if(form.elements[key].value)q.set(key,form.elements[key].value);
      const from=form.elements.from.value,to=form.elements.to.value;if(from&&to&&from>to)throw new Error('Période invalide.');
      if(from)q.set('from',new Date(from+'T00:00:00Z').toISOString());
      if(to){const end=new Date(to+'T00:00:00Z');end.setUTCDate(end.getUTCDate()+1);q.set('to',end.toISOString());}
      const response=await request(`/api/manage/exports/${kind}?${q}`);
      if(!(response.headers.get('content-type')||'').startsWith('text/csv'))throw new Error('Réponse inattendue du serveur.');
      const blob=await response.blob(),url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=kind+'.csv';document.body.append(a);a.click();a.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);
      notice.textContent=`Téléchargement lancé : ${response.headers.get('X-Export-Row-Count')||'0'} ligne(s).`;
    }catch(e){error.textContent=e.message;error.hidden=false;notice.textContent='';}
    finally{busy=false;controls.forEach(c=>{c.disabled=false;});form.elements.status.disabled=form.elements.kind.value==='audit';panel.setAttribute('aria-busy','false');}
  };
  async function init(){
    form.querySelector('button').disabled=true;update();
    try{
      const me=await(await request('/api/auth/me')).json();root=me.role==='SUPER_ADMIN_ROOT';if(!root&&me.role!=='OWNER_LIBRARY')throw new Error('Profil non autorisé.');
      if(root){const options=[];let offset=0;while(true){const page=await(await request(`/api/admin/owners?offset=${offset}&limit=100`)).json();for(const o of page.results)if(o.library)options.push(new Option(o.library.name,o.library.id));offset+=page.results.length;if(!page.results.length||offset>=page.total)break;}form.elements.libraryId.append(...options);}
      ready=true;update();form.querySelector('button').disabled=false;
    }catch(e){error.textContent=e.message;error.hidden=false;}
  }
  init();
})();
