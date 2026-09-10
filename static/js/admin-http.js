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
  function message(status, code) {
    if (status === 401) return sessionMessage;
    if (status === 403) return code === 'password_change_required'
      ? 'Changez votre mot de passe pour poursuivre.' : 'Vous ne disposez pas des droits nécessaires.';
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
