# Requêtes HTTP du frontend

`admin-http.js`, chargé avant les modules de l’administration, expose
`window.DeftaHTTP.request` (Response) et `window.DeftaHTTP.json` (JSON ou null
pour HTTP 204). Statistiques, alertes, historique client, exports CSV et
paramètres l’utilisent. Les autres modules seront migrés progressivement.

Le client conserve les en-têtes fournis, ajoute le jeton courant et refuse les
adresses hors API de même origine et les redirections. Il ne répète aucune
requête, ne supprime pas la session et ne redirige pas l’utilisateur.
Les messages français communs sont affichés dans les zones d’erreur existantes.
Les messages bruts du serveur ne sont pas affichés. Une erreur réseau lors
d’une modification invite à vérifier son enregistrement avant de recommencer.
Les annulations restent des AbortError, afin de préserver la protection contre
les réponses obsolètes dans l’historique client.

## Vérification

Avec Node.js 18 ou plus :

```sh
node --test scripts/test-admin-http.cjs
python3 scripts/check-admin-accessibility.py
```

Recette navigateur après redémarrage et rechargement complet de la page :

- Ouvrir statistiques et alertes, puis filtrer les résultats.
- Ouvrir un historique client, changer de page puis fermer pendant le chargement.
- Télécharger un CSV et vérifier son contenu.
- Charger et enregistrer les paramètres ; vérifier le résultat après rechargement.
- Simuler le mode hors connexion dans les outils réseau du navigateur : un
  message compréhensible doit apparaître. Rétablir le réseau puis recharger.
- Vérifier les messages de session expirée et d’accès refusé avec des sessions
  de test adaptées. Aucun formulaire ne doit être effacé par une redirection.

Les tests Node couvrent le client HTTP, pas le comportement d’un navigateur.
La suite navigateur automatisée reste à réaliser dans la priorité 12.
