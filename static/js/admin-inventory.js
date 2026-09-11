(() => {
  "use strict";

  // Authentication and cross-module refreshes remain owned by the dashboard.
  function create({apiFetch, textCell, actionButton, formatDate, showError,
    errorBox, isRoot, reloadAudit}) {
    const state = {inventory: [], inventoryOffset: 0, inventoryLimit: 10};
    let initialized = false;
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
      const libraryID = isRoot() ? form.elements.libraryId.value : "";
      if (status) query.set("status", status);
      if (libraryID) query.set("libraryId", libraryID);
      const payload = await apiFetch(`/api/manage/inventory?${query}`);
      if (!payload.results.length && state.inventoryOffset > 0) {
        state.inventoryOffset = Math.max(0, state.inventoryOffset - state.inventoryLimit);
        return reloadInventory();
      }
      renderInventory(payload);
    }

    function init() {
      if (initialized) return;
      initialized = true;
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
    }
    return Object.freeze({init, reload: reloadInventory});
  }
  window.DeftaInventory = Object.freeze({create});
})();
