# Test d’autorisation concurrente — v1.6

Ce document couvre l’US-3 du backlog OWASP : démontrer qu’un propriétaire ne peut
pas accéder ou modifier les ressources d’une autre librairie, y compris lorsque
les requêtes sont parallèles et que les identifiants sont réels.

## Scénario HTTP automatisé

Le test `TestCommercialHTTPRejectsConcurrentCrossLibraryAccess` initialise :

- deux propriétaires et deux librairies distinctes ;
- une vente confirmée appartenant à la première librairie ;
- 48 requêtes concurrentes exécutées par le propriétaire de la seconde.

Les requêtes alternent entre :

- `GET /api/manage/sales/{id}` ;
- `POST /api/manage/sales/{id}/payments`.

## Résultats attendus

- chaque requête hors périmètre retourne `404 Not Found` ;
- aucune donnée de la vente n’est renvoyée ;
- aucun paiement n’est créé ;
- l’invariant SQLite `COUNT(payments) = 0` est vérifié après la rafale.

Le choix de `404` évite de confirmer à un propriétaire l’existence d’un objet
d’une autre librairie.

## Exécution

```sh
go test -race -tags fts5 ./cmd -run TestCommercialHTTPRejectsConcurrentCrossLibraryAccess -count=1
```

Puis, avant revue :

```sh
go test -race -tags fts5 ./...
git diff --check
```

## Extension de la matrice

Les prochaines itérations de l’US-3 appliqueront la même matrice aux livres,
clients, fournisseurs, achats, retours, couvertures et référentiels globaux.
Chaque test doit vérifier à la fois le statut HTTP, l’absence de fuite de données
et l’absence d’effet de bord en base.

## Limites

Ce test ne remplace pas un essai multi-instance. Il vérifie le cloisonnement
applicatif et la sécurité des décisions d’autorisation sous concurrence dans une
instance et une base SQLite de test.
