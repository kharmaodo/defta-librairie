# Découpage du tableau de bord

## Journal d’audit

`admin-audit.js` contient le rendu, les filtres, la pagination et leur état.
Il expose `window.DeftaAudit.create`, qui reçoit le client `apiFetch` et les
fonctions d’affichage du tableau de bord. `init()` branche les événements une
seule fois ; `reload()` recharge les données, notamment après une opération
métier. Le module est chargé avant `admin-auth.js` dans la page administration.
Il ne lance aucune requête au chargement du script et n’est pas chargé sur login.

L’authentification et le renouvellement de session restent gérés par le client
existant de `admin-auth.js`. Leur migration vers le client HTTP commun reste
un incrément distinct. Les autres responsabilités du module principal restent
à extraire progressivement ; la priorité 12 demeure partielle.

## Contrôles

```sh
node --test scripts/test-admin-audit.cjs scripts/test-admin-http.cjs
python3 scripts/check-admin-accessibility.py
go test -tags fts5 ./...
```

Les tests Node utilisent un DOM simulé ; ils ne remplacent pas une recette
navigateur. Après redémarrage et rechargement complet :

- Se connecter comme root puis comme propriétaire ; vérifier leur portée d’audit.
- Filtrer par acteur, action, ressource, succès/échec et dates.
- Naviguer entre les pages, puis changer de filtre : retour en première page.
- Vérifier un résultat vide et les boutons précédent/suivant.
- Effectuer une opération sur un livre de test et vérifier l’actualisation de l’audit.
- Vérifier la reconnexion, la déconnexion et le changement de mot de passe obligatoire.

Les tests navigateur automatisés restent à développer.

## Sessions

`admin-sessions.js` extrait le rendu, les filtres, la pagination et les actions
de révocation. Sa fabrique reçoit le client HTTP existant, le rechargement de
l’audit et une fonction de déconnexion fournie par le tableau de bord.
Cette dernière est appelée uniquement après la révocation courante réussie.
Les rechargements après les opérations sur les propriétaires sont conservés.

```sh
node --test scripts/test-admin-sessions.cjs scripts/test-admin-audit.cjs scripts/test-admin-http.cjs
```

Recette navigateur avec des comptes de test :

- Vérifier les sessions comme root et comme propriétaire, puis les filtres et pages.
- Annuler une confirmation de révocation : aucune session ne doit être supprimée.
- Révoquer une autre session : la session courante reste utilisable et la liste
  ainsi que l’audit sont actualisés.
- Déconnecter les autres appareils : vérifier le nombre affiché et le maintien
  de la session courante.
- Révoquer la session courante : vérifier le retour à la connexion.
- Simuler un échec réseau de révocation : afficher l’erreur sans déconnexion
  locale automatique ; rétablir le réseau et vérifier l’état réel de la session.

Ces tests Node simulent le DOM et le client HTTP. L’authentification réelle et
les autorisations restent à vérifier avec les tests Go et la recette navigateur.

## Propriétaires

`admin-owners.js` regroupe le rendu, les filtres, la pagination, les formulaires
et les actions administratives. Son état est privé. Le tableau de bord lui
fournit le client HTTP existant et les fonctions de rechargement des sessions,
de l’audit et des sélecteurs de librairie. Les événements sont initialisés
uniquement pour root après le contrôle du changement obligatoire de mot de passe.
Les autorisations restent contrôlées par le serveur.

Le chargement des options conserve les filtres propriétaire/librairie ACTIVE
et parcourt les pages. Une page vide interrompt le parcours si les résultats
diminuent pendant la lecture. Une action sur une ligne devenue inconnue est ignorée.

```sh
node --test scripts/test-admin-owners.cjs scripts/test-admin-sessions.cjs scripts/test-admin-audit.cjs scripts/test-admin-http.cjs
```

Recette navigateur sur des comptes de test :

- Comme root, créer un propriétaire et vérifier sa librairie dans les sélecteurs
  des livres, stocks et ventes. Modifier ses coordonnées sans saisir de mot de
  passe : son mot de passe doit rester utilisable.
- Filtrer et paginer la liste ; vérifier les actions proposées pour ACTIVE,
  LOCKED et DISABLED.
- Annuler une confirmation de désactivation, puis désactiver et réactiver un
  propriétaire de test. Vérifier les sélecteurs après chaque changement.
- Réinitialiser un mot de passe temporaire, vérifier la confirmation, les
  sessions et l’audit, puis la connexion avec changement obligatoire.
- Déverrouiller un compte de test verrouillé et vérifier son nouvel état.
- Comme propriétaire, vérifier le fonctionnement du tableau de bord et
  l’absence des contrôles réservés à root.

Les tests Node simulent le DOM et les réponses HTTP ; ils ne constituent pas
une vérification navigateur ni un test des autorisations du serveur.

## Livres

`admin-books.js` contient la recherche, la pagination, les formulaires,
l’historique et la suppression des livres. Il reçoit le rôle courant par une
fonction et conserve le client HTTP du tableau de bord. La création et la
suppression rechargent le stock, comme la modification. Les tags restent
coordonnés avec la librairie sélectionnée par root. Le catalogue utilisé dans
les ventes reste chargé séparément par `loadSaleBooks` dans le module principal.

```sh
node --test scripts/test-admin-books.cjs scripts/test-admin-owners.cjs scripts/test-admin-sessions.cjs scripts/test-admin-audit.cjs scripts/test-admin-http.cjs
```

Recette navigateur sur des données de test :

- Comme propriétaire, rechercher et paginer, créer un livre puis vérifier son
  stock initial. Modifier le livre et consulter son historique.
- Comme root, créer un livre dans une librairie choisie et vérifier les tags
  proposés ; en modification, la librairie du livre reste verrouillée.
- Modifier le même livre dans deux onglets : la seconde sauvegarde avec une
  version ancienne doit afficher le refus et garder le formulaire ouvert.
- Annuler une suppression, puis supprimer un livre de test supprimable et
  vérifier la liste ainsi que le stock. Vérifier aussi un refus de suppression.
- Ouvrir une vente dans la librairie concernée et vérifier les livres proposés.
- Vérifier qu’une session avec changement de mot de passe obligatoire conserve
  son parcours de changement avant l’accès aux fonctions métier.

Les tests Node emploient un DOM simulé. Les tests Go et la recette navigateur
restent nécessaires pour valider les appels et autorisations réels.

## Stock

`admin-inventory.js` regroupe liste, filtres, pagination, mouvements, seuil et
historique. Il reçoit le client HTTP existant, le rôle courant et le rechargement
de l’audit. Les livres et ventes continuent d’appeler le rechargement du stock.
Les versions et méthodes HTTP des opérations sont conservées.

```sh
node --test scripts/test-admin-*.cjs
```

Recette navigateur avec un livre de test :

- Vérifier les filtres de statut, les pages et la sélection de librairie comme root.
- Effectuer une entrée puis une sortie ; contrôler quantité, historique et audit.
- Effectuer un ajustement avec motif et changer le seuil ; vérifier le statut affiché.
- Provoquer une sortie supérieure au stock ou un conflit de version entre deux
  onglets : le refus doit laisser le formulaire ouvert.
- Créer un livre puis confirmer/annuler une vente de test : vérifier le rechargement
  du stock et les quantités attendues.
- Comme propriétaire, vérifier que seuls les stocks de sa librairie sont affichés.

Les tests Node simulent le DOM et le client HTTP ; tests Go et recette navigateur
restent nécessaires pour vérifier le fonctionnement réel.

## Ventes

`admin-sales.js` contient liste, filtres, formulaires, catalogues des livres et
clients, détails, impression et transitions des ventes. Le tableau de bord
fournit le client HTTP, le rôle courant et les rechargements stock/audit.
`loadSaleBooks` est désormais dans ce module ; les paragraphes précédents
concernant son emplacement décrivaient les étapes antérieures.

```sh
node --test scripts/test-admin-*.cjs
```

Recette navigateur sur des données de test :

- Créer puis modifier un brouillon avec plusieurs livres et un client rattaché.
  Vérifier les quantités, le total estimé et les détails enregistrés.
- Comme root, changer de librairie et vérifier le renouvellement des livres,
  clients et lignes du formulaire ; comme propriétaire, vérifier sa portée.
- Confirmer une vente puis l’annuler : contrôler stock, statut et audit.
- Provoquer un stock insuffisant ou une version ancienne : conserver le refus
  initial affiché et vérifier qu’aucune transition supplémentaire n’est envoyée.
- Annuler une confirmation de suppression, puis supprimer un brouillon de test.
- Filtrer et paginer, consulter les détails et vérifier l’impression avec les
  coordonnées de la librairie de la vente.

Les  tests Node utilisent un DOM et un client HTTP simulés. Le parcours complet
navigateur, notamment les listes de choix et l’impression, reste à valider.

## Tags

`admin-tags.js` regroupe affichage, suggestions, création et suppression.
Les fonctions `reload` et `render` restent accessibles aux formulaires des
livres par le tableau de bord. Le rôle courant et le client HTTP sont injectés.

```sh
node --test scripts/test-admin-*.cjs
```

Recette navigateur : créer puis supprimer un tag de test comme propriétaire,
vérifier les suggestions dans un livre, puis comme root changer de librairie
et vérifier la portée des tags. Sans librairie sélectionnée, root doit voir
les suggestions vides et ne pas pouvoir créer de tag. Annuler une suppression
et provoquer un refus de création : vérifier que la saisie reste disponible.

Les tests Node simulent le DOM et les réponses HTTP. La recette navigateur
et les tests Go restent nécessaires pour valider le fonctionnement réel.
