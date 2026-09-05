(() => {
  "use strict";

  const token = () => sessionStorage.getItem("defta.accessToken") || "";
  const state = {customers: [], libraries: [], isRoot: false, offset: 0, limit: 10, total: 0};

  async function api(path, options = {}) {
    const headers = new Headers(options.headers || {});
    headers.set("Authorization", `Bearer ${token()}`);
    const response = await fetch(path, {...options, headers});
    const type = response.headers.get("content-type") || "";
    const payload = type.includes("json") ? await response.json() : null;
    if (!response.ok) throw new Error(payload?.message || `Requête refusée (${response.status})`);
    return payload;
  }

  function fillLibraries(select, includeAll) {
    const items = includeAll ? [{id: "", name: "Toutes"}, ...state.libraries] : state.libraries;
    select.replaceChildren(...items.map((library) => {
      const option = document.createElement("option");
      option.value = library.id;
      option.textContent = library.name;
      return option;
    }));
  }

  function addCell(row, value, className = "") {
    const cell = row.insertCell();
    cell.textContent = value || "—";
    cell.className = className;
    return cell;
  }

  function actionButton(label, action, id, danger = false) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = `row-button${danger ? " danger" : ""}`;
    button.dataset.action = action;
    button.dataset.id = id;
    button.textContent = label;
    return button;
  }

  async function loadCustomers() {
    const form = document.querySelector("#customer-filters");
    const params = new URLSearchParams({offset: String(state.offset), limit: String(state.limit)});
    for (const name of ["q", "status", "libraryId"]) {
      const value = form.elements[name]?.value?.trim();
      if (value) params.set(name, value);
    }
    const payload = await api(`/api/manage/customers?${params}`);
    state.customers = payload.results;
    state.total = payload.total;
    const body = document.querySelector("#customers-body");
    body.replaceChildren();
    payload.results.forEach((customer) => {
      const row = body.insertRow();
      addCell(row, customer.reference);
      addCell(row, customer.name);
      addCell(row, customer.phone);
      addCell(row, customer.email);
      addCell(row, customer.status === "ACTIVE" ? "Actif" : "Désactivé", "pill");
      const actions = addCell(row, "");
      actions.className = "row-actions";
      const buttons = [actionButton("Modifier", "edit", customer.id)];
      if (customer.status === "ACTIVE") buttons.push(actionButton("Désactiver", "disable", customer.id, true));
      else buttons.push(actionButton("Réactiver", "reactivate", customer.id));
      actions.replaceChildren(...buttons);
    });
    if (!payload.results.length) {
      const row = body.insertRow();
      const cell = addCell(row, "Aucun client", "empty");
      cell.colSpan = 6;
    }
    const page = Math.floor(state.offset / state.limit) + 1;
    const pages = Math.max(1, Math.ceil(state.total / state.limit));
    document.querySelector("#customers-page-label").textContent = `Page ${page} sur ${pages} · ${state.total} résultat${state.total > 1 ? "s" : ""}`;
    document.querySelector("#customers-previous").disabled = state.offset === 0;
    document.querySelector("#customers-next").disabled = state.offset + state.limit >= state.total;
  }

  function openCustomer(customer = null) {
    const form = document.querySelector("#customer-form");
    form.reset();
    for (const name of ["id", "version", "name", "phone", "email", "address", "notes"]) {
      form.elements[name].value = customer?.[name] || "";
    }
    if (state.isRoot) {
      fillLibraries(form.elements.libraryId, false);
      form.elements.libraryId.value = customer?.libraryId || state.libraries[0]?.id || "";
      form.elements.libraryId.disabled = Boolean(customer);
    }
    document.querySelector("#customer-form-title").textContent = customer ? customer.reference : "Nouveau client";
    document.querySelector("#customer-error").hidden = true;
    document.querySelector("#customer-dialog").showModal();
  }

  async function init() {
    if (!document.querySelector("#customers-body")) return;
    try {
      const me = await api("/api/auth/me");
      state.isRoot = me.role === "SUPER_ADMIN_ROOT";
      if (state.isRoot) {
        const owners = await api("/api/admin/owners?status=ACTIVE&libraryStatus=ACTIVE&limit=100");
        state.libraries = owners.results.map((owner) => owner.library);
        document.querySelectorAll(".customer-root").forEach((element) => { element.hidden = false; });
        fillLibraries(document.querySelector("#customer-filters [name=libraryId]"), true);
      }
      await loadCustomers();
    } catch (_) {
      return;
    }

    document.querySelector("#add-customer-button").onclick = () => openCustomer();
    document.querySelectorAll("[data-customer-close]").forEach((button) => {
      button.onclick = () => document.querySelector("#customer-dialog").close();
    });
    document.querySelector("#customer-filters").onsubmit = async (event) => {
      event.preventDefault(); state.offset = 0; await loadCustomers();
    };
    document.querySelector("#reset-customer-filters").onclick = async () => {
      document.querySelector("#customer-filters").reset(); state.offset = 0; await loadCustomers();
    };
    document.querySelector("#customers-previous").onclick = async () => {
      state.offset = Math.max(0, state.offset - state.limit); await loadCustomers();
    };
    document.querySelector("#customers-next").onclick = async () => {
      if (state.offset + state.limit < state.total) { state.offset += state.limit; await loadCustomers(); }
    };
    document.querySelector("#customer-form").onsubmit = async (event) => {
      event.preventDefault();
      const form = event.currentTarget;
      const id = form.elements.id.value;
      const payload = {name: form.elements.name.value, phone: form.elements.phone.value,
        email: form.elements.email.value, address: form.elements.address.value, notes: form.elements.notes.value};
      if (id) payload.version = Number(form.elements.version.value);
      if (state.isRoot && !id) payload.libraryId = form.elements.libraryId.value;
      try {
        await api(id ? `/api/manage/customers/${id}` : "/api/manage/customers", {
          method: id ? "PUT" : "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify(payload),
        });
        document.querySelector("#customer-dialog").close();
        await loadCustomers();
      } catch (reason) {
        const box = document.querySelector("#customer-error"); box.textContent = reason.message; box.hidden = false;
      }
    };
    document.querySelector("#customers-body").onclick = async (event) => {
      const button = event.target.closest("button");
      if (!button) return;
      const customer = state.customers.find((item) => item.id === button.dataset.id);
      if (button.dataset.action === "edit") return openCustomer(customer);
      if (!confirm(`${button.textContent} ${customer.name} ?`)) return;
      const reactivate = button.dataset.action === "reactivate";
      await api(`/api/manage/customers/${customer.id}${reactivate ? "/reactivate" : ""}?version=${customer.version}`,
        {method: reactivate ? "POST" : "DELETE"});
      await loadCustomers();
    };
  }

  window.addEventListener("DOMContentLoaded", init);
})();
