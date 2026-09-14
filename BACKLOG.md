# Backlog de référence — Defta Librairie

État consolidé le 14 septembre 2026 sur `develop`, commit `bd7ddc3`
(synthèse métier de `v1.2.0` fusionnée par la PR #65). Les douze priorités et leurs éléments
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
| 12 | Qualité frontend | Découpage du JavaScript, messages d’erreur globaux, accessibilité et tests navigateur | Réalisé | Accessibilité structurelle des 20 dialogues, client HTTP, erreurs métier et modules frontend livrés. Les parcours navigateur couvrent authentification, cycles métier, exports, impressions et garanties clavier principales. Un audit manuel avec lecteur d’écran reste une vérification de production recommandée. |

## Périmètre fonctionnel consolidé

L’accessibilité clavier navigateur est validée et fusionnée au commit `f7c9cae`.
Les douze priorités sont réalisées dans le périmètre convenu. La recette finale,
la matrice de couverture et la checklist de déploiement sont centralisées dans
`docs/DELIVERY.md` et automatisées par `scripts/check-delivery.sh`.

La course d’affichage détectée par le contrôle de release dans les scénarios
créant une vente est corrigée et fusionnée (PR #50). Le contrôle final est vert,
la préparation est fusionnée (PR #51) et le tag `v1.0.0` est publié sur le commit
`773d0c7`.

La priorité 11 est réalisée dans le périmètre livré ; objectifs de reprise,
sauvegardes hors machine et rétention des logs restent des décisions de déploiement.

## Après le delivery

Exécuter le contrôle final sur le commit candidat, décider les paramètres
d’exploitation, taguer la release puis ouvrir un nouveau backlog pour toute
extension. Les tests et la qualité frontend restent obligatoires à chaque incrément.

La règle livrée pour les alertes reste explicite : les achats DRAFT sont des
achats en brouillon ; aucun état SENT ou ORDERED n’est présumé.

## Backlog v1.1.0 — refonte du dashboard d’administration

La prochaine version mineure modernise uniquement l’interface d’administration.
Elle ne modifie ni les règles métier, ni les routes HTTP, ni les formats de données.
Le contrat détaillé de la refonte est défini dans `docs/ADMIN_UI_V1_1.md`.

| Ordre | Incrément | Livrable vérifiable | État |
|---|---|---|---|
| 1 | Fondation et contrat de non-régression | Audit du DOM, architecture de navigation, sélecteurs protégés et critères d’acceptation | Réalisé |
| 2 | Structure du dashboard | Header modernisé, navigation thématique, sous-menus accessibles et ancres de sections | Réalisé |
| 3 | Système visuel | `admin.css` non minifié, commenté, responsive et organisé par composants | Réalisé |
| 4 | Navigation responsive | Comportement desktop/mobile, focus, fermeture et état actif | Réalisé |
| 5 | Couverture navigateur | Tests Playwright des menus, du clavier, des ancres et de l’accessibilité sans réduire les parcours existants | Réalisé |
| 6 | Stabilisation et release | Suite Go/FTS5, contrôles statiques et suite Playwright complète sans test ignoré | Réalisé |

Après chaque fusion vérifiée dans `origin/develop`, mettre à jour ici le commit
de référence, l’état de l’incrément fusionné et le prochain incrément actif.

Les six incréments de `v1.1.0` sont réalisés. Le contrôle final comprend neuf
étapes et treize parcours Chromium sans test ignoré. Le tag annoté `v1.1.0` est
publié sur le commit `eaf5d8a`.

La publication des exécutables Windows AMD64, Raspberry Pi OS ARM64 et Raspberry
Pi OS ARMv7 est réalisée. Les archives incluent les ressources d’exécution et
sont accompagnées de sommes SHA-256. La release `v1.1.0` est clôturée.

## Backlog v1.2.0 — expérience du dashboard

La prochaine version améliore progressivement l’expérience d’utilisation du
dashboard livré en `v1.1.0`. Elle conserve les contrats HTTP, les règles métier,
les identifiants DOM protégés et le contenu du footer. Le périmètre détaillé et
les critères de décision sont définis dans `docs/ROADMAP_V1_2.md`.

| Ordre | Incrément | Livrable vérifiable | État |
|---|---|---|---|
| 1 | Cadrage et mesures de référence | Inventaire UX, budget de performance, breakpoints et contrat de non-régression | Réalisé |
| 2 | Synthèse du tableau de bord | Indicateurs métier prioritaires, états vide/chargement/erreur et accès aux rubriques | Réalisé |
| 3 | Responsive et ergonomie | Navigation et panneaux affinés pour mobile, tablette et bureau | À valider |
| 4 | Système visuel | Composants harmonisés et thème sombre accessible, sans duplication des règles CSS | À réaliser |
| 5 | Performance frontend | Chargement mesuré, initialisation différée sûre et réduction du travail initial | À réaliser |
| 6 | Couverture navigateur | Tests Playwright desktop/mobile, thème, clavier et non-régression métier | À réaliser |
| 7 | Stabilisation et release | Recette complète, documentation, changelog, archives et sommes SHA-256 | À réaliser |

La synthèse métier est fusionnée. Le responsive est renforcé aux largeurs 390,
768, 1 024 et 1 440 px ; formulaires, actions, tableaux, navigation et cartes
conservent un comportement utilisable sans débordement global. Après validation
et fusion, le prochain incrément actif sera le système visuel et le thème sombre.

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
| Accessibilité frontend | `templates/admin.html`, `static/js/admin-supplier-returns.js`, `scripts/check-admin-accessibility.py`, `docs/FRONTEND_ACCESSIBILITY.md`, `tests/browser/accessibility.spec.cjs` |
| Exploitation existante | `scripts/backup-db.sh`, `scripts/restore-db.py`, `scripts/test-restore-db.py`, `docs/SQLITE_RESTORE.md`, `internal/middleware/observability.go`, `cmd/main.go` |
| Delivery final | `docs/DELIVERY.md`, `scripts/check-delivery.sh`, `docs/RELEASE.md`, `CHANGELOG.md` |

## Mise à jour après chaque validation

Actualiser la ligne concernée après validation et fusion, citer les nouveaux
éléments de preuve et garder les travaux non terminés explicites. Les paragraphes
historiques du README décrivent les mécanismes ; ce backlog fait référence pour
l’état d’avancement et l’ordre des priorités.


À chaque fusion vérifiée dans `origin/develop`, mettre à jour le commit de
référence, la ligne concernée et le prochain incrément dans le patch suivant.
Une fusion partielle conserve le statut Partiel et détaille les éléments restants.
