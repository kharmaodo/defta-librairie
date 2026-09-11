(() => {
  "use strict";
  function create({apiFetch, showError, errorBox, isRoot, reloadAudit}) {
    const state = {tags: []};
    let initialized = false;
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

    async function reloadTags() {
      const libraryID = isRoot() ? document.querySelector("#tag-library").value : "";
      if (isRoot() && !libraryID) {
        renderTags({results: [], total: 0});
        return;
      }
      const query = new URLSearchParams();
      if (libraryID) query.set("libraryId", libraryID);
      renderTags(await apiFetch(`/api/manage/tags?${query}`));
    }

    function init() {
      if (initialized) return;
      initialized = true;
      document.querySelector("#tag-library").addEventListener("change", async () => {
        try { await reloadTags(); } catch (error) { showError(errorBox, error); }
      });

      document.querySelector("#tag-form").addEventListener("submit", async (event) => {
        event.preventDefault();
        const form = event.currentTarget;
        const payload = {name: form.elements.name.value.trim()};
        if (isRoot()) payload.libraryId = form.elements.libraryId.value;
        if (isRoot() && !payload.libraryId) { showError(errorBox, new Error("Choisissez une librairie.")); return; }
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

    }
    return Object.freeze({init, reload: reloadTags, render: renderTags});
  }
  window.DeftaTags = Object.freeze({create});
})();
