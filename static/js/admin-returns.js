(() => {
  "use strict";

  const state = {isRoot: false, libraries: [], returns: [], offset: 0, limit: 10, total: 0, current: null, balance: null, settlements: []};
  const token = () => sessionStorage.getItem("defta.accessToken") || "";
  const money = (value) => new Intl.NumberFormat("fr-FR", {style: "currency", currency: "XOF", maximumFractionDigits: 2}).format(value || 0);

  async function api(path, options = {}) {
    const headers = new Headers(options.headers || {});
    headers.set("Authorization", `Bearer ${token()}`);
    const response = await fetch(path, {...options, headers});
    const type = response.headers.get("content-type") || "";
    const payload = type.includes("json") ? await response.json() : null;
    if (!response.ok) throw new Error(payload?.message || `Requête refusée (${response.status})`);
    return payload;
  }

  function showError(selector, error) { const box = document.querySelector(selector); box.textContent = error.message || "Une erreur est survenue."; box.hidden = false; }
  function cell(row, value, className = "") { const item = row.insertCell(); item.textContent = value ?? "—"; item.className = className; return item; }
  function action(label, name, id, danger = false) { const button = document.createElement("button"); button.type = "button"; button.textContent = label; button.dataset.action = name; button.dataset.id = id; button.className = `row-button${danger ? " danger" : ""}`; return button; }
  function fill(select, items, label) { select.replaceChildren(...items.map((item) => { const option = document.createElement("option"); option.value = item.id; option.textContent = label(item); return option; })); }

  async function loadReturns() {
    const form = document.querySelector("#return-filters");
    const query = new URLSearchParams({offset: String(state.offset), limit: String(state.limit)});
    for (const name of ["status", "libraryId", "from", "to"]) { const value = form.elements[name]?.value?.trim(); if (value) query.set(name, value); }
    const payload = await api(`/api/manage/customer-returns?${query}`);
    state.returns = payload.results; state.total = payload.total;
    const body = document.querySelector("#returns-body"); body.replaceChildren();
    payload.results.forEach((item) => {
      const row = body.insertRow();
      cell(row, item.reference); cell(row, item.saleReference || item.saleId);
      cell(row, item.resolution === "CREDIT_NOTE" ? "Avoir" : "Remboursement"); cell(row, money(item.totalAmount));
      cell(row, item.status === "COMPLETED" ? "Consulter" : "—");
      cell(row, {DRAFT: "Brouillon", COMPLETED: "Finalisé", CANCELLED: "Annulé"}[item.status] || item.status, `pill return-${item.status.toLowerCase()}`);
      const actions = cell(row, ""); actions.className = "row-actions";
      if (item.status === "COMPLETED") actions.replaceChildren(action("Règlements", "settlements", item.id));
    });
    if (!payload.results.length) { const row = body.insertRow(); const empty = cell(row, "Aucun retour client", "empty"); empty.colSpan = 7; }
    const page = Math.floor(state.offset / state.limit) + 1, pages = Math.max(1, Math.ceil(state.total / state.limit));
    document.querySelector("#returns-page-label").textContent = `Page ${page} sur ${pages}`;
    document.querySelector("#returns-previous").disabled = state.offset === 0;
    document.querySelector("#returns-next").disabled = state.offset + state.limit >= state.total;
  }

  async function loadSettlementDetails(returnItem) {
    state.current = returnItem;
    const [balance, settlements] = await Promise.all([
      api(`/api/manage/customer-returns/${returnItem.id}/settlement-balance`),
      api(`/api/manage/customer-returns/${returnItem.id}/settlements?offset=0&limit=100`)
    ]);
    state.balance = balance; state.settlements = settlements.results;
    document.querySelector("#return-detail-title").textContent = `${returnItem.reference} · ${returnItem.resolution === "CREDIT_NOTE" ? "Avoir" : "Remboursement"}`;
    const summary = document.querySelector("#return-balance");
    summary.querySelector("[data-return-total]").textContent = money(balance.totalAmount);
    summary.querySelector("[data-return-settled]").textContent = money(balance.settledAmount);
    summary.querySelector("[data-return-remaining]").textContent = money(balance.remainingAmount);
    summary.querySelector("[data-return-balance-status]").textContent = {PENDING: "En attente", PARTIALLY_SETTLED: "Partiel", SETTLED: "Soldé"}[balance.settlementStatus] || balance.settlementStatus;
    document.querySelector("#add-return-settlement-button").disabled = balance.remainingAmount <= 0; summary.hidden = false;
    const body = document.querySelector("#return-settlements-body"); body.replaceChildren();
    settlements.results.forEach((item) => {
      const row = body.insertRow(); cell(row, new Date(item.createdAt).toLocaleString("fr-FR"));
      cell(row, {CASH: "Espèces", MOBILE_MONEY: "Mobile money", CARD: "Carte", CREDIT_NOTE: "Avoir"}[item.method] || item.method);
      cell(row, money(item.amount)); cell(row, item.externalReference || "—"); cell(row, item.status === "ISSUED" ? "Émis" : "Annulé", `pill payment-${item.status === "ISSUED" ? "recorded" : "voided"}`);
      const actions = cell(row, ""); actions.className = "row-actions"; if (item.status === "ISSUED") actions.replaceChildren(action("Annuler", "void", item.id, true));
    });
    if (!settlements.results.length) { const row = body.insertRow(); const empty = cell(row, "Aucun règlement", "empty"); empty.colSpan = 6; }
    const dialog = document.querySelector("#return-detail-dialog");
    if (!dialog.open) dialog.showModal();
  }

  function openSettlement() {
    const form = document.querySelector("#return-settlement-form"); form.reset();
    form.elements.method.value = state.current.resolution === "CREDIT_NOTE" ? "CREDIT_NOTE" : "CASH";
    [...form.elements.method.options].forEach((option) => { option.disabled = state.current.resolution === "CREDIT_NOTE" ? option.value !== "CREDIT_NOTE" : option.value === "CREDIT_NOTE"; });
    form.elements.amount.value = state.balance.remainingAmount;
    document.querySelector("#return-settlement-error").hidden = true;
    document.querySelector("#return-settlement-dialog").showModal();
  }

  async function init() {
    if (!document.querySelector("#returns-body")) return;
    try {
      const me = await api("/api/auth/me"); state.isRoot = me.role === "SUPER_ADMIN_ROOT";
      if (state.isRoot) { const owners = await api("/api/admin/owners?status=ACTIVE&libraryStatus=ACTIVE&limit=100"); state.libraries = owners.results.map((owner) => owner.library); document.querySelector(".return-root").hidden = false; fill(document.querySelector("#return-filters [name=libraryId]"), [{id: "", name: "Toutes"}, ...state.libraries], (library) => library.name); }
      await loadReturns();
    } catch (error) { showError("#return-error", error); return; }
    document.querySelector("#return-filters").onsubmit = async (event) => { event.preventDefault(); state.offset = 0; try { await loadReturns(); } catch (error) { showError("#return-error", error); } };
    document.querySelector("#returns-previous").onclick = async () => { state.offset = Math.max(0, state.offset - state.limit); await loadReturns(); };
    document.querySelector("#returns-next").onclick = async () => { state.offset += state.limit; await loadReturns(); };
    document.querySelector("#returns-body").onclick = async (event) => { const button = event.target.closest('button[data-action="settlements"]'); if (!button) return; try { await loadSettlementDetails(state.returns.find((item) => item.id === button.dataset.id)); } catch (error) { showError("#return-error", error); } };
    document.querySelectorAll("[data-return-close]").forEach((button) => { button.onclick = () => document.querySelector(`#${button.dataset.returnClose}`).close(); });
    document.querySelector("#add-return-settlement-button").onclick = openSettlement;
    document.querySelector("#return-settlement-form").onsubmit = async (event) => { event.preventDefault(); const form = event.currentTarget; const payload = {method: form.elements.method.value, amount: Number(form.elements.amount.value), externalReference: form.elements.externalReference.value.trim(), notes: form.elements.notes.value.trim()}; try { await api(`/api/manage/customer-returns/${state.current.id}/settlements`, {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify(payload)}); document.querySelector("#return-settlement-dialog").close(); await loadSettlementDetails(state.current); } catch (error) { showError("#return-settlement-error", error); } };
    document.querySelector("#return-settlements-body").onclick = async (event) => { const button = event.target.closest('button[data-action="void"]'); if (!button) return; const item = state.settlements.find((value) => value.id === button.dataset.id); const reason = window.prompt("Motif obligatoire de l’annulation :"); if (!reason) return; try { await api(`/api/manage/return-settlements/${item.id}/void`, {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({version: item.version, reason})}); await loadSettlementDetails(state.current); } catch (error) { showError("#return-error", error); } };
  }

  window.addEventListener("DOMContentLoaded", init);
})();
