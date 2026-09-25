(() => {
  "use strict";
  let isRoot = false;
  let retryButton = null;
  const moderationStatus = Object.freeze({
    PENDING_SCAN: ["En attente d’analyse", "info"],
    SCANNING: ["Analyse en cours", "info"],
    APPROVED: ["Approuvée", "success"],
    REJECTED: ["Refusée", "danger"],
    REVIEW_REQUIRED: ["Revue manuelle requise", "warning"],
    FAILED: ["Analyse impossible", "failure"]
  });
  const moderationDecision = Object.freeze({
    MODEL_SAFE: ["Contenu conforme", "success"],
    MODEL_UNSAFE: ["Contenu non conforme", "danger"],
    RETRY_REQUESTED: ["Nouvelle analyse demandée", "info"],
    MODERATOR_UNAVAILABLE: ["Service de modération indisponible", "failure"],
    MANUAL_APPROVED: ["Approuvée manuellement", "success"],
    MANUAL_REJECTED: ["Refusée manuellement", "danger"]
  });

  function pill(label, tone, code) {
    const value = document.createElement("span");
    value.className = `pill ${tone}`;
    value.textContent = label;
    value.title = code;
    return value;
  }

  function statusPill(status) {
    const [label, tone] = moderationStatus[status] || ["État inconnu", "neutral"];
    return pill(label, tone, status || "");
  }

  function decisionPill(decision) {
    if (!decision) return null;
    const [label, tone] = moderationDecision[decision] || ["Décision enregistrée", "neutral"];
    return pill(label, tone, decision);
  }

  function appendTextCell(row, value) {
    const cell = row.insertCell();
    cell.textContent = value;
  }

  function appendPillCell(row, value) {
    const cell = row.insertCell();
    if (value) cell.append(value);
    else cell.textContent = "—";
  }

  function actionButton(label, decision, id) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = decision === "REJECT" ? "button ghost danger" : "button primary";
    button.textContent = label;
    button.dataset.decision = decision;
    button.dataset.submissionId = id;
    return button;
  }

  async function decide(button) {
    const decision = button.dataset.decision;
    const label = decision === "APPROVE" ? "approuver" : "refuser";
    if (!window.confirm(`Confirmer : ${label} cette soumission ?`)) return;
    button.disabled = true;
    try {
      await window.DeftaHTTP.json(`/api/manage/book-submissions/${encodeURIComponent(button.dataset.submissionId)}/decision`, {
        method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({decision})
      });
      await load();
    } catch (error) {
      const notice = document.querySelector("#book-submissions-panel [data-submission-notice]");
      notice.textContent = error.message || "La décision n’a pas pu être enregistrée.";
    } finally { button.disabled = false; }
  }

  async function retry(button) {
    const dialog = document.querySelector("#submission-retry-dialog");
    const error = dialog.querySelector("[data-submission-retry-error]");
    retryButton = button;
    error.hidden = true;
    error.textContent = "";
    dialog.showModal();
    dialog.querySelector("button[type=submit]").focus();
  }

  async function confirmRetry() {
    const button = retryButton;
    if (!button) return;
    const dialog = document.querySelector("#submission-retry-dialog");
    const error = dialog.querySelector("[data-submission-retry-error]");
    button.disabled = true;
    try {
      await window.DeftaHTTP.json(`/api/manage/book-submissions/${encodeURIComponent(button.dataset.submissionId)}/retry`, {method: "POST"});
      retryButton = null;
      dialog.close();
      await load();
    } catch (failure) {
      error.textContent = failure.message || "La relance n’a pas pu être enregistrée.";
      error.hidden = false;
    } finally { button.disabled = false; }
  }

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
        appendTextCell(row, item.title);
        appendPillCell(row, statusPill(item.moderationStatus));
        appendTextCell(row, item.moderationScore ?? "—");
        appendPillCell(row, decisionPill(item.decisionCode));
        appendTextCell(row, item.createdBookId || "—");
        const actions = row.insertCell();
        if (isRoot && item.moderationStatus === "REVIEW_REQUIRED") {
          actions.append(actionButton("Approuver", "APPROVE", item.id), actionButton("Refuser", "REJECT", item.id));
        } else if (isRoot && item.moderationStatus === "FAILED") {
          const retryButton = actionButton("Relancer", "RETRY", item.id);
          actions.append(retryButton);
        } else actions.textContent = "—";
      });
      notice.textContent = data.results.length ? "" : "Aucune soumission récente.";
    } catch (error) { notice.textContent = "Impossible de charger les soumissions."; }
  }
  document.addEventListener("DOMContentLoaded", async () => {
    const panel = document.querySelector("#book-submissions-panel");
    if (!panel) return;
    panel.querySelector("button").addEventListener("click", load);
    panel.querySelector("tbody").addEventListener("click", (event) => {
      const button = event.target.closest("button[data-decision]");
      if (button?.dataset.decision === "RETRY") retry(button);
      else if (button) decide(button);
    });
    const retryDialog = document.querySelector("#submission-retry-dialog");
    retryDialog.querySelectorAll("[data-submission-retry-cancel]").forEach((button) => button.addEventListener("click", () => { retryButton = null; retryDialog.close(); }));
    retryDialog.addEventListener("cancel", () => { retryButton = null; });
    retryDialog.querySelector("form").addEventListener("submit", (event) => { event.preventDefault(); confirmRetry(); });
    try {
      const user = await window.DeftaHTTP.json("/api/auth/me");
      isRoot = user.role === "SUPER_ADMIN_ROOT";
    } catch (_) { isRoot = false; }
    load();
  });
})();
