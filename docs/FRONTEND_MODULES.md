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
