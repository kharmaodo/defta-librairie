(() => {
  "use strict";

  // The dashboard supplies authentication and cross-module refresh callbacks.
  function create({apiFetch, textCell, actionButton, showError, errorBox,
    updateLibraryOptions, reloadAudit, reloadSessions}) {
    const state = {owners: [], ownerOffset: 0, ownerLimit: 10};
    let initialized = false;
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
          actions.replaceChildren(actionButton("Réinitialiser le mot de passe", "reset-owner-password", owner.id), actionButton("Déverrouiller", "unlock-owner", owner.id));
        } else if (owner.status === "DISABLED") {
          actions.replaceChildren(actionButton("Réinitialiser le mot de passe", "reset-owner-password", owner.id), actionButton("Réactiver", "reactivate-owner", owner.id));
        } else {
          actions.replaceChildren(actionButton("Modifier", "edit-owner", owner.id), actionButton("Réinitialiser le mot de passe", "reset-owner-password", owner.id), actionButton("Désactiver", "disable-owner", owner.id, true));
        }
      });
      const page = Math.floor(payload.offset / payload.limit) + 1;
      const pages = Math.max(1, Math.ceil(payload.total / payload.limit));
      document.querySelector("#owners-page-label").textContent = `Page ${page} sur ${pages}`;
      document.querySelector("#owners-previous").disabled = payload.offset === 0;
      document.querySelector("#owners-next").disabled = payload.offset + payload.results.length >= payload.total;
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
      form.querySelectorAll(".create-only").forEach((element) => { element.hidden = Boolean(owner); });
      document.querySelector("#owner-form-error").hidden = true;
      dialog.showModal();
    }

    function openOwnerPasswordReset(owner) {
      const dialog = document.querySelector("#owner-password-reset-dialog");
      const form = document.querySelector("#owner-password-reset-form");
      form.reset();
      form.elements.id.value = owner.id;
      document.querySelector("#owner-password-reset-title").textContent = `Mot de passe temporaire · ${owner.username}`;
      document.querySelector("#owner-password-reset-error").hidden = true;
      dialog.showModal();
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
        if (!payload.results.length) break;
        offset += payload.results.length;
        total = payload.total;
      } while (offset < total);
      updateLibraryOptions(owners);
    }

    function init() {
      if (initialized) return;
      initialized = true;
      document.querySelector("#add-owner-button").addEventListener("click", () => openOwnerForm());
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
      document.querySelector("#owner-password-reset-form").addEventListener("submit", async (event) => {
        event.preventDefault();
        const form = event.currentTarget;
        const formError = document.querySelector("#owner-password-reset-error");
        formError.hidden = true;
        if (form.elements.password.value !== form.elements.confirmation.value) {
          showError(formError, new Error("La confirmation ne correspond pas au mot de passe temporaire."));
          return;
        }
        try {
          await apiFetch(`/api/admin/owners/${form.elements.id.value}/reset-password`, {
            method: "POST", headers: {"Content-Type": "application/json"},
            body: JSON.stringify({password: form.elements.password.value})
          });
          document.querySelector("#owner-password-reset-dialog").close();
          await Promise.all([reloadOwners(), reloadAudit(), reloadSessions()]);
        } catch (error) { showError(formError, error); }
      });
      document.querySelector("#owners-body").addEventListener("click", async (event) => {
        const button = event.target.closest("button[data-action]");
        if (!button) return;
        const owner = state.owners.find((item) => item.id === button.dataset.id);
        if (!owner) return;
        if (button.dataset.action === "edit-owner") openOwnerForm(owner);
        if (button.dataset.action === "reset-owner-password") openOwnerPasswordReset(owner);
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
        if (button.dataset.action === "reactivate-owner" && window.confirm(`Réactiver ${owner.username} et sa librairie ?`)) {
          try {
            await apiFetch(`/api/admin/owners/${owner.id}/reactivate`, {method: "POST"});
            await Promise.all([reloadOwners(), reloadOwnerOptions(), reloadAudit(), reloadSessions()]);
          } catch (error) { showError(errorBox, error); }
        }
      });
    }
    return Object.freeze({init, reload: reloadOwners, reloadOptions: reloadOwnerOptions});
  }
  window.DeftaOwners = Object.freeze({create});
})();
