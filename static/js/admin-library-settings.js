(() => {
  'use strict';
  async function api(url,options={}){
    const token=sessionStorage.getItem('defta.accessToken');if(!token)throw new Error('Connectez-vous pour accéder aux paramètres.');
    const response=await fetch(url,{...options,cache:'no-store',headers:{Authorization:`Bearer ${token}`,'Content-Type':'application/json'}});
    if(!response.ok)throw new Error(response.status===409?'Les paramètres ont changé : rechargez-les avant de réessayer.':response.status===400?'Paramètres invalides : vérifiez les champs et le logo.':`Paramètres indisponibles (HTTP ${response.status}).`);
    return response.status===204?null:response.json();
  }
  // Resolve the document's library, never the current settings form selection.
  window.deftaPrintSettings=async(selector,libraryID)=>{
    const receipt=document.querySelector(selector);receipt.querySelectorAll('[data-print-settings]').forEach(e=>e.remove());
    if(!libraryID)throw new Error('Librairie du document inconnue.');
    const v=await api('/api/manage/library-settings?'+new URLSearchParams({libraryId:libraryID}));
    const header=document.createElement('div');header.dataset.printSettings='';
    if(v.logoData){const img=document.createElement('img');img.src=v.logoData;img.alt='Logo';img.style.maxWidth='120px';img.style.maxHeight='100px';await img.decode();header.append(img);}
    const name=document.createElement('h2');name.textContent=v.name;header.append(name);
    const contact=document.createElement('p');contact.style.whiteSpace='pre-wrap';contact.textContent=[v.address,v.phone,v.email].filter(Boolean).join('\n');header.append(contact);receipt.prepend(header);
    const footer=document.createElement('p');footer.dataset.printSettings='';footer.style.whiteSpace='pre-wrap';footer.textContent=v.printFooter;receipt.append(footer);
  };
  const panel=document.querySelector('#library-settings-panel');if(!panel)return;
  const select=panel.querySelector('[data-select]'),form=panel.querySelector('[data-settings]'),notice=panel.querySelector('[data-notice]'),error=panel.querySelector('[data-error]');
  let root=false,current=null,busy=false;
  const keys=['address','phone','email','defaultLowStockThreshold','printFooter'];
  function lock(value){busy=value;panel.setAttribute('aria-busy',String(value));panel.querySelectorAll('input,textarea,select,button').forEach(e=>{e.disabled=value;});}
  function render(v){current=v;form.reset();for(const key of keys)form.elements[key].value=v[key];panel.querySelector('[data-name]').textContent=v.name;const img=panel.querySelector('[data-logo]');img.hidden=!v.logoData;if(v.logoData)img.src=v.logoData;else img.removeAttribute('src');form.hidden=false;}
  function url(){return '/api/manage/library-settings'+(root?'?'+new URLSearchParams({libraryId:select.elements.libraryId.value}):'');}
  async function load(){if(busy)return;lock(true);form.hidden=true;current=null;error.hidden=true;notice.textContent='Chargement…';try{if(root&&!select.elements.libraryId.value){notice.textContent='Choisissez une librairie.';return;}render(await api(url()));notice.textContent='Paramètres chargés.';}catch(e){error.textContent=e.message;error.hidden=false;notice.textContent='';}finally{lock(false);}}
  select.onsubmit=e=>{e.preventDefault();load();};select.onchange=()=>{form.hidden=true;current=null;notice.textContent='Chargez les paramètres de la librairie sélectionnée.';};
  form.onsubmit=async e=>{
    e.preventDefault();if(busy||!current)return;lock(true);error.hidden=true;notice.textContent='Enregistrement…';
    try{
      const payload={currency:'XOF',version:current.version,logoData:current.logoData};for(const key of keys)payload[key]=key==='defaultLowStockThreshold'?Number(form.elements[key].value):form.elements[key].value;
      const file=form.elements.logo.files[0];
      if(form.elements.removeLogo.checked)payload.logoData='';else if(file){if(file.size>131072||!['image/png','image/jpeg'].includes(file.type))throw new Error('Logo PNG/JPEG de 128 Kio maximum requis.');payload.logoData=await new Promise((resolve,reject)=>{const r=new FileReader();r.onload=()=>resolve(r.result);r.onerror=()=>reject(new Error('Lecture du logo impossible.'));r.readAsDataURL(file);});}
      await api(url(),{method:'PUT',body:JSON.stringify(payload)});
      // Do not suggest retrying a successful save if the following read fails.
      current=null;form.hidden=true;notice.textContent='Paramètres enregistrés.';
      try{render(await api(url()));}catch(e){notice.textContent='Paramètres enregistrés. Rechargez-les pour poursuivre.';}
    }catch(e){error.textContent=e.message;error.hidden=false;notice.textContent='';}finally{lock(false);}
  };
  async function init(){lock(true);try{const me=await api('/api/auth/me');root=me.role==='SUPER_ADMIN_ROOT';if(!root&&me.role!=='OWNER_LIBRARY')throw new Error('Profil non autorisé.');select.querySelector('[data-library]').hidden=!root;if(root){let offset=0;const options=[];while(true){const page=await api(`/api/admin/owners?offset=${offset}&limit=100`);for(const o of page.results)if(o.library)options.push(new Option(o.library.name,o.library.id));offset+=page.results.length;if(!page.results.length||offset>=page.total)break;}select.elements.libraryId.append(...options);}lock(false);await load();}catch(e){error.textContent=e.message;error.hidden=false;lock(false);}}
  init();
})();
