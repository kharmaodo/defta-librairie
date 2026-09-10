# Backlog de référence — Defta Librairie

État consolidé le 9 septembre 2026 sur `develop`, commit `d24d82f`
(fusion du paramétrage de librairie, PR #26). Les douze priorités et leurs éléments
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
| 10 | Documentation API | Contrat OpenAPI/Swagger et exemples complets `curl` | Partiel | Contrat OpenAPI 3.0.3 des 93 opérations API, consultation locale et exemples curl implémentés. Contrôles structurels validés ; tests Go et recette navigateur à valider avant fusion. |
| 11 | Exploitation | Health checks enrichis, métriques, logs structurés et stratégie de restauration | Partiel | Sauvegardes SQLite contrôlées et arrêt gracieux existants. Ajouter les contrôles de santé, métriques, logs structurés et procédure de restauration testée. |
| 12 | Qualité frontend | Découpage du JavaScript, messages d’erreur globaux, accessibilité et tests navigateur | Partiel | Modules JavaScript métier déjà séparés. Harmoniser les erreurs, revoir l’accessibilité et intégrer les tests navigateur. Les recettes Go/HTTP ne couvrent pas le rendu visuel. |

## Incrément en validation : documentation API (priorité 10)

Le paramétrage est validé et fusionné au commit `d24d82f`. Le propriétaire du
projet valide XOF comme devise unique du périmètre actuel. La priorité 9 est
donc réalisée : aucun changement de devise ni conversion des montants
historiques n’est prévu dans cette version. Une éventuelle prise en charge de
plusieurs devises nécessitera un périmètre distinct et une nouvelle décision.

Le contrat `static/openapi.json` documente les 93 opérations enregistrées dans
`cmd/main.go`. La page locale, les exemples curl et les contrôles de dérive
routes/schémas sont ajoutés. Les contrôles Python et JavaScript passent ; les
tests Go et la recette navigateur restent à valider avant fusion.

## Ordre de poursuite

Poursuivre les priorités 10, 11 et 12 dans cet ordre. Les tests et la
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
| Exploitation existante | `scripts/backup-db.sh`, `cmd/main.go` |

## Mise à jour après chaque validation

Actualiser la ligne concernée après validation et fusion, citer les nouveaux
éléments de preuve et garder les travaux non terminés explicites. Les paragraphes
historiques du README décrivent les mécanismes ; ce backlog fait référence pour
l’état d’avancement et l’ordre des priorités.


À chaque fusion vérifiée dans `origin/develop`, mettre à jour le commit de
référence, la ligne concernée et le prochain incrément dans le patch suivant.
Une fusion partielle conserve le statut Partiel et détaille les éléments restants.
