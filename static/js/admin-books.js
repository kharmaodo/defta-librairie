(() => {
  "use strict";

  // The dashboard owns authentication, role state and cross-module refreshes.
  function create({apiFetch, textCell, actionButton, formatDate, showError,
    errorBox, isRoot, reloadInventory, reloadTags, renderTags}) {
    const state = {books: [], bookOffset: 0, bookLimit: 10, bookQuery: ""};
    let initialized = false;
    let coverTimer = null;
    let coverPreviewURL = null;
    let coverRequest = 0;
    let coverListGeneration = 0;
    const listCoverURLs = new Set();
    let coversAvailable = true;
    const defaultCover = "/static/img/book-cover-placeholder.svg";

    function setCoverImage(image, url) {
      image.src = url || defaultCover;
      image.alt = url ? "Couverture du livre" : "Couverture par défaut";
    }

    async function loadListCover(id, image, generation) {
      if (!coversAvailable) return;
      try {
        const response = await window.DeftaHTTP.request(`/api/manage/books/${id}/cover?variant=thumb&format=jpeg`);
        const blob = await response.blob();
        if (generation !== coverListGeneration) return;
        const url = URL.createObjectURL(blob);
        listCoverURLs.add(url);
        setCoverImage(image, url);
      } catch (error) {
        if (error.status === 503 && error.code === "covers_disabled") coversAvailable = false;
      }
    }
    const coverDialog = () => document.querySelector("#book-dialog");
    const coverStatus = () => document.querySelector("#book-cover-status");
    const coverRetry = () => document.querySelector("#book-cover-retry");

    function clearCoverPreview() {
      if (coverPreviewURL) URL.revokeObjectURL(coverPreviewURL);
      coverPreviewURL = null;
      const image = document.querySelector("#book-cover-preview");
      setCoverImage(image, null);
    }

    function resetCoverState() {
      coverRequest++;
      clearTimeout(coverTimer);
      coverTimer = null;
      clearCoverPreview();
      coverStatus().textContent = "Aucune couverture";
      coverRetry().hidden = true;
    }

    async function loadCoverPreview(id, generation) {
      try {
        const response = await window.DeftaHTTP.request(`/api/manage/books/${id}/cover?variant=thumb&format=jpeg`);
        const blob = await response.blob();
        if (generation !== coverRequest || !coverDialog().open) return;
        clearCoverPreview();
        coverPreviewURL = URL.createObjectURL(blob);
        const image = document.querySelector("#book-cover-preview");
        setCoverImage(image, coverPreviewURL);
      } catch (error) {
        if (generation === coverRequest && error.status !== 404) {
          coverStatus().textContent = "Aperçu indisponible. Vous pouvez réessayer plus tard.";
        }
      }
    }

    function displayCoverStatus(status) {
      const labels = {
        PENDING: "Couverture en attente de traitement.",
        PROCESSING: "Traitement de la couverture en cours.",
        READY: "Couverture disponible.",
        FAILED: "Le traitement de la couverture a échoué."
      };
      coverStatus().textContent = labels[status.status] || "État de la couverture indisponible.";
      coverRetry().hidden = !status.canRetry;
    }

    async function refreshCoverStatus(id, generation = coverRequest) {
      try {
        const status = await apiFetch(`/api/manage/books/${id}/cover/status`);
        if (generation !== coverRequest || !coverDialog().open) return;
        displayCoverStatus(status);
        await loadCoverPreview(id, generation);
        if (generation !== coverRequest || !coverDialog().open) return;
        if (status.status === "PENDING" || status.status === "PROCESSING") {
          clearTimeout(coverTimer);
          coverTimer = setTimeout(() => refreshCoverStatus(id, generation), 3000);
        }
      } catch (error) {
        if (generation !== coverRequest || !coverDialog().open) return;
        if (error.status === 404) {
          coverStatus().textContent = "Aucune couverture";
        } else if (error.status === 503 && error.code === "covers_disabled") {
          coverStatus().textContent = "L’ajout de couvertures est temporairement indisponible.";
          document.querySelector("#book-cover-file").disabled = true;
        } else {
          coverStatus().textContent = "Impossible de vérifier la couverture. Réessayez en rouvrant le livre.";
        }
      }
    }

    function renderBooks(payload) {
      state.books = payload.results;
      document.querySelector("#book-total").textContent = payload.total;
      const body = document.querySelector("#books-body");
      body.replaceChildren();
      const generation = ++coverListGeneration;
      for (const url of listCoverURLs) URL.revokeObjectURL(url);
      listCoverURLs.clear();
      if (!payload.results.length) {
        const row = body.insertRow(); textCell(row, "Aucun livre dans ce périmètre", "empty").colSpan = 6;
      } else {
        payload.results.forEach((book) => {
          const row = body.insertRow();
          const title = textCell(row, "");
          const cover = document.createElement("img");
          cover.className = "book-cover-thumb";
          cover.loading = "lazy";
          setCoverImage(cover, null);
          const name = document.createElement("span");
          name.textContent = book.title;
          title.className = "book-cover-title";
          title.append(cover, name);
          loadListCover(book.id, cover, generation);
          textCell(row, book.auteur);
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
      resetCoverState();
      form.reset();
      form.elements.cover.disabled = false;
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
      loadBookTaxonomy(book).catch(error => showError(document.querySelector("#book-form-error"), error));
      dialog.showModal();
      if (book) refreshCoverStatus(book.id);
    }

    function selectedValues(select) {
      return [...select.selectedOptions].map(option => option.value).filter(Boolean);
    }

    function replaceReferenceOptions(select, values, selected, placeholder) {
      const selectedSet = new Set((selected || []).map(String));
      select.replaceChildren();
      if (placeholder) {
        const option = document.createElement("option");
        option.value = "";
        option.textContent = placeholder;
        select.append(option);
      }
      values.forEach(value => {
        const option = document.createElement("option");
        option.value = String(value.id);
        option.textContent = value.name || value.code;
        option.selected = selectedSet.has(option.value);
        select.append(option);
      });
    }

    async function loadBookTaxonomy(book) {
      const form = document.querySelector("#book-form");
      const libraryId = form.elements.libraryId.value;
      const tagQuery = libraryId ? `?libraryId=${encodeURIComponent(libraryId)}` : "";
      const [categories, publishers, tags] = await Promise.all([
        apiFetch("/api/manage/categories"),
        apiFetch("/api/manage/publishers"),
        isRoot() && !libraryId ? Promise.resolve([]) : apiFetch(`/api/manage/tags${tagQuery}`)
      ]);
      replaceReferenceOptions(form.elements.categoryIds, categories, book?.categoryIds || [], "");
      syncPrimaryCategories(book?.primaryCategoryId);
      replaceReferenceOptions(form.elements.publisherId, publishers,
        book?.publisherId ? [book.publisherId] : [], "Non renseigné");
      replaceReferenceOptions(form.elements.tagIds, tags, book?.tagIds || [], "");
    }

    function syncPrimaryCategories(selectedPrimaryID) {
      const form = document.querySelector("#book-form");
      const selectedCategoryIDs = new Set(selectedValues(form.elements.categoryIds));
      const categories = [...form.elements.categoryIds.options]
        .filter(option => selectedCategoryIDs.has(option.value))
        .map(option => ({id: option.value, name: option.textContent}));
      const current = selectedPrimaryID || form.elements.primaryCategoryId.value;
      replaceReferenceOptions(form.elements.primaryCategoryId, categories,
        current ? [current] : [], "Non renseignée");
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
        coverUrl: form.elements.coverUrl.value,
        tagIds: selectedValues(form.elements.tagIds)
      };
      const categoryIds = selectedValues(form.elements.categoryIds).map(Number);
      if (categoryIds.length) payload.categoryIds = categoryIds;
      if (form.elements.primaryCategoryId.value) payload.primaryCategoryId = Number(form.elements.primaryCategoryId.value);
      if (form.elements.publisherId.value) payload.publisherId = Number(form.elements.publisherId.value);
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
      coverDialog().addEventListener("close", resetCoverState);
      coverRetry().addEventListener("click", async () => {
        const id = document.querySelector("#book-form").elements.id.value;
        if (!id) return;
        coverRetry().disabled = true;
        try {
          const status = await apiFetch(`/api/manage/books/${id}/cover/retry`, {method: "POST"});
          displayCoverStatus(status);
          refreshCoverStatus(id);
        } catch (error) { showError(document.querySelector("#book-form-error"), error); }
        finally { coverRetry().disabled = false; }
      });
      document.querySelector("#add-book-button").addEventListener("click", () => openBookForm());
      document.querySelector("#book-search-form").addEventListener("submit", async (event) => {
        event.preventDefault();
        state.bookQuery = event.currentTarget.elements.q.value.trim();
        state.bookOffset = 0;
        errorBox.hidden = true;
        try { await reloadBooks(); }
        catch (error) { showError(errorBox, error); }
      });
      document.querySelector("#book-form [name=categoryIds]").addEventListener("change", () => syncPrimaryCategories());

      document.querySelector("#book-form [name=libraryId]").addEventListener("change", async (event) => {
        if (!isRoot()) return;
        document.querySelector("#tag-library").value = event.currentTarget.value;
        try {
          await Promise.all([reloadTags(), loadBookTaxonomy()]);
        } catch (error) { showError(errorBox, error); }
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
          const file = form.elements.cover.files[0];
          if (file && (!["image/jpeg", "image/png"].includes(file.type) || file.size > 5 * 1024 * 1024 || !file.size)) {
            throw new Error("Choisissez une image JPEG ou PNG de 5 Mo maximum.");
          }
          if (!id && file) {
            const submission = new FormData();
            const payload = bookPayload(form);
            Object.entries(payload).forEach(([key, value]) => submission.set(key, String(value)));
            submission.set("cover", file);
            const pending = await apiFetch("/api/manage/book-submissions", {method: "POST", body: submission});
            form.elements.cover.value = "";
            formError.textContent = `Soumission ${pending.id} reçue : le livre sera créé après validation de la couverture.`;
            formError.hidden = false;
            return;
          }
          const saved = await apiFetch(id ? `/api/manage/books/${id}` : "/api/manage/books", {
            method: id ? "PUT" : "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify(bookPayload(form))
          });
          form.elements.id.value = saved.id;
          form.elements.version.value = saved.version;
          form.elements.libraryId.disabled = true;
          if (!id) state.bookOffset = 0;
          await Promise.all([reloadBooks(), reloadInventory()]);
          if (file) {
            const data = new FormData();
            data.set("cover", file);
            await apiFetch(`/api/manage/books/${saved.id}/cover`, {method: "POST", body: data});
            form.elements.cover.value = "";
            coverStatus().textContent = "Couverture en attente de traitement.";
            refreshCoverStatus(saved.id);
          } else {
            coverDialog().close();
          }
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
        if (button.dataset.action === "delete-book") {
          await window.DeftaDeleteConfirmation.run({
            trigger: button,
            expected: book.title,
            subject: `le livre « ${book.title} »`,
            execute: async () => {
              await apiFetch(`/api/manage/books/${id}`, {method: "DELETE"});
              await Promise.all([reloadBooks(), reloadInventory()]);
            }
          });
        }
      });
    }
    return Object.freeze({init, reload: reloadBooks});
  }
  window.DeftaBooks = Object.freeze({create});
})();
