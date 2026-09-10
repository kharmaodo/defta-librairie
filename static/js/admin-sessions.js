(() => {
  "use strict";

  // The dashboard owns authentication and supplies its existing session-aware client.
  function create({apiFetch, textCell, actionButton, formatDate, showError,
    errorBox, reloadAudit, onCurrentRevoked}) {
    const state = {currentSessionId: "", sessionOffset: 0, sessionLimit: 20};
    let initialized = false;
    function renderSessions(payload) {
      state.currentSessionId = payload.currentSessionId;
      document.querySelector("#session-total").textContent = payload.total;
      const body = document.querySelector("#sessions-body");
      body.replaceChildren();
      if (!payload.results.length) {
        const row = body.insertRow(); textCell(row, "Aucune session active", "empty").colSpan = 8;
      } else payload.results.forEach((session) => {
        const row = body.insertRow();
        textCell(row, session.username);
        textCell(row, session.role, "pill");
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
      const page = Math.floor(payload.offset / payload.limit) + 1;
      const pages = Math.max(1, Math.ceil(payload.total / payload.limit));
      document.querySelector("#sessions-page-label").textContent = `Page ${page} sur ${pages}`;
      document.querySelector("#sessions-previous").disabled = payload.offset === 0;
      document.querySelector("#sessions-next").disabled = payload.offset + payload.results.length >= payload.total;
    }

    async function reloadSessions() {
      const form = document.querySelector("#session-filters");
      const query = new URLSearchParams({offset: String(state.sessionOffset), limit: String(state.sessionLimit)});
      ["username", "role", "ipAddress", "userAgent"].forEach((name) => {
        const value = form.elements[name].value.trim();
        if (value) query.set(name, value);
      });
      const payload = await apiFetch(`/api/auth/sessions?${query}`);
      if (!payload.results.length && state.sessionOffset > 0) {
        state.sessionOffset = Math.max(0, state.sessionOffset - state.sessionLimit);
        return reloadSessions();
      }
      renderSessions(payload);
    }

    function init() {
      if (initialized) return;
      initialized = true;
      document.querySelector("#refresh-sessions-button").addEventListener("click", async () => {
        errorBox.hidden = true;
        try { await reloadSessions(); }
        catch (error) { showError(errorBox, error); }
      });
      document.querySelector("#revoke-other-sessions-button").addEventListener("click", async () => {
        if (!window.confirm("Déconnecter tous les autres appareils de ce compte ?")) return;
        errorBox.hidden = true;
        try {
          const result = await apiFetch("/api/auth/sessions/revoke-others", {method: "POST"});
          document.querySelector("#session-scope-note").textContent = `${result.revoked} autre(s) appareil(s) déconnecté(s). La session courante reste active.`;
          state.sessionOffset = 0;
          await Promise.all([reloadSessions(), reloadAudit()]);
        } catch (error) { showError(errorBox, error); }
      });
      document.querySelector("#session-filters").addEventListener("submit", async (event) => {
        event.preventDefault();
        state.sessionOffset = 0;
        try { await reloadSessions(); } catch (error) { showError(errorBox, error); }
      });
      document.querySelector("#sessions-previous").addEventListener("click", async () => {
        state.sessionOffset = Math.max(0, state.sessionOffset - state.sessionLimit);
        try { await reloadSessions(); } catch (error) { showError(errorBox, error); }
      });
      document.querySelector("#sessions-next").addEventListener("click", async () => {
        state.sessionOffset += state.sessionLimit;
        try { await reloadSessions(); } catch (error) { showError(errorBox, error); }
      });
      document.querySelector("#sessions-body").addEventListener("click", async (event) => {
        const button = event.target.closest("button[data-action=revoke-session]");
        if (!button || !window.confirm("Révoquer cette session active ?")) return;
        const current = button.dataset.id === state.currentSessionId;
        try {
          await apiFetch(`/api/auth/sessions/${button.dataset.id}`, {method: "DELETE"});
          if (current) {
            onCurrentRevoked(); return;
          }
          await Promise.all([reloadSessions(), reloadAudit()]);
        } catch (error) { showError(errorBox, error); }
      });

    }
    return Object.freeze({init, reload: reloadSessions});
  }
  window.DeftaSessions = Object.freeze({create});
})();
