(() => {
  "use strict";

  const ACCESS_KEY = "defta.accessToken";
  const REFRESH_KEY = "defta.refreshToken";
  const USERNAME_KEY = "defta.username";
  const page = document.body.dataset.page;

  const tokens = {
    access: () => sessionStorage.getItem(ACCESS_KEY),
    refresh: () => sessionStorage.getItem(REFRESH_KEY),
    save: (payload) => {
      sessionStorage.setItem(ACCESS_KEY, payload.accessToken);
      sessionStorage.setItem(REFRESH_KEY, payload.refreshToken);
    },
    clear: () => {
      sessionStorage.removeItem(ACCESS_KEY);
      sessionStorage.removeItem(REFRESH_KEY);
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
    const refreshToken = tokens.refresh();
    if (!refreshToken) throw new Error("Session expirée");
    const response = await fetch("/api/auth/refresh", {
      method: "POST", headers: {"Content-Type": "application/json"},
      body: JSON.stringify({refreshToken})
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

  function showError(element, error) {
    let message = error.message || "Une erreur est survenue.";
    if (error.status === 429 && error.retryAfter) message += ` Réessayez dans ${error.retryAfter} secondes.`;
    element.textContent = message;
    element.hidden = false;
  }

  async function initLogin() {
    const form = document.querySelector("#login-form");
    const errorBox = document.querySelector("#login-error");
    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      errorBox.hidden = true;
      const button = form.querySelector("button[type=submit]");
      button.disabled = true;
      try {
        const data = new FormData(form);
        const response = await fetch("/api/auth/login", {
          method: "POST", headers: {"Content-Type": "application/json"},
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
    document.querySelector("#owner-total").textContent = payload.total;
    const body = document.querySelector("#owners-body");
    body.replaceChildren();
    if (!payload.results.length) {
      const row = body.insertRow(); textCell(row, "Aucun propriétaire", "empty").colSpan = 4; return;
    }
    payload.results.forEach((owner) => {
      const row = body.insertRow();
      textCell(row, owner.username);
      textCell(row, owner.library && owner.library.name);
      textCell(row, owner.status, "pill");
      textCell(row, owner.library && owner.library.status, "pill");
    });
  }

  function renderBooks(payload) {
    document.querySelector("#book-total").textContent = payload.total;
    const body = document.querySelector("#books-body");
    body.replaceChildren();
    if (!payload.results.length) {
      const row = body.insertRow(); textCell(row, "Aucun livre dans ce périmètre", "empty").colSpan = 5; return;
    }
    payload.results.forEach((book) => {
      const row = body.insertRow();
      textCell(row, book.title); textCell(row, book.auteur);
      textCell(row, new Intl.NumberFormat("fr-FR").format(book.price || 0));
      textCell(row, book.tags); textCell(row, book.status, "pill");
    });
  }

  async function logout() {
    const refreshToken = tokens.refresh();
    try {
      if (refreshToken) await fetch("/api/auth/logout", {
        method: "POST", headers: {"Content-Type": "application/json"},
        body: JSON.stringify({refreshToken})
      });
    } finally {
      tokens.clear(); window.location.replace("/login");
    }
  }

  async function initDashboard() {
    if (!tokens.access() || !tokens.refresh()) { window.location.replace("/login"); return; }
    document.querySelector("#logout-button").addEventListener("click", logout);
    const errorBox = document.querySelector("#dashboard-error");
    try {
      const user = await apiFetch("/api/auth/me");
      const isRoot = user.role === "SUPER_ADMIN_ROOT";
      document.querySelector("#user-name").textContent = sessionStorage.getItem(USERNAME_KEY) || user.id;
      document.querySelector("#role-badge").textContent = isRoot ? "SUPER ADMIN ROOT" : "PROPRIÉTAIRE";
      document.querySelector("#scope-text").textContent = isRoot
        ? "Vous supervisez toutes les librairies et tous les catalogues."
        : "Vous gérez exclusivement les livres de votre librairie.";
      if (isRoot) document.querySelectorAll(".root-only").forEach((element) => { element.hidden = false; });
      const requests = [apiFetch("/api/manage/books?offset=0&limit=30").then(renderBooks)];
      if (isRoot) requests.push(apiFetch("/api/admin/owners").then(renderOwners));
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
