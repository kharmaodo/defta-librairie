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
