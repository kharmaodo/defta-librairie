(() => {
  "use strict";

  const ACCESS_KEY = "defta.accessToken";
  const USERNAME_KEY = "defta.username";
  const page = document.body.dataset.page;
  const state = {isRoot: false, passwordChangeRequired: false};

  const tokens = {
    access: () => sessionStorage.getItem(ACCESS_KEY),
    save: (payload) => {
      sessionStorage.setItem(ACCESS_KEY, payload.accessToken);
    },
    clear: () => window.DeftaHTTP.clearSession()
  };

  const refreshSession = () => window.DeftaHTTP.refreshSession();
  const apiFetch = (path, options) => window.DeftaHTTP.json(path, options);
  if (page === "dashboard") window.DeftaHTTP.enableSessionRefresh();

  let audit, sessions, owners, books, inventory, sales, tags;
  const reloadTags = () => tags.reload();
  const renderTags = payload => tags.render(payload);
  const reloadSales = () => sales.reload();
  const reloadInventory = () => inventory.reload();
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
    if (new URLSearchParams(window.location.search).get("logoutFailed") === "1") {
      showError(errorBox, new Error("La déconnexion du serveur n’a pas été confirmée. Reconnectez-vous pour révoquer les sessions si nécessaire."));
    }
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
        const payload = await window.DeftaHTTP.authJSON("login", {
          username: data.get("username"), password: data.get("password")
        });
        if (!payload || typeof payload.accessToken !== "string" || !payload.accessToken || !payload.user?.username) {
          throw new Error("Réponse de connexion invalide.");
        }
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

  function initEntityForms(errorBox) {
    const passwordDialog = document.querySelector("#password-dialog");
    passwordDialog.addEventListener("cancel", (event) => {
      if (state.passwordChangeRequired) event.preventDefault();
    });
    document.querySelectorAll("[data-close]").forEach((button) => button.addEventListener("click", () => document.querySelector(`#${button.dataset.close}`).close()));
    document.querySelector("#change-password-button").addEventListener("click", () => {
      const form = document.querySelector("#password-form");
      form.reset();
      document.querySelector("#password-form-error").hidden = true;
      document.querySelector("#password-dialog").showModal();
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

  }

  async function logout() {
    try {
      tokens.clear();
      await window.DeftaHTTP.authJSON("logout");
    } catch (_) {
      window.location.replace("/login?logoutFailed=1"); return;
    } finally {
      tokens.clear();
    }
    window.location.replace("/login");
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
    inventory = window.DeftaInventory.create({apiFetch, textCell, actionButton, formatDate,
      showError, errorBox, isRoot: () => state.isRoot, reloadAudit});
    sales = window.DeftaSales.create({apiFetch, textCell, actionButton, formatDate,
      showError, errorBox, isRoot: () => state.isRoot, reloadInventory, reloadAudit});
    tags = window.DeftaTags.create({apiFetch, showError, errorBox,
      isRoot: () => state.isRoot, reloadAudit});
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
      inventory.init();
      sales.init();
      tags.init();
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
