(() => {
  "use strict";

  // Instantiated by the dashboard after authentication setup, never on login.
  function create({apiFetch, textCell, showError, errorBox}) {
    const state = {auditOffset: 0, auditLimit: 20};
    let initialized = false;
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

    function init() {
      if (initialized) return;
      initialized = true;
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
    }
    return Object.freeze({init, reload: reloadAudit});
  }
  window.DeftaAudit = Object.freeze({create});
})();
