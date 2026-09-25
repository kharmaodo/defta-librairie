# Baseline sécurité OWASP — v1.6.0

Ce document réalise l’US-1 du backlog `docs/BACKLOG_OWASP_V1_6.md`.
Il décrit le périmètre de test avant toute génération de charge. Les contrôles
seront prouvés par des tests déterministes et des scénarios de charge exécutés
uniquement sur un environnement isolé.

## Actifs et frontières

| Actif | Données ou effet à protéger | Frontière |
|---|---|---|
| API d’administration | Livres, stock, ventes, paiements, retours, clients, fournisseurs et audit | JWT, rôle et librairie |
| API root | Propriétaires, métriques et référentiels globaux | rôle `SUPER_ADMIN_ROOT` |
| Catalogue public | Recherche, couverture active et métadonnées publiées | exposition volontaire minimale |
| Sessions | Jetons, cookies, révocation et changement de mot de passe | authentification et cycle de session |
| Couvertures | Source privée, quarantaine, variantes actives et modération | MinIO privé, worker et décisions de modération |
| Infrastructure locale | SQLite, NATS, MinIO et service de modération | réseau Docker et configuration |

## Rôles de référence

| Acteur | Portée attendue |
|---|---|
| Anonyme | Santé, catalogue et uniquement les couvertures publiques `READY` actives |
| Propriétaire | Ressources de sa librairie, paramètres et opérations métier autorisées |
| Super-admin root | Portée globale, propriétaires, métriques et mutations des référentiels globaux |
| Service interne | Worker de couverture, NATS, MinIO et modération, sans exposition HTTP publique directe |

## Matrice routes × contrôles

| Famille de routes | Exemples | Contrôles à prouver | Risques OWASP |
|---|---|---|---|
| Authentification et sessions | `/api/auth/login`, `/refresh`, `/logout`, `/sessions` | réponses non énumérantes, throttling, expiration, révocation et cookie sûr | API2, API4 |
| Administration root | `/api/admin/owners`, `/api/admin/metrics` | refus strict propriétaire/anonyme, journalisation et absence de fuite | API1, API3, API5 |
| Référentiels globaux | `/api/manage/categories`, `/publishers` | lecture autorisée aux rôles prévus ; mutation root-only | API3, API5 |
| Ressources de librairie | livres, clients, fournisseurs, caisses, tags | cloisonnement A/B sur lecture et mutation ; identifiants manipulés | API1, API3 |
| Stock et approvisionnement | inventaire, achats, réceptions, retours fournisseur | transition valide, idempotence et cohérence de stock sous concurrence | API6 |
| Vente et règlement | ventes, paiements, retours client, remboursements | absence de double débit/crédit, plafond et ordre des transitions | API6 |
| Catalogue et médias | `/api/books`, couvertures publiques/admin | exposition publique minimale, source privée, statut `READY` obligatoire | API1, API3, API4 |
| Soumissions/modération | `/api/manage/book-submissions` | séparation des rôles, reprise sûre, pas de promotion d’un média refusé | API5, API6 |
| Exports, recherche et statistiques | exports, FTS, statistiques, audit | limites de pagination/durée/volume, cloisonnement et absence de surcharge | API1, API4 |
| Santé et configuration | health, readiness, erreurs HTTP | aucune donnée sensible, méthodes limitées, logs corrélés | API8, API9 |

## Invariants à tester

1. Un propriétaire de la librairie A ne lit ni ne modifie un objet de la
   librairie B, avec l’identifiant réel, absent ou falsifié.
2. Les mutations root-only répondent par un refus cohérent pour un propriétaire
   et un anonyme, sans effet de bord.
3. Une transition métier ne peut pas être rejouée ni contournée par parallélisme
   ou ordre de requêtes invalide.
4. Une couverture refusée ou non finalisée n’est jamais rendue publique et ne
   remplace pas la couverture active.
5. Les endpoints coûteux respectent les limites de taille, durée, pagination et
   concurrence définies par les incréments suivants.
6. Les échecs sont corrélables par `request_id`, sans secret, jeton ni donnée
   personnelle inutile dans les logs.

## Jeux de données de sécurité

Chaque campagne utilisera :

- un super-admin root ;
- deux propriétaires et deux librairies distinctes ;
- livres, tags, clients, fournisseurs, achats et ventes propres à chaque
  librairie ;
- une couverture `READY`, une couverture en attente et une soumission rejetée ;
- identifiants inconnus et identifiants valides appartenant à l’autre librairie.

Les données sont créées dans une base et des buckets jetables, recréés pour
chaque campagne.

## Mesures initiales à fixer

Les US suivantes fixeront, sur l’environnement isolé, les seuils suivants :

| Mesure | À relever |
|---|---|
| Latence | p50, p95 et p99 par scénario |
| Fiabilité | taux de succès, 4xx attendus et 5xx inattendus |
| Débit | requêtes/s soutenues et pics contrôlés |
| Ressources | CPU, mémoire, connexions, fichiers ouverts et taille SQLite |
| Intégrité | doublons, stock, montants, audit et versions de ressource |
| Défense | nombre de refus de limite, refus d’autorisation et verrouillages |

## Preuves attendues

- tests HTTP Go de la matrice d’autorisation ;
- rapports de charge versionnés, avec paramètres et résultats ;
- requêtes SQLite d’invariants après scénario ;
- extraits de métriques et logs structurés anonymisés ;
- contrôle de delivery qui refuse une régression de sécurité définie.

## Hors périmètre de l’US-1

Cette baseline ne lance aucun test volumétrique, ne modifie pas les seuils
d’application et ne vise pas la production. Elle sert de contrat aux US-2 à
US-6.

## Références OWASP

- [OWASP API Security Top 10 — 2023](https://owasp.org/API-Security/editions/2023/en/0x11-t10/)
- [API1: Broken Object Level Authorization](https://owasp.org/API-Security/editions/2023/en/0xa1-broken-object-level-authorization/)
- [API4: Unrestricted Resource Consumption](https://owasp.org/API-Security/editions/2023/en/0xa4-unrestricted-resource-consumption/)
- [OWASP Web Security Testing Guide](https://owasp.org/www-project-web-security-testing-guide/latest/)
