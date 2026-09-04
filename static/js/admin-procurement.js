(() => {
  "use strict";
  const token = () => sessionStorage.getItem("defta.accessToken") || "";
  const state = {suppliers: [], purchases: [], books: [], libraries: [], isRoot: false};
  const money = (value) => new Intl.NumberFormat("fr-FR", {style: "currency", currency: "XOF", maximumFractionDigits: 0}).format(value || 0);
  async function api(path, options = {}) {
    const headers = new Headers(options.headers || {}); headers.set("Authorization", `Bearer ${token()}`);
    const response = await fetch(path, {...options, headers});
    const type = response.headers.get("content-type") || ""; const payload = type.includes("json") ? await response.json() : null;
    if (!response.ok) throw new Error(payload?.message || `Requête refusée (${response.status})`);
    return payload;
  }
  function cell(row, value, className = "") { const td = row.insertCell(); td.textContent = value || "—"; td.className = className; return td; }
  function button(label, action, id, danger = false) { const b = document.createElement("button"); b.type = "button"; b.className = `row-button${danger ? " danger" : ""}`; b.dataset.action = action; b.dataset.id = id; b.textContent = label; return b; }
  function error(id, reason) { const box = document.querySelector(id); box.textContent = reason.message; box.hidden = false; }
  async function loadSuppliers() {
    const payload = await api("/api/manage/suppliers?limit=100"); state.suppliers = payload.results;
    const body = document.querySelector("#suppliers-body"); body.replaceChildren();
    payload.results.forEach((item) => { const row = body.insertRow(); cell(row,item.name); cell(row,item.contactName); cell(row,item.phone); cell(row,item.status,"pill"); const actions=cell(row,""); actions.className="row-actions"; const list=[button("Modifier","edit",item.id)]; if(item.status==="ACTIVE") list.push(button("Désactiver","disable",item.id,true)); else list.push(button("Réactiver","reactivate",item.id)); actions.replaceChildren(...list); });
    if (!payload.results.length) { const row=body.insertRow(); const td=cell(row,"Aucun fournisseur","empty"); td.colSpan=5; }
  }
  async function loadPurchases() {
    const payload = await api("/api/manage/purchases?limit=100"); state.purchases = payload.results;
    const names = new Map(state.suppliers.map((s) => [s.id,s.name])); const body=document.querySelector("#purchases-body"); body.replaceChildren();
    payload.results.forEach((item) => { const row=body.insertRow(); cell(row,item.reference); cell(row,names.get(item.supplierId)); cell(row,item.lines.reduce((n,l)=>n+l.quantity,0)); cell(row,money(item.totalAmount)); cell(row,item.status,"pill"); const actions=cell(row,""); actions.className="row-actions"; const list=[]; if(item.status==="DRAFT") list.push(button("Modifier","edit-purchase",item.id),button("Réceptionner","receive",item.id),button("Annuler","cancel",item.id,true)); actions.replaceChildren(...list); });
    if (!payload.results.length) { const row=body.insertRow(); const td=cell(row,"Aucun bon d'achat","empty"); td.colSpan=6; }
  }
  async function loadBooks() { state.books=(await api("/api/manage/books?limit=100")).results; }
  function fill(select, items, label) { select.replaceChildren(...items.map((item)=>{const o=document.createElement("option");o.value=item.id;o.textContent=label(item);return o;})); }
  function fillLibraries(form, selected="") { if(!state.isRoot)return; fill(form.elements.libraryId,state.libraries,l=>l.name); form.elements.libraryId.value=selected; }
  function openSupplier(item=null) { const f=document.querySelector("#supplier-form"); f.reset(); ["id","version","name","contactName","phone","email","address"].forEach((n)=>{f.elements[n].value=item?.[n]||"";}); fillLibraries(f,item?.libraryId); document.querySelector("#supplier-error").hidden=true; document.querySelector("#supplier-dialog").showModal(); }
  function updatePurchaseTotal() { let total=0; document.querySelectorAll("#purchase-lines .purchase-line").forEach(row=>{total+=(Number(row.querySelector("[name=quantity]").value)||0)*(Number(row.querySelector("[name=unitCost]").value)||0);}); document.querySelector("#purchase-estimated-total").textContent=money(total); }
  function addPurchaseLine(line=null) { const fragment=document.querySelector("#purchase-line-template").content.cloneNode(true);const row=fragment.querySelector(".purchase-line"),select=row.querySelector("[name=bookId]");fill(select,state.books,b=>b.title);if(line){select.value=String(line.bookId);row.querySelector("[name=quantity]").value=line.quantity;row.querySelector("[name=unitCost]").value=line.unitCost;}row.addEventListener("input",updatePurchaseTotal);row.querySelector(".remove-purchase-line").onclick=()=>{row.remove();updatePurchaseTotal();};document.querySelector("#purchase-lines").append(row);updatePurchaseTotal();}
  function openPurchase(item=null) { const f=document.querySelector("#purchase-form"); f.reset(); fillLibraries(f,item?.libraryId); fill(f.elements.supplierId,state.suppliers.filter(s=>s.status==="ACTIVE"),s=>s.name); f.elements.id.value=item?.id||""; f.elements.version.value=item?.version||""; if(item)f.elements.supplierId.value=item.supplierId;document.querySelector("#purchase-lines").replaceChildren();(item?.lines?.length?item.lines:[null]).forEach(addPurchaseLine);document.querySelector("#purchase-error").hidden=true;document.querySelector("#purchase-dialog").showModal(); }
  async function init() {
    if (!document.querySelector("#suppliers-body")) return;
    try { const me=await api("/api/auth/me"); state.isRoot=me.role==="SUPER_ADMIN_ROOT"; if(state.isRoot){const owners=await api("/api/admin/owners?status=ACTIVE&libraryStatus=ACTIVE&limit=100");state.libraries=owners.results.map(o=>o.library);document.querySelectorAll(".procurement-root").forEach(x=>x.hidden=false);} await Promise.all([loadSuppliers(),loadBooks()]); await loadPurchases(); } catch (_) { return; }
    document.querySelector("#add-supplier-button").onclick=()=>openSupplier(); document.querySelector("#add-purchase-button").onclick=()=>openPurchase();
    document.querySelector("#add-purchase-line-button").onclick=()=>addPurchaseLine();
    document.querySelectorAll("[data-procurement-close]").forEach(b=>b.onclick=()=>document.querySelector(`#${b.dataset.procurementClose}`).close());
    document.querySelector("#supplier-form").onsubmit=async(e)=>{e.preventDefault();const f=e.currentTarget,id=f.elements.id.value,p={name:f.elements.name.value,contactName:f.elements.contactName.value,phone:f.elements.phone.value,email:f.elements.email.value,address:f.elements.address.value};if(state.isRoot)p.libraryId=f.elements.libraryId.value;if(id)p.version=Number(f.elements.version.value);try{await api(id?`/api/manage/suppliers/${id}`:"/api/manage/suppliers",{method:id?"PUT":"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(p)});document.querySelector("#supplier-dialog").close();await loadSuppliers();await loadPurchases();}catch(x){error("#supplier-error",x);}};
    document.querySelector("#purchase-form").onsubmit=async(e)=>{e.preventDefault();const f=e.currentTarget,id=f.elements.id.value,lines=Array.from(document.querySelectorAll("#purchase-lines .purchase-line")).map(row=>({bookId:Number(row.querySelector("[name=bookId]").value),quantity:Number(row.querySelector("[name=quantity]").value),unitCost:Number(row.querySelector("[name=unitCost]").value)})),p={supplierId:f.elements.supplierId.value,lines};const ids=lines.map(l=>l.bookId);if(!lines.length)return error("#purchase-error",new Error("Ajoutez au moins une ligne."));if(new Set(ids).size!==ids.length)return error("#purchase-error",new Error("Un livre ne peut apparaître qu'une seule fois."));if(state.isRoot)p.libraryId=f.elements.libraryId.value;if(id)p.version=Number(f.elements.version.value);try{await api(id?`/api/manage/purchases/${id}`:"/api/manage/purchases",{method:id?"PUT":"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(p)});document.querySelector("#purchase-dialog").close();await loadPurchases();}catch(x){error("#purchase-error",x);}};
    document.querySelector("#suppliers-body").onclick=async(e)=>{const b=e.target.closest("button");if(!b)return;const s=state.suppliers.find(x=>x.id===b.dataset.id);if(b.dataset.action==="edit")return openSupplier(s);if(!confirm(`${b.textContent} ${s.name} ?`))return;await api(`/api/manage/suppliers/${s.id}${b.dataset.action==="reactivate"?"/reactivate":""}?version=${s.version}`,{method:b.dataset.action==="reactivate"?"POST":"DELETE"});await loadSuppliers();};
    document.querySelector("#purchases-body").onclick=async(e)=>{const b=e.target.closest("button");if(!b)return;const p=state.purchases.find(x=>x.id===b.dataset.id);if(b.dataset.action==="edit-purchase")return openPurchase(p);if(!confirm(`${b.textContent} ${p.reference} ?`))return;await api(`/api/manage/purchases/${p.id}/${b.dataset.action}`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({version:p.version})});await loadPurchases();};
  }
  window.addEventListener("DOMContentLoaded", init);
})();
