(() => {
  "use strict";

  // Authentication, stock and audit are coordinated by the dashboard.
  function create({apiFetch, textCell, actionButton, formatDate, showError,
    errorBox, isRoot, reloadInventory, reloadAudit}) {
    const state = {sales: [], saleBooks: [], saleCustomers: [], saleOffset: 0, saleLimit: 10};
    let initialized = false;
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

    function formatMoney(value) {
      return new Intl.NumberFormat("fr-FR", {style: "currency", currency: "XOF", maximumFractionDigits: 0}).format(value || 0);
    }

    async function loadSaleBooks(libraryID) {
      if (isRoot() && !libraryID) {
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
      if (isRoot() && !libraryID) {
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
      if (isRoot() && form.elements.libraryId.value) payload.libraryId = form.elements.libraryId.value;
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

    async function reloadSales() {
      const form = document.querySelector("#sale-filters");
      const query = new URLSearchParams({offset: String(state.saleOffset), limit: String(state.saleLimit)});
      const status = form.elements.status.value;
      const libraryID = isRoot() ? form.elements.libraryId.value : "";
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

    function init() {
      if (initialized) return;
      initialized = true;
      document.querySelector("#add-sale-button").addEventListener("click", async () => {
        try { await openSaleForm(); } catch (error) { showError(errorBox, error); }
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
        if (isRoot() && !form.elements.id.value && !payload.libraryId) {
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
    return Object.freeze({init, reload: reloadSales});
  }
  window.DeftaSales = Object.freeze({create});
})();
