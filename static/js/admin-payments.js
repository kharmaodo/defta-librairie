(() => {
  "use strict";

  const state = {
    isRoot: false, libraries: [], registers: [], activeRegisters: [], sales: [],
    registerOffset: 0, registerLimit: 10, registerTotal: 0, saleID: "", balance: null
  };
  const token = () => sessionStorage.getItem("defta.accessToken") || "";
  const money = (value) => new Intl.NumberFormat("fr-FR", {
    style: "currency", currency: "XOF", maximumFractionDigits: 2
  }).format(value || 0);

  async function api(path, options = {}) {
    const headers = new Headers(options.headers || {});
    headers.set("Authorization", `Bearer ${token()}`);
    const response = await fetch(path, {...options, headers});
    const type = response.headers.get("content-type") || "";
    const payload = type.includes("json") ? await response.json() : null;
    if (!response.ok) throw new Error(payload?.message || `Requête refusée (${response.status})`);
    return payload;
  }

  function showError(selector, error) {
    const box = document.querySelector(selector);
    box.textContent = error.message || "Une erreur est survenue.";
    box.hidden = false;
  }

  function cell(row, value, className = "") {
    const item = row.insertCell();
    item.textContent = value ?? "—";
    item.className = className;
    return item;
  }

  function button(label, action, id, danger = false) {
    const item = document.createElement("button");
    item.type = "button";
    item.textContent = label;
    item.dataset.action = action;
    item.dataset.id = id;
    item.className = `row-button${danger ? " danger" : ""}`;
    return item;
  }

  function fill(select, items, label) {
    select.replaceChildren(...items.map((item) => {
      const option = document.createElement("option");
      option.value = item.id;
      option.textContent = label(item);
      return option;
    }));
  }

  function libraryName(id) {
    return state.libraries.find((item) => item.id === id)?.name || id;
  }

  function selectedLibrary(selector) {
    return state.isRoot ? document.querySelector(selector)?.value || "" : "";
  }

  async function loadRegisters() {
    const form = document.querySelector("#cash-register-filters");
    const query = new URLSearchParams({
      offset: String(state.registerOffset), limit: String(state.registerLimit)
    });
    for (const name of ["q", "status", "libraryId"]) {
      const value = form.elements[name]?.value?.trim();
      if (value) query.set(name, value);
    }
    const payload = await api(`/api/manage/cash-registers?${query}`);
    state.registers = payload.results;
    state.registerTotal = payload.total;
    const body = document.querySelector("#cash-registers-body");
    body.replaceChildren();
    payload.results.forEach((register) => {
      const row = body.insertRow();
      cell(row, register.name);
      cell(row, libraryName(register.libraryId));
      cell(row, register.status === "ACTIVE" ? "Active" : "Désactivée", "pill");
      cell(row, register.version);
      const actions = cell(row, "");
      actions.className = "row-actions";
      const buttons = [button("Modifier", "edit", register.id)];
      if (register.status === "ACTIVE") buttons.push(button("Désactiver", "disable", register.id, true));
      else buttons.push(button("Réactiver", "reactivate", register.id));
      actions.replaceChildren(...buttons);
    });
    if (!payload.results.length) {
      const row = body.insertRow();
      const empty = cell(row, "Aucune caisse", "empty");
      empty.colSpan = 5;
    }
    const page = Math.floor(state.registerOffset / state.registerLimit) + 1;
    const pages = Math.max(1, Math.ceil(state.registerTotal / state.registerLimit));
    document.querySelector("#cash-registers-page-label").textContent = `Page ${page} sur ${pages}`;
    document.querySelector("#cash-registers-previous").disabled = state.registerOffset === 0;
    document.querySelector("#cash-registers-next").disabled = state.registerOffset + state.registerLimit >= state.registerTotal;
  }

  async function loadSales() {
    const libraryID = selectedLibrary("#payment-sale-filter [name=libraryId]");
    const query = new URLSearchParams({status: "CONFIRMED", offset: "0", limit: "100"});
    if (libraryID) query.set("libraryId", libraryID);
    const payload = await api(`/api/manage/sales?${query}`);
    state.sales = payload.results;
    const select = document.querySelector("#payment-sale-filter [name=saleId]");
    fill(select, [{id: "", reference: "Choisir une vente"}, ...state.sales], (sale) =>
      sale.id ? `${sale.reference} · ${sale.customerName || "Client comptoir"} · ${money(sale.totalAmount)}` : sale.reference
    );
    state.saleID = "";
    clearPayments();
  }

  async function loadActiveRegisters(libraryID) {
    const query = new URLSearchParams({status: "ACTIVE", offset: "0", limit: "100"});
    if (libraryID) query.set("libraryId", libraryID);
    const payload = await api(`/api/manage/cash-registers?${query}`);
    state.activeRegisters = payload.results;
  }

  function clearPayments() {
    state.balance = null;
    document.querySelector("#payment-balance").hidden = true;
    const body = document.querySelector("#payments-body");
    body.innerHTML = '<tr><td colspan="6" class="empty">Sélectionnez une vente confirmée.</td></tr>';
    document.querySelector("#payment-error").hidden = true;
  }

  async function loadPayments() {
    if (!state.saleID) return clearPayments();
    const [balance, payments] = await Promise.all([
      api(`/api/manage/sales/${state.saleID}/payment-balance`),
      api(`/api/manage/sales/${state.saleID}/payments?offset=0&limit=100`)
    ]);
    state.balance = balance;
    const summary = document.querySelector("#payment-balance");
    summary.querySelector("[data-payment-total]").textContent = money(balance.totalAmount);
    summary.querySelector("[data-payment-paid]").textContent = money(balance.paidAmount);
    summary.querySelector("[data-payment-remaining]").textContent = money(balance.remainingAmount);
    summary.querySelector("[data-payment-status]").textContent = {
      UNPAID: "Non payée", PARTIALLY_PAID: "Partiellement payée", PAID: "Payée"
    }[balance.paymentStatus] || balance.paymentStatus;
    document.querySelector("#add-payment-button").disabled = balance.remainingAmount <= 0;
    summary.hidden = false;
    const body = document.querySelector("#payments-body");
    body.replaceChildren();
    payments.results.forEach((payment) => {
      const row = body.insertRow();
      cell(row, new Date(payment.createdAt).toLocaleString("fr-FR"));
      cell(row, {CASH: "Espèces", MOBILE_MONEY: "Mobile money", CARD: "Carte"}[payment.method] || payment.method);
      cell(row, money(payment.amount));
      cell(row, payment.externalReference || "—");
      cell(row, payment.status === "RECORDED" ? "Enregistré" : "Annulé", `pill payment-${payment.status.toLowerCase()}`);
      const actions = cell(row, "");
      actions.className = "row-actions";
      if (payment.status === "RECORDED") actions.replaceChildren(button("Annuler", "void", payment.id, true));
    });
    if (!payments.results.length) {
      const row = body.insertRow();
      const empty = cell(row, "Aucun règlement", "empty");
      empty.colSpan = 6;
    }
    body.dataset.payments = JSON.stringify(payments.results);
  }

  function openRegister(register = null) {
    const form = document.querySelector("#cash-register-form");
    form.reset();
    form.elements.id.value = register?.id || "";
    form.elements.version.value = register?.version || "";
    form.elements.name.value = register?.name || "";
    if (state.isRoot) {
      fill(form.elements.libraryId, state.libraries, (library) => library.name);
      form.elements.libraryId.value = register?.libraryId || state.libraries[0]?.id || "";
      form.elements.libraryId.disabled = Boolean(register);
    }
    document.querySelector("#cash-register-form-title").textContent = register ? `Modifier · ${register.name}` : "Nouvelle caisse";
    document.querySelector("#cash-register-error").hidden = true;
    document.querySelector("#cash-register-dialog").showModal();
  }

  async function openPayment() {
    const sale = state.sales.find((item) => item.id === state.saleID);
    await loadActiveRegisters(sale?.libraryId || "");
    if (!state.activeRegisters.length) throw new Error("Aucune caisse active pour cette librairie.");
    const form = document.querySelector("#payment-form");
    form.reset();
    fill(form.elements.cashRegisterId, state.activeRegisters, (register) => register.name);
    form.elements.amount.value = state.balance?.remainingAmount || "";
    document.querySelector("#payment-form-error").hidden = true;
    document.querySelector("#payment-dialog").showModal();
  }

  async function init() {
    if (!document.querySelector("#cash-registers-body")) return;
    try {
      const me = await api("/api/auth/me");
      state.isRoot = me.role === "SUPER_ADMIN_ROOT";
      if (state.isRoot) {
        const owners = await api("/api/admin/owners?status=ACTIVE&libraryStatus=ACTIVE&limit=100");
        state.libraries = owners.results.map((owner) => owner.library);
        document.querySelectorAll(".payment-root").forEach((element) => { element.hidden = false; });
        const allLibraries = [{id: "", name: "Toutes"}, ...state.libraries];
        fill(document.querySelector("#cash-register-filters [name=libraryId]"), allLibraries, (library) => library.name);
        fill(document.querySelector("#payment-sale-filter [name=libraryId]"), state.libraries, (library) => library.name);
      }
      await Promise.all([loadRegisters(), loadSales()]);
    } catch (error) {
      showError("#payment-error", error);
      return;
    }

    document.querySelector("#add-cash-register-button").onclick = () => openRegister();
    document.querySelector("#add-payment-button").onclick = async () => {
      try { await openPayment(); } catch (error) { showError("#payment-error", error); }
    };
    document.querySelectorAll("[data-payment-close]").forEach((item) => {
      item.onclick = () => document.querySelector(`#${item.dataset.paymentClose}`).close();
    });
    document.querySelector("#cash-register-filters").onsubmit = async (event) => {
      event.preventDefault(); state.registerOffset = 0; await loadRegisters();
    };
    document.querySelector("#cash-registers-previous").onclick = async () => {
      state.registerOffset = Math.max(0, state.registerOffset - state.registerLimit); await loadRegisters();
    };
    document.querySelector("#cash-registers-next").onclick = async () => {
      state.registerOffset += state.registerLimit; await loadRegisters();
    };
    if (state.isRoot) document.querySelector("#payment-sale-filter [name=libraryId]").onchange = loadSales;
    document.querySelector("#payment-sale-filter").onsubmit = async (event) => {
      event.preventDefault();
      state.saleID = event.currentTarget.elements.saleId.value;
      try { await loadPayments(); } catch (error) { showError("#payment-error", error); }
    };
    document.querySelector("#cash-register-form").onsubmit = async (event) => {
      event.preventDefault();
      const form = event.currentTarget;
      const id = form.elements.id.value;
      const payload = {name: form.elements.name.value.trim()};
      if (state.isRoot) payload.libraryId = form.elements.libraryId.value;
      if (id) payload.version = Number(form.elements.version.value);
      try {
        await api(id ? `/api/manage/cash-registers/${id}` : "/api/manage/cash-registers", {
          method: id ? "PUT" : "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify(payload)
        });
        document.querySelector("#cash-register-dialog").close();
        await loadRegisters();
        if (state.saleID) await loadPayments();
      } catch (error) { showError("#cash-register-error", error); }
    };
    document.querySelector("#cash-registers-body").onclick = async (event) => {
      const action = event.target.closest("button[data-action]");
      if (!action) return;
      const register = state.registers.find((item) => item.id === action.dataset.id);
      if (action.dataset.action === "edit") return openRegister(register);
      if (!window.confirm(`${action.textContent} la caisse ${register.name} ?`)) return;
      try {
        await api(`/api/manage/cash-registers/${register.id}${action.dataset.action === "reactivate" ? "/reactivate" : ""}?version=${register.version}`, {
          method: action.dataset.action === "reactivate" ? "POST" : "DELETE"
        });
        await loadRegisters();
      } catch (error) { showError("#payment-error", error); }
    };
    document.querySelector("#payment-form").onsubmit = async (event) => {
      event.preventDefault();
      const form = event.currentTarget;
      const payload = {
        cashRegisterId: form.elements.cashRegisterId.value, method: form.elements.method.value,
        amount: Number(form.elements.amount.value), externalReference: form.elements.externalReference.value.trim(),
        notes: form.elements.notes.value.trim()
      };
      try {
        await api(`/api/manage/sales/${state.saleID}/payments`, {
          method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify(payload)
        });
        document.querySelector("#payment-dialog").close();
        await loadPayments();
      } catch (error) { showError("#payment-form-error", error); }
    };
    document.querySelector("#payments-body").onclick = async (event) => {
      const action = event.target.closest('button[data-action="void"]');
      if (!action) return;
      const payments = JSON.parse(event.currentTarget.dataset.payments || "[]");
      const payment = payments.find((item) => item.id === action.dataset.id);
      const reason = window.prompt("Motif obligatoire de l’annulation :");
      if (!reason) return;
      try {
        await api(`/api/manage/payments/${payment.id}/void`, {
          method: "POST", headers: {"Content-Type": "application/json"},
          body: JSON.stringify({version: payment.version, reason})
        });
        await loadPayments();
      } catch (error) { showError("#payment-error", error); }
    };
  }

  window.addEventListener("DOMContentLoaded", init);
})();
