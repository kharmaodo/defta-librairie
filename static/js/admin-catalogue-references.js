(() => {
  "use strict";

  function create({apiFetch, showError, errorBox, isRoot, reloadAudit}) {
    const state = {kind: "", items: []};
    const config = {
      categories: {label: "catégorie", plural: "Catégories"},
      publishers: {label: "éditeur", plural: "Éditeurs"}
    };
    const dialog = () => document.querySelector("#catalogue-reference-dialog");
    const form = () => document.querySelector("#catalogue-reference-form");

    function panel(kind) { return document.querySelector(`#${kind}-panel`); }
    function body(kind) { return panel(kind).querySelector("tbody"); }

    function display(row, value) {
      const cell = row.insertCell(); cell.textContent = value || "—"; return cell;
    }

    function render(kind, payload) {
      const items = Array.isArray(payload) ? payload : payload.results || [];
      const target = body(kind); target.replaceChildren();
      if (!items.length) {
        const row = target.insertRow(); const cell = row.insertCell();
        cell.colSpan = 7; cell.className = "empty"; cell.textContent = `Aucun ${config[kind].label} défini`;
        return;
      }
      items.forEach((item) => {
        const row = target.insertRow();
        display(row, item.code); display(row, item.name); display(row, item.ar); display(row, item.fr); display(row, item.en);
        display(row, item.active ? "Actif" : "Désactivé");
        const actions = row.insertCell();
        if (isRoot()) {
          const edit = document.createElement("button"); edit.type = "button"; edit.className = "row-button"; edit.textContent = "Modifier"; edit.dataset.action = "edit"; edit.dataset.id = item.id; actions.append(edit);
          if (item.active) {
            const disable = document.createElement("button"); disable.type = "button"; disable.className = "row-button danger"; disable.textContent = "Désactiver"; disable.dataset.action = "disable"; disable.dataset.id = item.id; actions.append(" ", disable);
          }
        } else actions.textContent = "—";
      });
      state.items = items;
    }

    async function reload(kind) {
      render(kind, await apiFetch(`/api/manage/${kind}`));
    }

    function open(kind, item) {
      state.kind = kind;
      const referenceForm = form(); referenceForm.reset();
      referenceForm.elements.id.value = item ? item.id : "";
      referenceForm.elements.code.value = item?.code || "";
      referenceForm.elements.name.value = item?.name || "";
      referenceForm.elements.ar.value = item?.ar || "";
      referenceForm.elements.fr.value = item?.fr || "";
      referenceForm.elements.en.value = item?.en || "";
      document.querySelector("#catalogue-reference-dialog-title").textContent = item ? `Modifier l’${config[kind].label}` : `Nouvel ${config[kind].label}`;
      dialog().showModal(); referenceForm.elements.code.focus();
    }

    async function submit(event) {
      event.preventDefault();
      const referenceForm = form(); const id = referenceForm.elements.id.value;
      const payload = Object.fromEntries(new FormData(referenceForm).entries());
      delete payload.id;
      try {
        await apiFetch(id ? `/api/manage/${state.kind}/${id}` : `/api/manage/${state.kind}`, {
          method: id ? "PUT" : "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify(payload)
        });
        dialog().close(); await Promise.all([reload(state.kind), reloadAudit()]);
      } catch (error) { showError(referenceForm.querySelector("[data-reference-error]"), error); }
    }

    function initKind(kind) {
      panel(kind).querySelector("[data-reference-create]").addEventListener("click", () => open(kind));
      panel(kind).querySelector("tbody").addEventListener("click", async (event) => {
        const button = event.target.closest("button[data-action]"); if (!button) return;
        const item = state.items.find((value) => String(value.id) === button.dataset.id); if (!item) return;
        if (button.dataset.action === "edit") { open(kind, item); return; }
        if (!window.confirm(`Désactiver ${config[kind].label} « ${item.name} » ?`)) return;
        try {
          await apiFetch(`/api/manage/${kind}/${item.id}/disable`, {method: "POST"});
          await Promise.all([reload(kind), reloadAudit()]);
        } catch (error) { showError(errorBox, error); }
      });
    }

    function init() {
      ["categories", "publishers"].forEach(initKind);
      dialog().querySelector("[data-reference-cancel]").addEventListener("click", () => dialog().close());
      form().addEventListener("submit", submit);
    }
    return Object.freeze({init, reload});
  }
  window.DeftaCatalogueReferences = Object.freeze({create});
})();
