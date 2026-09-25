# Gate de release sécurité et résilience — v1.6

Cette procédure réalise l’US-6. L’incrément est intégré à `develop`; les corrections de stabilisation #171 (aperçus de couverture) et #172 (verrouillage des soumissions acceptées) font partie de la recette finale. Elle sépare les contrôles déterministes,
obligatoires pour chaque release, des campagnes de charge qui exigent un
environnement isolé et une mesure de référence.

## Gate obligatoire

`scripts/check-delivery.sh` exécute désormais
`scripts/check-owasp-v1.6.py` avant les tests Go et la détection de courses.
Le contrôle exige :

- les contrats et le backlog OWASP v1.6 ;
- le scénario k6 protégé par `AUTH_LOAD_ALLOW=isolated` ;
- les tests de limite d’authentification ;
- les tests d’isolation inter-librairies ;
- le test de confirmation de vente concurrente ;
- la borne de recherche catalogue.

Les tests Go normaux et `-race`, déjà inclus dans le delivery, exécutent
ensuite les preuves comportementales.

## Campagne isolée à archiver

Avant une release, sur une instance de recette uniquement :

1. démarrer l’API sur une base SQLite temporaire ;
2. relever `/api/health/live` et `/api/health/ready` avant et après ;
3. lancer le scénario k6 d’authentification ;
4. exécuter les tests concurrents Go ;
5. collecter les métriques root, les logs JSON par `request_id`, la taille de
   la base et les erreurs HTTP ;
6. archiver un résumé anonymisé sans jeton, cookie, mot de passe ni donnée
   métier réelle.

## Mesures à consigner

| Mesure | Valeur de référence | Seuil release |
|---|---|---|
| Latence p50/p95/p99 | À mesurer | Défini après baseline |
| Taux de 5xx inattendus | À mesurer | Défini après baseline |
| Refus attendus 401/403/409/429 | À mesurer | Pas de divergence fonctionnelle |
| CPU, mémoire, connexions | À mesurer | Défini après baseline |
| Intégrité SQLite | zéro anomalie | zéro anomalie |
| Santé live/ready | 200 | 200 avant et après |

Aucun seuil chiffré n’est imposé avant une mesure reproductible. Les objectifs
de reprise, la rétention et les alertes d’exploitation restent des décisions
d’environnement.

## Commandes

```sh
./scripts/check-delivery.sh

go test -race -tags fts5 ./internal/middleware ./cmd \
  -run 'TestRateLimiterEnforcesConcurrentLimitPerClient|TestCommercialHTTPRejectsConcurrentCrossLibraryAccess|TestCommercialHTTPConfirmsSaleOnlyOnceUnderConcurrency' \
  -count=1
```

La campagne k6 conserve son garde-fou :

```sh
AUTH_LOAD_ALLOW=isolated AUTH_LOAD_BASE_URL=http://127.0.0.1:8080 \
  k6 run tests/load/auth-rate-limit.js
```
