# Requêtes HTTP du frontend

`admin-http.js`, chargé avant les modules de l’administration, expose
`window.DeftaHTTP.request` (Response) et `window.DeftaHTTP.json` (JSON ou null
pour HTTP 204). Statistiques, alertes, historique client, exports CSV et
paramètres l’utilisent, ainsi que les clients, l’approvisionnement, les
paiements/caisses et les retours clients/fournisseurs. Le module principal
`admin-auth.js` utilise aussi ce client, y compris pour la session.

Le client conserve les en-têtes fournis, ajoute le jeton courant et refuse les
adresses hors API de même origine et les redirections. Sur le tableau de bord, un refus 401 peut entraîner un renouvellement partagé
puis une seule reprise. Les erreurs réseau et métier ne sont pas reprises.
Le contrôleur d’authentification conserve les décisions de redirection.
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

## Recette des modules métier

Sur des données de test, vérifier les listes et une création/modification dans
chacun des cinq modules migrés. Vérifier aussi un paiement valide et son solde,
un retour client et son règlement, puis un retour fournisseur et son expédition.

Provoquer un doublon de référence client, un paiement supérieur au reste à payer,
un remboursement supérieur aux encaissements disponibles et une expédition de
retour fournisseur avec stock insuffisant. Chaque refus doit afficher son motif
français et laisser le formulaire disponible pour correction. Vérifier le stock
et les soldes après les opérations pour confirmer qu’un refus n’a rien modifié.

Les messages métier reposent sur une liste explicite de couples statut/code API.
Les erreurs inconnues conservent le message générique du statut. Les erreurs
401, 403 et 5xx restent prioritaires. Aucun texte brut renvoyé par le serveur
n’est affiché et aucune requête n’est répétée automatiquement.

## Session et recette

`authJSON` limite les appels sans Bearer à login, refresh et logout, avec le
marqueur de cookie existant. `enableSessionRefresh` active le renouvellement
sur le tableau de bord ; la page login ne l’active pas. `clearSession` efface
le stockage local et empêche un renouvellement en cours de rétablir le jeton.
Un échec de déconnexion serveur est signalé sur la page de connexion.

La règle de non-répétition des paragraphes précédents s’applique aux erreurs
réseau et métier. Exception explicite : après un refus 401 de l’authentification,
une seule reprise est permise, y compris pour une modification dont le corps
est une chaîne JSON. Aucune boucle après un second 401. Les modules actuels
n’utilisent pas de corps de requête sous forme de flux.

Recette avec comptes de test : connexion correcte/incorrecte, changement de mot
de passe obligatoire, accès propriétaire/root, renouvellement après expiration
du jeton d’accès, plusieurs écrans chargés simultanément, session révoquée,
déconnexion puis rechargement. Vérifier aussi réseau indisponible et refus 403.
Contrôler dans l’onglet réseau le renouvellement partagé et l’absence de reprises
répétées. Tester les cinq écrans récents et les modules extraits.

```sh
node --test scripts/test-admin-*.cjs
go test -tags fts5 ./...
```

Les tests Node ne remplacent pas la vérification des cookies et redirections
réels dans le navigateur.
