# Performance du dashboard — v1.2.0

## Objectif mesuré

L’optimisation cible les lectures JSON identiques déclenchées simultanément au
chargement. Les modules du dashboard demandent tous le profil courant pour
appliquer le périmètre root ou propriétaire. Sans mutualisation, ces appels
concurrents produisent plusieurs requêtes `GET /api/auth/me`.

Le budget navigateur fixe désormais cette lecture initiale à une seule requête.

## Stratégie

`admin-http.js` conserve temporairement la promesse d’une lecture JSON lorsque :

- aucun objet d’options n’est fourni ;
- le chemin demandé est strictement identique ;
- la première requête est encore en cours.

L’entrée est supprimée dès la résolution ou le rejet. Il ne s’agit donc pas d’un
cache applicatif : une lecture séquentielle provoque toujours une nouvelle
requête et observe les données courantes.

Les appels avec options, notamment ceux portant un `AbortSignal`, ne sont jamais
mutualisés. Les écritures continuent à fournir un objet d’options avec leur
méthode HTTP et restent indépendantes.

## Budgets reproductibles

| Mesure | Budget |
|---|---:|
| `GET /api/auth/me` au premier dashboard root | 1 |
| Scripts `admin-*.js` chargés | 22 maximum |
| Chargements de `admin.css` | 1 |
| HTML non compressé | 70 000 octets |
| CSS non compressé | 35 000 octets |
| JavaScript admin non compressé | 190 000 octets |

Les trois budgets d’octets restent contrôlés par
`scripts/check-admin-v1.2-foundation.py`. Les budgets réseau sont vérifiés dans
Chromium par `tests/browser/performance.spec.cjs`.

Aucun seuil absolu en millisecondes n’est utilisé : il serait dépendant du CPU,
du stockage et de la charge de la machine de recette.

## Non-régression

`test-admin-http.cjs` vérifie que :

1. deux lectures simultanées identiques partagent une seule requête ;
2. une lecture ultérieure interroge de nouveau le serveur ;
3. deux appels munis d’options restent indépendants.

La suite d’authentification continue de vérifier que deux requêtes protégées
parallèles ne renouvellent un jeton invalide qu’une seule fois.
