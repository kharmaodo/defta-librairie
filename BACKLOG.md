# Backlog de référence — Defta Librairie

État consolidé le 11 septembre 2026 sur `develop`, commit `9f568d9`
(fusion du module des livres, PR #37). Les douze priorités et leurs éléments
principaux reprennent le périmètre rappelé par le propriétaire du projet.
Les validations et fusions annoncées sont prises en compte ; cette consolidation
n’exécute pas une nouvelle recette ni un audit de production.

- **Réalisé** : les éléments du périmètre indiqué sont implémentés et leurs incréments validés/fusionnés.
- **Partiel** : des éléments existent, mais la ligne comporte encore un travail identifié.
- **À développer** : aucune implémentation de la fonctionnalité demandée n’a été repérée.

## État des douze priorités

| Priorité | Fonctionnalité | Éléments principaux du périmètre | État | Reste à finaliser |
|---|---|---|---|---|
| 1 | Finalisation de l’approvisionnement | Tests finaux, contrôle des filtres, merge dans `develop` | Réalisé | Aucun incrément d’approvisionnement restant identifié dans ce périmètre ; maintenir la non-régression. |
| 2 | Gestion des clients | CRUD, recherche, historique des achats, rattachement aux ventes | Réalisé | Historique paginé et filtré validé et fusionné, y compris après désactivation du client. |
| 3 | Paiements et caisse | Espèces, mobile money, carte, paiements partiels, reste à payer | Réalisé | Les modes de paiement sont enregistrés dans l’application ; aucune intégration à un prestataire de paiement n’est présumée. Le solde existant porte sur la vente brute. |
| 4 | Retours et remboursements | Retour client, restauration du stock, avoir, remboursement et audit | Réalisé | Le périmètre comprend l’émission d’avoirs, les remboursements partiels, leur audit et le plafond remboursable. La consommation d’un avoir sur une nouvelle vente n’est pas incluse implicitement. |
| 5 | Retours fournisseurs | Sortie de stock liée à un achat réceptionné et justification | Réalisé | Brouillons, expédition atomique, annulation, motif, coûts figés, écarts et affichage sont livrés. |
| 6 | Statistiques commerciales | Chiffre d’affaires, marge, ventes et achats par période | Réalisé | Ventes brutes/nettes, annulations, retours, achats nets, marge nullable et écarts fournisseurs disponibles. Les coûts inconnus sont signalés. |
| 7 | Alertes métier | Stock faible, rupture, commandes en attente, fournisseurs désactivés | Réalisé | Tableau commun validé et fusionné. Les achats DRAFT sont explicitement désignés comme achats en brouillon. |
| 8 | Exports | CSV des stocks, ventes, achats, fournisseurs et audit | Réalisé | Cinq exports validés et fusionnés. Audit limité aux événements propres du propriétaire, global pour root. |
| 9 | Paramétrage de librairie | Devise, coordonnées, logo, seuil par défaut, informations d’impression | Réalisé | Coordonnées, logo, seuil des nouveaux livres et impressions validés et fusionnés (PR #26). XOF validé comme devise unique de cette version ; aucun changement ni conversion des montants historiques. |
| 10 | Documentation API | Contrat OpenAPI/Swagger et exemples complets `curl` | Réalisé | Contrat, consultation locale, exemples et contrôles validés et fusionnés (PR #27). Le contrat est actualisé avec chaque nouvelle route. |
| 11 | Exploitation | Health checks enrichis, métriques, logs structurés et stratégie de restauration | Réalisé | Sondes (PR #29), métriques/logs (PR #30) et restauration testée (PR #31) validés et fusionnés. Les décisions de déploiement sont conservées ci-dessous. |
| 12 | Qualité frontend | Découpage du JavaScript, messages d’erreur globaux, accessibilité et tests navigateur | Partiel | Accessibilité des 20 dialogues validée et fusionnée (PR #32). Client HTTP commun validé et intégré pour cinq écrans. Erreurs des cinq modules métier validées et fusionnées (PR #33). Journal d’audit extrait et fusionné (PR #34). Sessions extraites et fusionnées (PR #35). Propriétaires extraits et fusionnés (PR #36). Livres extraits et fusionnés (PR #37). Extraction du stock en validation ; suite du découpage, migration HTTP du module principal et tests navigateur automatisés restent à finaliser. |

## Incrément en validation : extraction du stock (priorité 12)

Les livres sont validés et fusionnés (PR #37). Cet incrément extrait
la liste, les filtres, la pagination, les mouvements, le seuil et l’historique
du stock dans `admin-inventory.js`. Le tableau de bord fournit le rôle courant,
le client HTTP existant et le rechargement de l’audit. Les rechargements de
stock déclenchés par les livres et les ventes sont conservés.
Les événements sont initialisés après le contrôle du mot de passe obligatoire.
Recette navigateur et fusion de cet incrément restent à valider.

Restent les ventes et tags dans `admin-auth.js`, la migration HTTP compatible
avec l’authentification et les tests navigateur automatisés.
La priorité 12 reste partielle.

La priorité 11 est réalisée dans le périmètre livré ; objectifs de reprise,
sauvegardes hors machine et rétention des logs restent des décisions de déploiement.

## Ordre de poursuite

Finaliser les incréments de la priorité 12. Les tests et la
qualité des nouveaux écrans s’appliquent à chaque incrément, sans attendre la ligne 12.

La règle livrée pour les alertes reste explicite : les achats DRAFT sont des
achats en brouillon ; aucun état SENT ou ORDERED n’est présumé.

## Questions métier à cadrer sans étendre silencieusement le périmètre

- Solde net après retour : conserver la distinction entre vente brute, retours,
  encaissements et remboursements ; décider de l’affichage commercial attendu.
- Consommation des avoirs sur une nouvelle vente : l’émission est livrée, mais
  imputation, utilisation partielle et solde d’avoir restent une extension à cadrer.
- Multidevise : hors périmètre de cette version, conformément à la validation
  de XOF comme devise unique. Toute extension devra définir le traitement de
  l’historique avant implémentation.

## Preuves dans le dépôt

| Domaine | Principaux fichiers de référence |
|---|---|
| Approvisionnement | `internal/repositories/purchase_repository.go`, `internal/services/purchase_service_test.go` |
| Clients | `internal/handlers/customers.go`, `internal/repositories/customer_repository.go`, `static/js/admin-customers.js`, `static/js/admin-customer-history.js`, `internal/services/customer_history_test.go`, `internal/handlers/customer_history_test.go` |
| Paiements et retours | `internal/repositories/payment_repository.go`, `internal/repositories/return_settlement_repository.go`, migrations 015, 016, 021 et 022 |
| Retours fournisseurs | `internal/repositories/supplier_return_shipping.go`, migration 020, `static/js/admin-supplier-returns.js` |
| Statistiques | `internal/repositories/commercial_statistics_repository.go`, `static/js/admin-statistics.js` |
| Recettes | `internal/services/commercial_lifecycle_acceptance_test.go`, `cmd/commercial_http_test.go` |
| Stock et alertes existantes | `internal/repositories/business_alert_repository.go`, `internal/services/business_alert_service_test.go`, `internal/handlers/business_alerts_test.go`, `static/js/admin-business-alerts.js` |
| Documentation API | `static/openapi.json`, `static/api-docs.html`, `docs/API_EXAMPLES.md`, `cmd/openapi_contract_test.go`, `scripts/check-openapi.py` |
| Paramétrage livré | `internal/migrations/sql/023_create_library_settings.sql`, `internal/services/library_settings_service_test.go`, `static/js/admin-library-settings.js` |
| Accessibilité frontend | `templates/admin.html`, `static/js/admin-supplier-returns.js`, `scripts/check-admin-accessibility.py`, `docs/FRONTEND_ACCESSIBILITY.md` |
| Exploitation existante | `scripts/backup-db.sh`, `scripts/restore-db.py`, `scripts/test-restore-db.py`, `docs/SQLITE_RESTORE.md`, `internal/middleware/observability.go`, `cmd/main.go` |

## Mise à jour après chaque validation

Actualiser la ligne concernée après validation et fusion, citer les nouveaux
éléments de preuve et garder les travaux non terminés explicites. Les paragraphes
historiques du README décrivent les mécanismes ; ce backlog fait référence pour
l’état d’avancement et l’ordre des priorités.


À chaque fusion vérifiée dans `origin/develop`, mettre à jour le commit de
référence, la ligne concernée et le prochain incrément dans le patch suivant.
Une fusion partielle conserve le statut Partiel et détaille les éléments restants.
