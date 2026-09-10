(() => {
  'use strict';
  const operations=document.querySelector('#operations'),schemas=document.querySelector('#schemas'),status=document.querySelector('#status'),search=document.querySelector('#search');
  const element=(tag,text)=>{const e=document.createElement(tag);if(text!==undefined)e.textContent=text;return e;};
  const pre=value=>{const p=element('pre');p.append(element('code',typeof value==='string'?value:JSON.stringify(value,null,2)));return p;};
  const items=[];
  function filter(){const q=search.value.toLowerCase().trim();let count=0;for(const item of items){item.node.hidden=!item.text.includes(q);if(!item.node.hidden)count++;}status.textContent=`${count} opération(s) affichée(s) sur ${items.length}.`;}
  search.addEventListener('input',filter);
  async function load(){
    try{
      const response=await fetch('/static/openapi.json',{cache:'no-store'});if(!response.ok)throw new Error(`HTTP ${response.status}`);const spec=await response.json();
      for(const [path,methods] of Object.entries(spec.paths))for(const [method,op] of Object.entries(methods)){
        const node=element('details');node.append(element('summary',`${method.toUpperCase()} ${path}`),element('p',op.description));
        if(op.parameters?.length){node.append(element('h3','Paramètres'));const wrap=element('div');wrap.className='scroll';const table=element('table'),head=element('thead'),hr=element('tr');for(const t of ['Nom','Emplacement','Obligatoire','Description / type']){const th=element('th',t);th.scope='col';hr.append(th);}head.append(hr);table.append(head);const body=element('tbody');for(const p of op.parameters){const row=element('tr');for(const t of [p.name,p.in,p.required?'Oui':'Non',(p.description||'')+' '+JSON.stringify(p.schema)])row.append(element('td',t));body.append(row);}table.append(body);wrap.append(table);node.append(wrap);}
        if(op.requestBody)node.append(element('h3','Corps de la requête'),pre(op.requestBody));
        node.append(element('h3','Réponses'),pre(op.responses));
        for(const sample of op['x-codeSamples']||[])node.append(element('h3',sample.label),pre(sample.source));
        operations.append(node);items.push({node,text:(method+' '+path+' '+op.tags.join(' ')).toLowerCase()});
      }
      for(const [name,schema] of Object.entries(spec.components.schemas)){const node=element('details');node.append(element('summary',name),pre(schema));schemas.append(node);}
      filter();
    }catch(e){status.textContent='Impossible de charger le contrat : '+e.message;status.setAttribute('role','alert');}
  }
  load();
})();
