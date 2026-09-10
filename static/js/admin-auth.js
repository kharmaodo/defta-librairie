(() => {
  "use strict";

  const ACCESS_KEY = "defta.accessToken";
  const USERNAME_KEY = "defta.username";
  const page = document.body.dataset.page;
  const state = {isRoot: false, passwordChangeRequired: false, sales: [], saleBooks: [], saleCustomers: [], inventory: [], tags: [], saleOffset: 0, saleLimit: 10, inventoryOffset: 0, inventoryLimit: 10};

  const tokens = {
    access: () => sessionStorage.getItem(ACCESS_KEY),
    save: (payload) => {
      sessionStorage.setItem(ACCESS_KEY, payload.accessToken);
    },
    clear: () => {
      sessionStorage.removeItem(ACCESS_KEY);
      sessionStorage.removeItem(USERNAME_KEY);
    }
  };

  async function json(response) {
    const contentType = response.headers.get("content-type") || "";
    const payload = contentType.includes("application/json") ? await response.json() : {};
    if (!response.ok) {
      const error = new Error(payload.message || `Requête refusée (${response.status})`);
      error.status = response.status;
      error.retryAfter = response.headers.get("retry-after");
      throw error;
    }
    return payload;
  }

  async function refreshSession() {
    const response = await fetch("/api/auth/refresh", {
      method: "POST", headers: {"X-Defta-Session": "cookie"}
    });
    const payload = await json(response);
    tokens.save(payload);
  }

  async function apiFetch(path, options = {}, retry = true) {
    const headers = new Headers(options.headers || {});
    headers.set("Authorization", `Bearer ${tokens.access() || ""}`);
    const response = await fetch(path, {...options, headers});
    if (response.status === 401 && retry) {
      await refreshSession();
      return apiFetch(path, options, false);
    }
    return json(response);
  }

  let audit, sessions, owners, books;
  const reloadBooks = () => books.reload();
  const reloadOwners = () => owners.reload();
  const reloadOwnerOptions = () => owners.reloadOptions();
  const reloadSessions = () => sessions.reload();
  const reloadAudit = () => audit.reload();

  function textCell(row, value, className) {
    const cell = row.insertCell();
    cell.textContent = value === null || value === undefined || value === "" ? "—" : String(value);
    if (className) cell.className = className;
    return cell;
  }

  function actionButton(label, action, id, danger = false) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = `row-button${danger ? " danger" : ""}`;
    button.dataset.action = action;
    button.dataset.id = String(id);
    button.textContent = label;
    return button;
  }

  function showError(element, error) {
    let message = error.message || "Une erreur est survenue.";
    if (error.status === 429 && error.retryAfter) message += ` Réessayez dans ${error.retryAfter} secondes.`;
    element.textContent = message;
    element.hidden = false;
  }

  async function initLogin() {
    const form = document.querySelector("#login-form");
    const errorBox = document.querySelector("#login-error");
    if (new URLSearchParams(window.location.search).get("passwordChanged") === "1") {
      const notice = document.querySelector("#login-notice");
      notice.textContent = "Mot de passe modifié. Reconnectez-vous avec votre nouveau mot de passe.";
      notice.hidden = false;
      window.history.replaceState({}, "", "/login");
    }
    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      errorBox.hidden = true;
      const button = form.querySelector("button[type=submit]");
      button.disabled = true;
      try {
        const data = new FormData(form);
        const response = await fetch("/api/auth/login", {
          method: "POST", headers: {"Content-Type": "application/json", "X-Defta-Session": "cookie"},
          body: JSON.stringify({username: data.get("username"), password: data.get("password")})
        });
        const payload = await json(response);
        tokens.save(payload);
        sessionStorage.setItem(USERNAME_KEY, payload.user.username);
        window.location.replace("/admin");
      } catch (error) {
        showError(errorBox, error);
      } finally {
        button.disabled = false;
      }
    });
  }

  function renderInventory(payload) {
    state.inventory = payload.results;
    const body = document.querySelector("#inventory-body");
    body.replaceChildren();
    if (!payload.results.length) {
      const row = body.insertRow(); textCell(row, "Aucun stock correspondant", "empty").colSpan = 6;
    } else payload.results.forEach((item) => {
      const row = body.insertRow();
      textCell(row, item.title);
      textCell(row, item.quantity);
      textCell(row, item.lowStockThreshold);
      const status = textCell(row, item.stockStatus === "OUT_OF_STOCK" ? "Rupture" : item.stockStatus === "LOW_STOCK" ? "Stock faible" : "En stock", "pill");
      status.classList.add(item.stockStatus === "OUT_OF_STOCK" ? "stock-out" : item.stockStatus === "LOW_STOCK" ? "stock-low" : "stock-ok");
      textCell(row, formatDate(item.updatedAt));
      const actions = textCell(row, "");
      actions.className = "row-actions";
      actions.replaceChildren(actionButton("Mouvement", "edit-inventory", item.bookId), actionButton("Historique", "history-inventory", item.bookId));
    });
    const page = Math.floor(payload.offset / payload.limit) + 1;
    const pages = Math.max(1, Math.ceil(payload.total / payload.limit));
    document.querySelector("#inventory-page-label").textContent = `Page ${page} sur ${pages}`;
    document.querySelector("#inventory-previous").disabled = payload.offset === 0;
    document.querySelector("#inventory-next").disabled = payload.offset + payload.results.length >= payload.total;
  }

  function renderSales(payload) {
    state.sales = payload.results;
    document.querySelector("#sale-total").textContent = payload.total;
    const body = document.querySelector("#sales-body");
    body.replaceChildren();
    if (!payload.results.length) {
      const row = body.insertRow(); textCell(row, "Aucune vente correspondante", "empty").colSpan = 7;
    } else payload.results.forEach((sale) => {
      const row = body.insertRow();
      textCell(row, sale.reference);
      textCell(row, sale.customerName || "Client comptoir");
      textCell(row, sale.lines.reduce((total, line) => total + line.quantity, 0));
      textCell(row, new Intl.NumberFormat("fr-FR", {style: "currency", currency: "XOF", maximumFractionDigits: 0}).format(sale.totalAmount || 0));
      const statusLabel = sale.status === "DRAFT" ? "Brouillon" : sale.status === "CONFIRMED" ? "Confirmée" : "Annulée";
      const status = textCell(row, statusLabel, "pill");
      status.classList.add(`sale-${sale.status.toLowerCase()}`);
      textCell(row, formatDate(sale.createdAt));
      const actions = textCell(row, "");
      actions.className = "row-actions";
      const buttons = [actionButton("Détails", "detail-sale", sale.id)];
      if (sale.status === "DRAFT") buttons.push(actionButton("Modifier", "edit-sale", sale.id), actionButton("Confirmer", "confirm-sale", sale.id), actionButton("Supprimer", "delete-sale", sale.id, true));
      if (sale.status === "CONFIRMED") buttons.push(actionButton("Annuler", "cancel-sale", sale.id, true));
      actions.replaceChildren(...buttons);
    });
    const page = Math.floor(payload.offset / payload.limit) + 1;
    const pages = Math.max(1, Math.ceil(payload.total / payload.limit));
    document.querySelector("#sales-page-label").textContent = `Page ${page} sur ${pages}`;
    document.querySelector("#sales-previous").disabled = payload.offset === 0;
    document.querySelector("#sales-next").disabled = payload.offset + payload.results.length >= payload.total;
  }

  function formatDate(value) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value || "—" : date.toLocaleString("fr-FR");
  }

  function updateLibraryOptions(ownerOptions) {
    document.querySelectorAll("#book-form [name=libraryId], #tag-form [name=libraryId], #inventory-filters [name=libraryId], #sale-filters [name=libraryId], #sale-form [name=libraryId]").forEach((select) => {
      const selected = select.value;
      select.replaceChildren();
      const placeholder = document.createElement("option");
      placeholder.value = ""; placeholder.textContent = "Choisir une librairie";
      select.append(placeholder);
      ownerOptions.forEach((owner) => {
        const option = document.createElement("option");
        option.value = owner.library.id;
        option.textContent = `${owner.library.name} · ${owner.username}`;
        select.append(option);
      });
      select.value = selected;
    });
  }

  function renderTags(payload) {
    state.tags = payload.results;
    const list = document.querySelector("#tags-list");
    list.replaceChildren();
    if (!payload.results.length) {
      const empty = document.createElement("span"); empty.className = "hint"; empty.textContent = "Aucun tag défini"; list.append(empty);
    } else payload.results.forEach((tag) => {
      const chip = document.createElement("span"); chip.className = "tag-chip"; chip.append(document.createTextNode(tag.name));
      const remove = document.createElement("button"); remove.type = "button"; remove.dataset.id = tag.id; remove.setAttribute("aria-label", `Supprimer ${tag.name}`); remove.textContent = "×"; chip.append(remove); list.append(chip);
    });
    const suggestions = document.querySelector("#tag-suggestions");
    suggestions.replaceChildren(...payload.results.map((tag) => {
      const option = document.createElement("option"); option.value = tag.name; return option;
    }));
  }

  function openInventoryForm(item) {
    const form = document.querySelector("#inventory-form");
    form.reset();
    form.elements.bookId.value = item.bookId;
    form.elements.version.value = item.version;
    form.elements.quantity.value = 1;
    document.querySelector("#inventory-form-title").textContent = `Stock · ${item.title}`;
    document.querySelector("#inventory-version-label").textContent = item.version;
    document.querySelector("#inventory-form-error").hidden = true;
    document.querySelector("#inventory-dialog").showModal();
  }

  function formatMoney(value) {
    return new Intl.NumberFormat("fr-FR", {style: "currency", currency: "XOF", maximumFractionDigits: 0}).format(value || 0);
  }

  async function loadSaleBooks(libraryID) {
    if (state.isRoot && !libraryID) {
      state.saleBooks = [];
      return;
    }
    const books = [];
    let offset = 0;
    let total = 0;
    do {
      const query = new URLSearchParams({offset: String(offset), limit: "100"});
      if (libraryID) query.set("libraryId", libraryID);
      const payload = await apiFetch(`/api/manage/books?${query}`);
      books.push(...payload.results);
      offset += payload.results.length;
      total = payload.total;
    } while (offset < total);
    state.saleBooks = books;
  }

  async function loadSaleCustomers(libraryID) {
    const select = document.querySelector("#sale-form [name=customerId]");
    select.replaceChildren();
    const placeholder = document.createElement("option");
    placeholder.value = "";
    placeholder.textContent = "Aucun · vente comptoir";
    select.append(placeholder);
    if (state.isRoot && !libraryID) {
      state.saleCustomers = [];
      return;
    }
    const customers = [];
    let offset = 0;
    let total = 0;
    do {
      const query = new URLSearchParams({status: "ACTIVE", offset: String(offset), limit: "100"});
      if (libraryID) query.set("libraryId", libraryID);
      const payload = await apiFetch(`/api/manage/customers?${query}`);
      customers.push(...payload.results);
      offset += payload.results.length;
      total = payload.total;
    } while (offset < total);
    state.saleCustomers = customers;
    customers.forEach((customer) => {
      const option = document.createElement("option");
      option.value = customer.id;
      option.textContent = `${customer.reference} · ${customer.name}`;
      select.append(option);
    });
  }

  function syncSaleCustomerName() {
    const form = document.querySelector("#sale-form");
    const customer = state.saleCustomers.find((item) => item.id === form.elements.customerId.value);
    const input = form.elements.customerName;
    if (customer) {
      input.value = customer.name;
      input.readOnly = true;
      input.dataset.linkedName = customer.name;
      return;
    }
    if (input.value === input.dataset.linkedName) input.value = "";
    input.readOnly = false;
    delete input.dataset.linkedName;
  }

  function updateSaleEstimate() {
    let total = 0;
    document.querySelectorAll("#sale-lines .sale-line").forEach((row) => {
      const book = state.saleBooks.find((item) => item.id === Number(row.querySelector("[name=bookId]").value));
      const quantity = Number(row.querySelector("[name=quantity]").value) || 0;
      const lineTotal = book ? book.price * quantity : 0;
      row.querySelector(".sale-line-price").textContent = book ? formatMoney(lineTotal) : "—";
      total += lineTotal;
    });
    document.querySelector("#sale-estimated-total").textContent = formatMoney(total);
  }

  function addSaleLine(line = null) {
    const fragment = document.querySelector("#sale-line-template").content.cloneNode(true);
    const row = fragment.querySelector(".sale-line");
    const select = row.querySelector("[name=bookId]");
    const placeholder = document.createElement("option");
    placeholder.value = ""; placeholder.textContent = "Choisir un livre";
    select.append(placeholder);
    state.saleBooks.forEach((book) => {
      const option = document.createElement("option");
      option.value = String(book.id);
      option.textContent = `${book.title} · ${formatMoney(book.price)}`;
      select.append(option);
    });
    if (line) {
      select.value = String(line.bookId);
      row.querySelector("[name=quantity]").value = line.quantity;
    }
    row.addEventListener("input", updateSaleEstimate);
    row.querySelector(".remove-sale-line").addEventListener("click", () => {
      row.remove(); updateSaleEstimate();
    });
    document.querySelector("#sale-lines").append(row);
    updateSaleEstimate();
  }

  async function openSaleForm(sale = null) {
    const dialog = document.querySelector("#sale-dialog");
    const form = document.querySelector("#sale-form");
    form.reset();
    form.elements.id.value = sale ? sale.id : "";
    form.elements.version.value = sale ? sale.version : "";
    form.elements.customerName.value = sale ? sale.customerName || "" : "";
    form.elements.libraryId.value = sale ? sale.libraryId : "";
    form.elements.libraryId.disabled = Boolean(sale);
    document.querySelector("#sale-lines").replaceChildren();
    document.querySelector("#sale-form-error").hidden = true;
    document.querySelector("#sale-form-title").textContent = sale ? `Modifier · ${sale.reference}` : "Nouvelle vente";
    const libraryID = sale ? sale.libraryId : form.elements.libraryId.value;
    await Promise.all([loadSaleBooks(libraryID), loadSaleCustomers(libraryID)]);
    form.elements.customerId.value = sale ? sale.customerId || "" : "";
    if (form.elements.customerId.value) syncSaleCustomerName();
    else {
      form.elements.customerName.readOnly = false;
      delete form.elements.customerName.dataset.linkedName;
    }
    (sale && sale.lines.length ? sale.lines : [null]).forEach(addSaleLine);
    dialog.showModal();
  }

  function salePayload(form) {
    const lines = Array.from(document.querySelectorAll("#sale-lines .sale-line")).map((row) => ({
      bookId: Number(row.querySelector("[name=bookId]").value),
      quantity: Number(row.querySelector("[name=quantity]").value)
    }));
    const payload = {customerId: form.elements.customerId.value, customerName: form.elements.customerName.value.trim(), lines};
    if (state.isRoot && form.elements.libraryId.value) payload.libraryId = form.elements.libraryId.value;
    if (form.elements.id.value) payload.version = Number(form.elements.version.value);
    return payload;
  }

  async function openSaleDetails(saleID) {
    const sale = await apiFetch(`/api/manage/sales/${saleID}`);
    const statusLabel = sale.status === "DRAFT" ? "Brouillon" : sale.status === "CONFIRMED" ? "Confirmée" : "Annulée";
    document.querySelector("#sale-detail-reference").textContent = sale.reference;
    document.querySelector("#sale-detail-customer").textContent = sale.customerName || "Client comptoir";
    document.querySelector("#sale-detail-created").textContent = formatDate(sale.createdAt);
    document.querySelector("#sale-detail-total").textContent = formatMoney(sale.totalAmount);
    const status = document.querySelector("#sale-detail-status");
    status.textContent = statusLabel;
    status.className = `pill sale-${sale.status.toLowerCase()}`;
    const transitionRow = document.querySelector("#sale-detail-transition-row");
    transitionRow.hidden = sale.status === "DRAFT";
    if (!transitionRow.hidden) {
      document.querySelector("#sale-detail-transition-label").textContent = sale.status === "CONFIRMED" ? "Confirmée le" : "Annulée le";
      document.querySelector("#sale-detail-transition-date").textContent = formatDate(sale.status === "CONFIRMED" ? sale.confirmedAt : sale.cancelledAt);
    }
    const body = document.querySelector("#sale-detail-lines");
    body.replaceChildren();
    sale.lines.forEach((line) => {
      const row = body.insertRow();
      textCell(row, line.title);
      textCell(row, line.quantity);
      textCell(row, formatMoney(line.unitPrice));
      textCell(row, formatMoney(line.lineTotal));
    });
    document.querySelector("#sale-detail-error").hidden = true;
    await window.deftaPrintSettings("#sale-receipt", sale.libraryId);
    document.querySelector("#sale-detail-dialog").showModal();
  }

  async function openInventoryHistory(item) {
    const payload = await apiFetch(`/api/manage/books/${item.bookId}/inventory/movements?offset=0&limit=100`);
    document.querySelector("#inventory-history-title").textContent = `Mouvements · ${item.title}`;
    const body = document.querySelector("#inventory-history-body");
    body.replaceChildren();
    if (!payload.results.length) {
      const row = body.insertRow(); textCell(row, "Aucun mouvement enregistré", "empty").colSpan = 6;
    } else payload.results.forEach((movement) => {
      const row = body.insertRow();
      textCell(row, formatDate(movement.createdAt));
      textCell(row, movement.movementType, "pill");
      textCell(row, movement.quantityDelta > 0 ? `+${movement.quantityDelta}` : movement.quantityDelta);
      textCell(row, movement.quantityBefore);
      textCell(row, movement.quantityAfter);
      textCell(row, movement.reason);
    });
    document.querySelector("#inventory-history-dialog").showModal();
  }

  async function reloadInventory() {
    const form = document.querySelector("#inventory-filters");
    const query = new URLSearchParams({offset: String(state.inventoryOffset), limit: String(state.inventoryLimit)});
    const status = form.elements.status.value;
    const libraryID = state.isRoot ? form.elements.libraryId.value : "";
    if (status) query.set("status", status);
    if (libraryID) query.set("libraryId", libraryID);
    const payload = await apiFetch(`/api/manage/inventory?${query}`);
    if (!payload.results.length && state.inventoryOffset > 0) {
      state.inventoryOffset = Math.max(0, state.inventoryOffset - state.inventoryLimit);
      return reloadInventory();
    }
    renderInventory(payload);
  }

  async function reloadSales() {
    const form = document.querySelector("#sale-filters");
    const query = new URLSearchParams({offset: String(state.saleOffset), limit: String(state.saleLimit)});
    const status = form.elements.status.value;
    const libraryID = state.isRoot ? form.elements.libraryId.value : "";
    if (status) query.set("status", status);
    if (libraryID) query.set("libraryId", libraryID);
    ["from", "to"].forEach((name) => {
      const value = form.elements[name].value;
      if (value) query.set(name, new Date(value).toISOString());
    });
    const payload = await apiFetch(`/api/manage/sales?${query}`);
    if (!payload.results.length && state.saleOffset > 0) {
      state.saleOffset = Math.max(0, state.saleOffset - state.saleLimit);
      return reloadSales();
    }
    renderSales(payload);
  }

  async function reloadTags() {
    const libraryID = state.isRoot ? document.querySelector("#tag-library").value : "";
    if (state.isRoot && !libraryID) {
      renderTags({results: [], total: 0});
      return;
    }
    const query = new URLSearchParams();
    if (libraryID) query.set("libraryId", libraryID);
    renderTags(await apiFetch(`/api/manage/tags?${query}`));
  }

  function initEntityForms(errorBox) {
    const passwordDialog = document.querySelector("#password-dialog");
    passwordDialog.addEventListener("cancel", (event) => {
      if (state.passwordChangeRequired) event.preventDefault();
    });
    document.querySelectorAll("[data-close]").forEach((button) => button.addEventListener("click", () => document.querySelector(`#${button.dataset.close}`).close()));
    document.querySelector("#add-sale-button").addEventListener("click", async () => {
      try { await openSaleForm(); } catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#change-password-button").addEventListener("click", () => {
      const form = document.querySelector("#password-form");
      form.reset();
      document.querySelector("#password-form-error").hidden = true;
      document.querySelector("#password-dialog").showModal();
    });
    document.querySelector("#inventory-filters").addEventListener("submit", async (event) => {
      event.preventDefault();
      state.inventoryOffset = 0;
      try { await reloadInventory(); } catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#inventory-previous").addEventListener("click", async () => {
      state.inventoryOffset = Math.max(0, state.inventoryOffset - state.inventoryLimit);
      try { await reloadInventory(); } catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#inventory-next").addEventListener("click", async () => {
      state.inventoryOffset += state.inventoryLimit;
      try { await reloadInventory(); } catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#sale-filters").addEventListener("submit", async (event) => {
      event.preventDefault();
      state.saleOffset = 0;
      errorBox.hidden = true;
      try { await reloadSales(); } catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#sales-previous").addEventListener("click", async () => {
      state.saleOffset = Math.max(0, state.saleOffset - state.saleLimit);
      try { await reloadSales(); } catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#sales-next").addEventListener("click", async () => {
      state.saleOffset += state.saleLimit;
      try { await reloadSales(); } catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#sale-form [name=libraryId]").addEventListener("change", async (event) => {
      try {
        await Promise.all([loadSaleBooks(event.currentTarget.value), loadSaleCustomers(event.currentTarget.value)]);
        const form = document.querySelector("#sale-form");
        form.elements.customerId.value = "";
        form.elements.customerName.value = "";
        form.elements.customerName.readOnly = false;
        document.querySelector("#sale-lines").replaceChildren();
        addSaleLine();
      } catch (error) { showError(document.querySelector("#sale-form-error"), error); }
    });
    document.querySelector("#sale-form [name=customerId]").addEventListener("change", syncSaleCustomerName);
    document.querySelector("#add-sale-line-button").addEventListener("click", () => addSaleLine());
    document.querySelector("#print-sale-button").addEventListener("click", () => {
      document.body.classList.add("printing-sale");
      const cleanup = () => document.body.classList.remove("printing-sale");
      window.addEventListener("afterprint", cleanup, {once: true});
      window.print();
      window.setTimeout(cleanup, 1000);
    });
    document.querySelector("#sale-form").addEventListener("submit", async (event) => {
      event.preventDefault();
      const form = event.currentTarget;
      const formError = document.querySelector("#sale-form-error");
      formError.hidden = true;
      const payload = salePayload(form);
      if (state.isRoot && !form.elements.id.value && !payload.libraryId) {
        showError(formError, new Error("Choisissez la librairie de la vente.")); return;
      }
      if (!payload.lines.length || payload.lines.some((line) => !line.bookId || line.quantity < 1)) {
        showError(formError, new Error("Ajoutez au moins un livre avec une quantité valide.")); return;
      }
      if (new Set(payload.lines.map((line) => line.bookId)).size !== payload.lines.length) {
        showError(formError, new Error("Un même livre ne peut apparaître qu’une seule fois.")); return;
      }
      const id = form.elements.id.value;
      try {
        await apiFetch(id ? `/api/manage/sales/${id}` : "/api/manage/sales", {
          method: id ? "PUT" : "POST", headers: {"Content-Type": "application/json"},
          body: JSON.stringify(payload)
        });
        document.querySelector("#sale-dialog").close();
        if (!id) state.saleOffset = 0;
        await Promise.all([reloadSales(), reloadAudit()]);
      } catch (error) { showError(formError, error); }
    });

    document.querySelector("#tag-library").addEventListener("change", async () => {
      try { await reloadTags(); } catch (error) { showError(errorBox, error); }
    });

    document.querySelector("#tag-form").addEventListener("submit", async (event) => {
      event.preventDefault();
      const form = event.currentTarget;
      const payload = {name: form.elements.name.value.trim()};
      if (state.isRoot) payload.libraryId = form.elements.libraryId.value;
      if (state.isRoot && !payload.libraryId) { showError(errorBox, new Error("Choisissez une librairie.")); return; }
      try {
        await apiFetch("/api/manage/tags", {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify(payload)});
        form.elements.name.value = "";
        await Promise.all([reloadTags(), reloadAudit()]);
      } catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#tags-list").addEventListener("click", async (event) => {
      const button = event.target.closest("button[data-id]");
      if (!button) return;
      const tag = state.tags.find((item) => item.id === button.dataset.id);
      if (!tag || !window.confirm(`Supprimer le tag « ${tag.name} » ?`)) return;
      try { await apiFetch(`/api/manage/tags/${tag.id}`, {method: "DELETE"}); await Promise.all([reloadTags(), reloadAudit()]); }
      catch (error) { showError(errorBox, error); }
    });

    document.querySelector("#password-form").addEventListener("submit", async (event) => {
      event.preventDefault();
      const form = event.currentTarget;
      const formError = document.querySelector("#password-form-error");
      formError.hidden = true;
      if (form.elements.newPassword.value !== form.elements.confirmation.value) {
        showError(formError, new Error("La confirmation ne correspond pas au nouveau mot de passe."));
        return;
      }
      try {
        await apiFetch("/api/auth/change-password", {
          method: "POST", headers: {"Content-Type": "application/json"},
          body: JSON.stringify({currentPassword: form.elements.currentPassword.value, newPassword: form.elements.newPassword.value})
        });
        tokens.clear();
        window.location.replace("/login?passwordChanged=1");
      } catch (error) { showError(formError, error); }
    });

    document.querySelector("#inventory-form").addEventListener("submit", async (event) => {
      event.preventDefault();
      const form = event.currentTarget;
      const formError = document.querySelector("#inventory-form-error");
      formError.hidden = true;
      const operation = form.elements.operation.value;
      const bookID = form.elements.bookId.value;
      const quantity = Number(form.elements.quantity.value);
      const version = Number(form.elements.version.value);
      const reason = form.elements.reason.value.trim();
      let path = `/api/manage/books/${bookID}/inventory`;
      let method = "PUT";
      let payload = {quantity, reason, version};
      if (operation === "ENTRY") { path += "/entries"; method = "POST"; }
      if (operation === "EXIT") { path += "/exits"; method = "POST"; }
      if (operation === "THRESHOLD") {
        path += "/threshold"; method = "PATCH"; payload = {lowStockThreshold: quantity, version};
      }
      if ((operation === "ENTRY" || operation === "EXIT") && quantity < 1) {
        showError(formError, new Error("La quantité doit être supérieure à zéro.")); return;
      }
      if (operation === "ADJUSTMENT" && !reason) {
        showError(formError, new Error("Le motif est obligatoire pour un ajustement.")); return;
      }
      try {
        await apiFetch(path, {method, headers: {"Content-Type": "application/json"}, body: JSON.stringify(payload)});
        document.querySelector("#inventory-dialog").close();
        await Promise.all([reloadInventory(), reloadAudit()]);
      } catch (error) { showError(formError, error); }
    });

    document.querySelector("#inventory-body").addEventListener("click", async (event) => {
      const button = event.target.closest("button[data-action]");
      if (!button) return;
      const item = state.inventory.find((entry) => entry.bookId === Number(button.dataset.id));
      if (!item) return;
      if (button.dataset.action === "edit-inventory") openInventoryForm(item);
      if (button.dataset.action === "history-inventory") {
        try { await openInventoryHistory(item); } catch (error) { showError(errorBox, error); }
      }
    });

    document.querySelector("#sales-body").addEventListener("click", async (event) => {
      const button = event.target.closest("button[data-action]");
      if (!button) return;
      const sale = state.sales.find((item) => item.id === button.dataset.id);
      if (!sale) return;
      if (button.dataset.action === "detail-sale") {
        try { await openSaleDetails(sale.id); } catch (error) { showError(errorBox, error); }
        return;
      }
      if (button.dataset.action === "edit-sale") {
        try { await openSaleForm(sale); } catch (error) { showError(errorBox, error); }
        return;
      }
      if (button.dataset.action === "delete-sale") {
        if (!window.confirm(`Supprimer définitivement le brouillon ${sale.reference} ?`)) return;
        button.disabled = true;
        try {
          await apiFetch(`/api/manage/sales/${sale.id}`, {method: "DELETE"});
          await Promise.all([reloadSales(), reloadAudit()]);
        } catch (error) { showError(errorBox, error); }
        finally { button.disabled = false; }
        return;
      }
      const confirm = button.dataset.action === "confirm-sale";
      const cancel = button.dataset.action === "cancel-sale";
      if (!confirm && !cancel) return;
      const question = confirm
        ? `Confirmer la vente ${sale.reference} et retirer les articles du stock ?`
        : `Annuler la vente ${sale.reference} et restituer les articles au stock ?`;
      if (!window.confirm(question)) return;
      button.disabled = true;
      try {
        await apiFetch(`/api/manage/sales/${sale.id}/${confirm ? "confirm" : "cancel"}`, {
          method: "POST", headers: {"Content-Type": "application/json"},
          body: JSON.stringify({version: sale.version})
        });
        await Promise.all([reloadSales(), reloadInventory(), reloadAudit()]);
      } catch (error) {
        showError(errorBox, error);
        try { await reloadSales(); } catch (_) { /* conserve l'erreur métier initiale */ }
      } finally { button.disabled = false; }
    });

  }

  async function logout() {
    try {
      await fetch("/api/auth/logout", {
        method: "POST", headers: {"X-Defta-Session": "cookie"}
      });
    } finally {
      tokens.clear(); window.location.replace("/login");
    }
  }

  async function initDashboard() {
    if (!tokens.access()) {
      try { await refreshSession(); }
      catch (_) { tokens.clear(); window.location.replace("/login"); return; }
    }
    document.querySelector("#logout-button").addEventListener("click", logout);
    const errorBox = document.querySelector("#dashboard-error");
    audit = window.DeftaAudit.create({apiFetch, textCell, showError, errorBox});
    audit.init();
    sessions = window.DeftaSessions.create({apiFetch, textCell, actionButton, formatDate,
      showError, errorBox, reloadAudit,
      onCurrentRevoked: () => { tokens.clear(); window.location.replace("/login"); }
    });
    sessions.init();
    owners = window.DeftaOwners.create({apiFetch, textCell, actionButton, showError,
      errorBox, updateLibraryOptions, reloadAudit, reloadSessions});
    books = window.DeftaBooks.create({apiFetch, textCell, actionButton, formatDate,
      showError, errorBox, isRoot: () => state.isRoot, reloadInventory, reloadTags, renderTags});
    initEntityForms(errorBox);
    try {
      const user = await apiFetch("/api/auth/me");
      const isRoot = user.role === "SUPER_ADMIN_ROOT";
      state.isRoot = isRoot;
      document.querySelector("#user-name").textContent = sessionStorage.getItem(USERNAME_KEY) || user.id;
      document.querySelector("#role-badge").textContent = isRoot ? "SUPER ADMIN ROOT" : "PROPRIÉTAIRE";
      document.querySelector("#scope-text").textContent = isRoot
        ? "Vous supervisez toutes les librairies et tous les catalogues."
        : "Vous gérez exclusivement les livres de votre librairie.";
      if (isRoot) document.querySelectorAll(".root-only").forEach((element) => { element.hidden = false; });
      if (isRoot) document.querySelectorAll(".root-only-field").forEach((element) => { element.hidden = false; });
      if (!isRoot) document.querySelector(".owner-audit-note").hidden = false;
      document.querySelector("#session-scope-note").textContent = isRoot
        ? "Vue globale des sessions actives de tous les utilisateurs."
        : "Seules les sessions actives de votre compte sont affichées.";
      if (user.passwordChangeRequired) {
        state.passwordChangeRequired = true;
        document.querySelector("#scope-text").textContent = "Vous devez remplacer votre mot de passe temporaire avant de gérer la librairie.";
        document.querySelectorAll("#password-dialog .password-optional-action").forEach((element) => { element.hidden = true; });
        document.querySelector("#password-dialog").showModal();
        return;
      }
      books.init();
      const requests = [
        reloadBooks(),
        reloadSales(),
        reloadInventory(),
        reloadTags(),
        reloadAudit(),
        reloadSessions()
      ];
      if (isRoot) { owners.init(); requests.push(reloadOwners(), reloadOwnerOptions()); }
      await Promise.all(requests);
    } catch (error) {
      if (error.status === 401 || error.message === "Session expirée") {
        tokens.clear(); window.location.replace("/login"); return;
      }
      showError(errorBox, error);
    }
  }

  if (page === "login") initLogin();
  if (page === "dashboard") initDashboard();
})();
