(() => {
  'use strict';
  class APIError extends Error {
    constructor(message, status = 0, code = '') {
      super(message);
      this.name = 'APIError';
      this.status = status;
      this.code = code;
    }
  }
  const sessionMessage = 'Session expirée : reconnectez-vous puis actualisez cette page.';
  // Only known status/code pairs produce business messages; never display server text.
  const businessMessages = {
    "409": {
      "customer_conflict": "Cette référence client existe déjà.",
      "supplier_conflict": "Ce fournisseur existe déjà dans cette librairie.",
      "cash_register_conflict": "Une caisse porte déjà ce nom.",
      "payment_reference_conflict": "Cette référence de paiement existe déjà.",
      "return_settlement_conflict": "Cette référence de règlement de retour existe déjà.",
      "payment_has_issued_refunds": "Ce paiement couvre des remboursements déjà émis et ne peut pas être annulé.",
      "payment_exceeds_balance": "Le paiement dépasse le reste à payer de la vente.",
      "refund_exceeds_payments": "Le remboursement dépasse les encaissements disponibles pour cette vente.",
      "return_settlement_exceeds_balance": "Le règlement dépasse le solde disponible du retour.",
      "return_quantity_exceeded": "La quantité retournée dépasse la quantité encore retournable de la vente.",
      "supplier_return_quantity_exceeded": "La quantité retournée dépasse la quantité encore retournable de l’achat réceptionné.",
      "supplier_return_insufficient_stock": "Stock insuffisant pour expédier ce retour fournisseur.",
      "payment_context_unavailable": "La vente ou la caisse est indisponible.",
      "customer_state_conflict": "Opération impossible dans l’état actuel : client.",
      "supplier_state_conflict": "Opération impossible dans l’état actuel : fournisseur.",
      "cash_register_state_conflict": "Opération impossible dans l’état actuel : caisse.",
      "payment_state_conflict": "Opération impossible dans l’état actuel : paiement.",
      "return_settlement_state_conflict": "Opération impossible dans l’état actuel : règlement de retour.",
      "purchase_not_editable": "Opération impossible dans l’état actuel : achat.",
      "customer_return_not_editable": "Opération impossible dans l’état actuel : retour client.",
      "supplier_return_not_editable": "Opération impossible dans l’état actuel : retour fournisseur."
    },
    "422": {
      "library_unavailable": "Cette librairie est indisponible.",
      "supplier_unavailable": "Ce fournisseur est indisponible pour cet achat.",
      "purchase_unavailable": "Cet achat est indisponible pour un retour fournisseur.",
      "sale_unavailable": "Cette vente est indisponible pour un retour client.",
      "return_settlement_unavailable": "Le contexte du retour ne permet pas ce règlement."
    },
    "400": {
      "invalid_purchase_book": "Un livre de cet achat est invalide ou inaccessible.",
      "invalid_purchase_line": "La ligne sélectionnée ne permet pas ce retour fournisseur.",
      "invalid_return_line": "La ligne sélectionnée ne permet pas ce retour client."
    }
  };
  function message(status, code) {
    if (status === 401) return sessionMessage;
    if (status === 403) return code === 'password_change_required'
      ? 'Changez votre mot de passe pour poursuivre.' : 'Vous ne disposez pas des droits nécessaires.';
    const known = businessMessages[status];
    if (known && Object.prototype.hasOwnProperty.call(known, code)) return known[code];
    if (status === 409) return 'Les données ont changé : rechargez-les avant de réessayer.';
    if (status === 422 && code === 'export_too_large') return 'Plus de 10 000 lignes : affinez les filtres.';
    if (status === 400 || status === 422) return 'Données invalides : vérifiez les champs et les filtres.';
    if (status === 404) return 'Ressource introuvable ou inaccessible.';
    if (status === 429) return 'Trop de demandes : patientez avant de réessayer.';
    if (status >= 500) return 'Service temporairement indisponible. Réessayez plus tard.';
    return `La demande a échoué (HTTP ${status}).`;
  }
  function transportError(error) {
    if (error.name === 'AbortError' || error instanceof APIError) return error;
    return new APIError('Connexion interrompue. Vérifiez votre réseau. Avant de répéter une modification, vérifiez si elle a été enregistrée.');
  }
  async function request(path, options = {}) {
    const url = new URL(path, window.location.origin);
    if (url.origin !== window.location.origin || !url.pathname.startsWith('/api/') || url.username || url.password) {
      throw new APIError('Adresse API non autorisée.');
    }
    const token = sessionStorage.getItem('defta.accessToken');
    if (!token) throw new APIError(sessionMessage, 401, 'unauthorized');
    const headers = new Headers(options.headers);
    headers.set('Authorization', `Bearer ${token}`);
    if (typeof options.body === 'string' && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
    let response;
    try {
      response = await fetch(url.href, {...options, headers, cache: 'no-store', redirect: 'error'});
    } catch (error) { throw transportError(error); }
    if (!response.ok) {
      let code = '';
      try {
        const body = await response.json();
        if (typeof body?.error === 'string') code = body.error;
      } catch (error) { if (error.name === 'AbortError') throw error; }
      throw new APIError(message(response.status, code), response.status, code);
    }
    return response;
  }
  async function json(path, options) {
    const response = await request(path, options);
    if (response.status === 204) return null;
    if (!/^application\/json(?:\s*;|$)/i.test(response.headers.get('Content-Type') || '')) {
      throw new APIError('Réponse inattendue du serveur.', response.status);
    }
    try { return await response.json(); }
    catch (error) {
      if (error instanceof SyntaxError) throw new APIError('Réponse JSON invalide du serveur.', response.status);
      throw transportError(error);
    }
  }
  window.DeftaHTTP = Object.freeze({request, json, APIError});
})();
