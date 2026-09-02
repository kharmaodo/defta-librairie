(() => {
  "use strict";

  const ACCESS_KEY = "defta.accessToken";
  const USERNAME_KEY = "defta.username";
  const page = document.body.dataset.page;
  const state = {isRoot: false, owners: [], ownerOptions: [], books: [], tags: [], currentSessionId: "", ownerOffset: 0, ownerLimit: 10, bookOffset: 0, bookLimit: 10, bookQuery: "", auditOffset: 0, auditLimit: 20};

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

  function renderOwners(payload) {
    state.owners = payload.results;
    document.querySelector("#owner-total").textContent = payload.total;
    const body = document.querySelector("#owners-body");
    body.replaceChildren();
    if (!payload.results.length) {
      const row = body.insertRow(); textCell(row, "Aucun propriétaire", "empty").colSpan = 5;
    } else payload.results.forEach((owner) => {
      const row = body.insertRow();
      textCell(row, owner.username);
      textCell(row, owner.library && owner.library.name);
      textCell(row, owner.status, "pill");
      textCell(row, owner.library && owner.library.status, "pill");
      const actions = textCell(row, "");
      actions.className = "row-actions";
      if (owner.status === "LOCKED") {
        actions.replaceChildren(actionButton("Déverrouiller", "unlock-owner", owner.id));
      } else {
        actions.replaceChildren(actionButton("Modifier", "edit-owner", owner.id), actionButton("Désactiver", "disable-owner", owner.id, true));
      }
    });
    const page = Math.floor(payload.offset / payload.limit) + 1;
    const pages = Math.max(1, Math.ceil(payload.total / payload.limit));
    document.querySelector("#owners-page-label").textContent = `Page ${page} sur ${pages}`;
    document.querySelector("#owners-previous").disabled = payload.offset === 0;
    document.querySelector("#owners-next").disabled = payload.offset + payload.results.length >= payload.total;
  }

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

  function renderAudit(payload) {
    document.querySelector("#audit-total").textContent = payload.total;
    const body = document.querySelector("#audit-body");
    body.replaceChildren();
    if (!payload.results.length) {
      const row = body.insertRow(); textCell(row, "Aucun événement correspondant", "empty").colSpan = 6;
    } else payload.results.forEach((entry) => {
      const row = body.insertRow();
      const parsedDate = new Date(entry.createdAt);
      textCell(row, Number.isNaN(parsedDate.getTime()) ? entry.createdAt : parsedDate.toLocaleString("fr-FR"));
      textCell(row, entry.action);
      textCell(row, entry.actorUsername || entry.actorUserId || "Système");
      textCell(row, `${entry.resourceType}${entry.resourceId ? ` · ${entry.resourceId}` : ""}`);
      textCell(row, entry.success ? "Succès" : "Échec", entry.success ? "pill" : "pill failure");
      textCell(row, entry.ipAddress);
    });
    const page = Math.floor(payload.offset / payload.limit) + 1;
    const pages = Math.max(1, Math.ceil(payload.total / payload.limit));
    document.querySelector("#audit-page-label").textContent = `Page ${page} sur ${pages}`;
    document.querySelector("#audit-previous").disabled = payload.offset === 0;
    document.querySelector("#audit-next").disabled = payload.offset + payload.results.length >= payload.total;
  }

  function renderSessions(payload) {
    state.currentSessionId = payload.currentSessionId;
    document.querySelector("#session-total").textContent = payload.total;
    const body = document.querySelector("#sessions-body");
    body.replaceChildren();
    if (!payload.results.length) {
      const row = body.insertRow(); textCell(row, "Aucune session active", "empty").colSpan = 7; return;
    }
    payload.results.forEach((session) => {
      const row = body.insertRow();
      textCell(row, session.username);
      textCell(row, session.userAgent, "device");
      textCell(row, session.ipAddress);
      textCell(row, formatDate(session.createdAt));
      textCell(row, formatDate(session.expiresAt));
      const current = session.id === payload.currentSessionId;
      textCell(row, current ? "Session courante" : "Active", "pill");
      const actions = textCell(row, "");
      actions.className = "row-actions";
      actions.replaceChildren(actionButton(current ? "Révoquer et quitter" : "Révoquer", "revoke-session", session.id, true));
    });
  }

  function formatDate(value) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value || "—" : date.toLocaleString("fr-FR");
  }

  function updateLibraryOptions() {
    document.querySelectorAll("#book-form [name=libraryId], #tag-form [name=libraryId]").forEach((select) => {
      const selected = select.value;
      select.replaceChildren();
      const placeholder = document.createElement("option");
      placeholder.value = ""; placeholder.textContent = "Choisir une librairie";
      select.append(placeholder);
      state.ownerOptions.forEach((owner) => {
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

  function openOwnerForm(owner = null) {
    const dialog = document.querySelector("#owner-dialog");
    const form = document.querySelector("#owner-form");
    form.reset();
    form.elements.id.value = owner ? owner.id : "";
    form.elements.username.value = owner ? owner.username : "";
    form.elements.email.value = owner ? owner.email || "" : "";
    form.elements.libraryName.value = owner ? owner.library.name : "";
    form.elements.libraryDescription.value = owner ? owner.library.description || "" : "";
    form.elements.status.value = owner ? owner.status : "ACTIVE";
    form.elements.libraryStatus.value = owner ? owner.library.status : "ACTIVE";
    form.elements.password.required = !owner;
    document.querySelector("#owner-form-title").textContent = owner ? "Modifier le propriétaire" : "Nouveau propriétaire";
    document.querySelector("#owner-password-help").textContent = owner ? "laisser vide pour conserver l’actuel" : "12 caractères minimum";
    form.querySelectorAll(".edit-only").forEach((element) => { element.hidden = !owner; });
    document.querySelector("#owner-form-error").hidden = true;
    dialog.showModal();
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
    if (state.isRoot) {
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
    if (state.isRoot && form.elements.libraryId.value) payload.libraryId = form.elements.libraryId.value;
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

  async function reloadOwners() {
    const form = document.querySelector("#owner-filters");
    const query = new URLSearchParams({offset: String(state.ownerOffset), limit: String(state.ownerLimit)});
    ["q", "status", "libraryStatus"].forEach((name) => {
      const value = form.elements[name].value.trim();
      if (value) query.set(name, value);
    });
    const payload = await apiFetch(`/api/admin/owners?${query}`);
    if (!payload.results.length && state.ownerOffset > 0) {
      state.ownerOffset = Math.max(0, state.ownerOffset - state.ownerLimit);
      return reloadOwners();
    }
    renderOwners(payload);
  }

  async function reloadOwnerOptions() {
    const owners = [];
    let offset = 0;
    let total = 0;
    do {
      const payload = await apiFetch(`/api/admin/owners?status=ACTIVE&libraryStatus=ACTIVE&offset=${offset}&limit=100`);
      owners.push(...payload.results);
      offset += payload.results.length;
      total = payload.total;
    } while (offset < total);
    state.ownerOptions = owners;
    updateLibraryOptions();
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

  async function reloadAudit() {
    const form = document.querySelector("#audit-filters");
    const query = new URLSearchParams({offset: String(state.auditOffset), limit: String(state.auditLimit)});
    ["actor", "action", "resourceType", "resourceId", "success"].forEach((name) => {
      const value = form.elements[name].value.trim();
      if (value) query.set(name, value);
    });
    ["from", "to"].forEach((name) => {
      const value = form.elements[name].value;
      if (value) query.set(name, new Date(value).toISOString());
    });
    const payload = await apiFetch(`/api/audit-logs?${query}`);
    if (!payload.results.length && state.auditOffset > 0) {
      state.auditOffset = Math.max(0, state.auditOffset - state.auditLimit);
      return reloadAudit();
    }
    renderAudit(payload);
  }

  async function reloadSessions() {
    renderSessions(await apiFetch("/api/auth/sessions?offset=0&limit=30"));
  }

  function initEntityForms(errorBox) {
    document.querySelectorAll("[data-close]").forEach((button) => button.addEventListener("click", () => document.querySelector(`#${button.dataset.close}`).close()));
    document.querySelector("#add-book-button").addEventListener("click", () => openBookForm());
    document.querySelector("#add-owner-button").addEventListener("click", () => openOwnerForm());
    document.querySelector("#change-password-button").addEventListener("click", () => {
      const form = document.querySelector("#password-form");
      form.reset();
      document.querySelector("#password-form-error").hidden = true;
      document.querySelector("#password-dialog").showModal();
    });
    document.querySelector("#audit-filters").addEventListener("submit", async (event) => {
      event.preventDefault();
      state.auditOffset = 0;
      errorBox.hidden = true;
      try { await reloadAudit(); }
      catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#audit-previous").addEventListener("click", async () => {
      state.auditOffset = Math.max(0, state.auditOffset - state.auditLimit);
      try { await reloadAudit(); } catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#audit-next").addEventListener("click", async () => {
      state.auditOffset += state.auditLimit;
      try { await reloadAudit(); } catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#owner-filters").addEventListener("submit", async (event) => {
      event.preventDefault();
      state.ownerOffset = 0;
      errorBox.hidden = true;
      try { await reloadOwners(); } catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#owners-previous").addEventListener("click", async () => {
      state.ownerOffset = Math.max(0, state.ownerOffset - state.ownerLimit);
      try { await reloadOwners(); } catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#owners-next").addEventListener("click", async () => {
      state.ownerOffset += state.ownerLimit;
      try { await reloadOwners(); } catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#book-search-form").addEventListener("submit", async (event) => {
      event.preventDefault();
      state.bookQuery = event.currentTarget.elements.q.value.trim();
      state.bookOffset = 0;
      errorBox.hidden = true;
      try { await reloadBooks(); }
      catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#tag-library").addEventListener("change", async () => {
      try { await reloadTags(); } catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#book-form [name=libraryId]").addEventListener("change", async (event) => {
      if (!state.isRoot) return;
      document.querySelector("#tag-library").value = event.currentTarget.value;
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
    document.querySelector("#books-previous").addEventListener("click", async () => {
      state.bookOffset = Math.max(0, state.bookOffset - state.bookLimit);
      try { await reloadBooks(); } catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#books-next").addEventListener("click", async () => {
      state.bookOffset += state.bookLimit;
      try { await reloadBooks(); } catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#refresh-sessions-button").addEventListener("click", async () => {
      errorBox.hidden = true;
      try { await reloadSessions(); }
      catch (error) { showError(errorBox, error); }
    });
    document.querySelector("#sessions-body").addEventListener("click", async (event) => {
      const button = event.target.closest("button[data-action=revoke-session]");
      if (!button || !window.confirm("Révoquer cette session active ?")) return;
      const current = button.dataset.id === state.currentSessionId;
      try {
        await apiFetch(`/api/auth/sessions/${button.dataset.id}`, {method: "DELETE"});
        if (current) {
          tokens.clear(); window.location.replace("/login"); return;
        }
        await Promise.all([reloadSessions(), reloadAudit()]);
      } catch (error) { showError(errorBox, error); }
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

    document.querySelector("#owner-form").addEventListener("submit", async (event) => {
      event.preventDefault();
      const form = event.currentTarget;
      const formError = document.querySelector("#owner-form-error");
      formError.hidden = true;
      const id = form.elements.id.value;
      const password = form.elements.password.value;
      const payload = id ? {
        username: form.elements.username.value, email: form.elements.email.value,
        status: form.elements.status.value,
        library: {name: form.elements.libraryName.value, description: form.elements.libraryDescription.value, status: form.elements.libraryStatus.value}
      } : {
        username: form.elements.username.value, email: form.elements.email.value, password,
        library: {name: form.elements.libraryName.value, description: form.elements.libraryDescription.value}
      };
      if (id && password) payload.password = password;
      try {
        await apiFetch(id ? `/api/admin/owners/${id}` : "/api/admin/owners", {
          method: id ? "PATCH" : "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify(payload)
        });
        document.querySelector("#owner-dialog").close();
        if (!id) state.ownerOffset = 0;
        await Promise.all([reloadOwners(), reloadOwnerOptions()]);
      } catch (error) { showError(formError, error); }
    });

    document.querySelector("#book-form").addEventListener("submit", async (event) => {
      event.preventDefault();
      const form = event.currentTarget;
      const formError = document.querySelector("#book-form-error");
      formError.hidden = true;
      const id = form.elements.id.value;
      if (state.isRoot && !id && !form.elements.libraryId.value) {
        showError(formError, new Error("Choisissez la librairie destinataire.")); return;
      }
      try {
        await apiFetch(id ? `/api/manage/books/${id}` : "/api/manage/books", {
          method: id ? "PUT" : "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify(bookPayload(form))
        });
        document.querySelector("#book-dialog").close();
        if (!id) state.bookOffset = 0;
        await reloadBooks();
      } catch (error) { showError(formError, error); }
    });

    document.querySelector("#owners-body").addEventListener("click", async (event) => {
      const button = event.target.closest("button[data-action]");
      if (!button) return;
      const owner = state.owners.find((item) => item.id === button.dataset.id);
      if (button.dataset.action === "edit-owner") openOwnerForm(owner);
      if (button.dataset.action === "disable-owner" && window.confirm(`Désactiver ${owner.username} et sa librairie ?`)) {
        try { await apiFetch(`/api/admin/owners/${owner.id}`, {method: "DELETE"}); await Promise.all([reloadOwners(), reloadOwnerOptions()]); }
        catch (error) { showError(errorBox, error); }
      }
      if (button.dataset.action === "unlock-owner" && window.confirm(`Déverrouiller le compte ${owner.username} ?`)) {
        try {
          await apiFetch(`/api/admin/owners/${owner.id}/unlock`, {method: "POST"});
          await Promise.all([reloadOwners(), reloadOwnerOptions(), reloadAudit(), reloadSessions()]);
        } catch (error) { showError(errorBox, error); }
      }
    });

    document.querySelector("#books-body").addEventListener("click", async (event) => {
      const button = event.target.closest("button[data-action]");
      if (!button) return;
      const id = Number(button.dataset.id);
      const book = state.books.find((item) => item.id === id);
      if (button.dataset.action === "history-book") {
        try { await openBookHistory(book); } catch (error) { showError(errorBox, error); }
      }
      if (button.dataset.action === "edit-book") openBookForm(book);
      if (button.dataset.action === "delete-book" && window.confirm(`Supprimer « ${book.title} » ?`)) {
        try { await apiFetch(`/api/manage/books/${id}`, {method: "DELETE"}); await reloadBooks(); }
        catch (error) { showError(errorBox, error); }
      }
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
      const requests = [
        reloadBooks(),
        reloadTags(),
        reloadAudit(),
        apiFetch("/api/auth/sessions?offset=0&limit=30").then(renderSessions)
      ];
      if (isRoot) requests.push(reloadOwners(), reloadOwnerOptions());
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
