(() => {
  "use strict";
  async function load() {
    const box = document.querySelector("#book-submissions-panel");
    if (!box) return;
    const body = box.querySelector("tbody");
    const notice = box.querySelector("[data-submission-notice]");
    try {
      const data = await window.DeftaHTTP.request("/api/manage/book-submissions?limit=30").then((r) => r.json());
      body.replaceChildren();
      data.results.forEach((item) => {
        const row = body.insertRow();
        [item.title, item.moderationStatus, item.moderationScore ?? "—", item.decisionCode || "—", item.createdBookId || "—"].forEach((value) => {
          const cell = row.insertCell(); cell.textContent = value;
        });
      });
      notice.textContent = data.results.length ? "" : "Aucune soumission récente.";
    } catch (error) { notice.textContent = "Impossible de charger les soumissions."; }
  }
  document.addEventListener("DOMContentLoaded", () => {
    const panel = document.querySelector("#book-submissions-panel");
    if (!panel) return;
    panel.querySelector("button").addEventListener("click", load);
    load();
  });
})();