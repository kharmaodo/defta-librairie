(() => {
  "use strict";

  // The dashboard owns authentication, role state and cross-module refreshes.
  function create({apiFetch, textCell, actionButton, formatDate, showError,
    errorBox, isRoot, reloadInventory, reloadTags, renderTags}) {
    const state = {books: [], bookOffset: 0, bookLimit: 10, bookQuery: ""};
    let initialized = false;
    function renderBooks(payload) {
      state.books = payload.results;
      document.querySelector("#book-total").textContent = payload.total;
      const body = document.querySelector("#books-body");
      body.replaceChildren();
      if (!payload.results.length) {
        const row = body.insertRow(); textCell(row, "Aucun livre dans ce périmètre", "empty").colSpan = 6;
      } else {
        payload.results.forEach((book) => {
          const row = body.insertRow();
          textCell(row, book.title); textCell(row, book.auteur);
          textCell(row, new Intl.NumberFormat("fr-FR").format(book.price || 0));
          textCell(row, book.tags); textCell(row, book.status, "pill");
          const actions = textCell(row, "");
          actions.className = "row-actions";
          actions.replaceChildren(actionButton("Historique", "history-book", book.id), actionButton("Modifier", "edit-book", book.id), actionButton("Supprimer", "delete-book", book.id, true));
        });
      }
      const page = Math.floor(payload.offset / payload.limit) + 1;
      const pages = Math.max(1, Math.ceil(payload.total / payload.limit));
      document.querySelector("#books-page-label").textContent = `Page ${page} sur ${pages}`;
      document.querySelector("#books-previous").disabled = payload.offset === 0;
      document.querySelector("#books-next").disabled = payload.offset + payload.results.length >= payload.total;
    }

    function openBookForm(book = null) {
      const dialog = document.querySelector("#book-dialog");
      const form = document.querySelector("#book-form");
      form.reset();
      form.elements.id.value = book ? book.id : "";
      form.elements.version.value = book ? book.version : "";
      form.elements.title.value = book ? book.title : "";
      form.elements.auteur.value = book ? book.auteur || "" : "";
      form.elements.editeur.value = book ? book.editeur || "" : "";
      form.elements.price.value = book ? book.price : 0;
      form.elements.volume.value = book ? book.volume : 0;
      form.elements.status.value = book ? book.status || "AVAILABLE" : "AVAILABLE";
      form.elements.categorie.value = book ? book.categorie || "" : "";
      form.elements.tags.value = book ? book.tags || "" : "";
      form.elements.coverUrl.value = book ? book.coverUrl || "" : "";
      form.elements.libraryId.value = book ? book.libraryId || "" : "";
      form.elements.libraryId.disabled = Boolean(book);
      if (isRoot()) {
        document.querySelector("#tag-library").value = form.elements.libraryId.value;
        reloadTags().catch(() => renderTags({results: [], total: 0}));
      }
      document.querySelector("#book-form-title").textContent = book ? "Modifier le livre" : "Nouveau livre";
      document.querySelector("#book-form-error").hidden = true;
      dialog.showModal();
    }

    function bookPayload(form) {
      const payload = {
        title: form.elements.title.value,
        auteur: form.elements.auteur.value,
        editeur: form.elements.editeur.value,
        price: Number(form.elements.price.value),
        volume: Number(form.elements.volume.value),
        status: form.elements.status.value,
        tags: form.elements.tags.value,
        categorie: form.elements.categorie.value,
        coverUrl: form.elements.coverUrl.value
      };
      if (isRoot() && form.elements.libraryId.value) payload.libraryId = form.elements.libraryId.value;
      if (form.elements.id.value) payload.version = Number(form.elements.version.value);
      return payload;
    }

    async function openBookHistory(book) {
      const payload = await apiFetch(`/api/manage/books/${book.id}/history?offset=0&limit=100`);
      document.querySelector("#book-history-title").textContent = `Historique · ${book.title}`;
      const body = document.querySelector("#book-history-body");
      body.replaceChildren();
      if (!payload.results.length) {
        const row = body.insertRow(); textCell(row, "Aucune évolution enregistrée", "empty").colSpan = 5;
      } else payload.results.forEach((entry) => {
        const row = body.insertRow();
        textCell(row, formatDate(entry.createdAt));
        textCell(row, entry.actorUsername || entry.actorUserId || "Système");
        textCell(row, entry.action, "pill");
        textCell(row, entry.oldValues || "—", "audit-json");
        textCell(row, entry.newValues || "—", "audit-json");
      });
      document.querySelector("#book-history-dialog").showModal();
    }

    async function reloadBooks() {
      const query = new URLSearchParams({offset: String(state.bookOffset), limit: String(state.bookLimit)});
      if (state.bookQuery) query.set("q", state.bookQuery);
      const payload = await apiFetch(`/api/manage/books?${query}`);
      if (!payload.results.length && state.bookOffset > 0) {
        state.bookOffset = Math.max(0, state.bookOffset - state.bookLimit);
        return reloadBooks();
      }
      renderBooks(payload);
    }

    function init() {
      if (initialized) return;
      initialized = true;
      document.querySelector("#add-book-button").addEventListener("click", () => openBookForm());
      document.querySelector("#book-search-form").addEventListener("submit", async (event) => {
        event.preventDefault();
        state.bookQuery = event.currentTarget.elements.q.value.trim();
        state.bookOffset = 0;
        errorBox.hidden = true;
        try { await reloadBooks(); }
        catch (error) { showError(errorBox, error); }
      });
      document.querySelector("#book-form [name=libraryId]").addEventListener("change", async (event) => {
        if (!isRoot()) return;
        document.querySelector("#tag-library").value = event.currentTarget.value;
        try { await reloadTags(); } catch (error) { showError(errorBox, error); }
      });
      document.querySelector("#books-previous").addEventListener("click", async () => {
        state.bookOffset = Math.max(0, state.bookOffset - state.bookLimit);
        try { await reloadBooks(); } catch (error) { showError(errorBox, error); }
      });
      document.querySelector("#books-next").addEventListener("click", async () => {
        state.bookOffset += state.bookLimit;
        try { await reloadBooks(); } catch (error) { showError(errorBox, error); }
      });
      document.querySelector("#book-form").addEventListener("submit", async (event) => {
        event.preventDefault();
        const form = event.currentTarget;
        const formError = document.querySelector("#book-form-error");
        formError.hidden = true;
        const id = form.elements.id.value;
        if (isRoot() && !id && !form.elements.libraryId.value) {
          showError(formError, new Error("Choisissez la librairie destinataire.")); return;
        }
        try {
          await apiFetch(id ? `/api/manage/books/${id}` : "/api/manage/books", {
            method: id ? "PUT" : "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify(bookPayload(form))
          });
          document.querySelector("#book-dialog").close();
          if (!id) state.bookOffset = 0;
          await Promise.all([reloadBooks(), reloadInventory()]);
        } catch (error) { showError(formError, error); }
      });
      document.querySelector("#books-body").addEventListener("click", async (event) => {
        const button = event.target.closest("button[data-action]");
        if (!button) return;
        const id = Number(button.dataset.id);
        const book = state.books.find((item) => item.id === id);
        if (!book) return;
        if (button.dataset.action === "history-book") {
          try { await openBookHistory(book); } catch (error) { showError(errorBox, error); }
        }
        if (button.dataset.action === "edit-book") openBookForm(book);
        if (button.dataset.action === "delete-book" && window.confirm(`Supprimer « ${book.title} » ?`)) {
          try { await apiFetch(`/api/manage/books/${id}`, {method: "DELETE"}); await Promise.all([reloadBooks(), reloadInventory()]); }
          catch (error) { showError(errorBox, error); }
        }
      });
    }
    return Object.freeze({init, reload: reloadBooks});
  }
  window.DeftaBooks = Object.freeze({create});
})();
