# Backlog de référence — Defta Librairie

État consolidé le 15 septembre 2026 sur `develop` après publication de
`v1.3.0` sur le commit `9f6bc6f`. Les douze priorités et leurs éléments
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
| 3 | Responsive et ergonomie | Navigation et panneaux affinés pour mobile, tablette et bureau | Réalisé |
| 4 | Système visuel | Composants harmonisés et thème sombre accessible, sans duplication des règles CSS | Réalisé |
| 5 | Performance frontend | Chargement mesuré, initialisation différée sûre et réduction du travail initial | Réalisé |
| 6 | Couverture navigateur | Tests Playwright desktop/mobile, thème, clavier et non-régression métier | Réalisé |
| 7 | Stabilisation et release | Recette complète, documentation, changelog, archives et sommes SHA-256 | Réalisé |

La mutualisation transitoire, les budgets de performance et la protection contre
les réponses obsolètes sont fusionnés par les PR #68 et #69. La couverture
navigateur consolidée est fusionnée par la PR #70 au commit `f7783cb`, avec vingt
parcours Chromium sans test ignoré. L’incrément final de stabilisation et release est réalisé. Le tag annoté
`v1.2.0` pointe sur `fdab7db` ; la release publique contient les archives Windows
AMD64, Raspberry Pi OS ARM64 et ARMv7, `BUILD-INFO.txt` et `SHA256SUMS`.

## Backlog v1.3.0 — lisibilité et suppressions sûres

Le contrat détaillé des versions `v1.3.0` et `v1.4.0` est défini dans
`docs/ROADMAP_V1_3_V1_4.md`. La confirmation renforcée concerne uniquement les
suppressions définitives et exige la saisie exacte du nom ou de la référence.

| Ordre | Incrément | Livrable vérifiable | État |
|---|---|---|---|
| 1 | Contrat et inventaire | Classement des suppressions définitives, suppressions logiques et transitions métier ; identifiants attendus documentés | Réalisé |
| 2 | Lisibilité des formulaires | Variable `--input-border-color`, états clair/sombre, focus, survol, désactivation et invalidité couverts | Réalisé |
| 3 | Confirmation renforcée | Dialogue commun accessible, égalité stricte, réinitialisation, focus et prévention de double soumission | Réalisé |
| 4 | Stabilisation et release | Tests JS, Go/FTS5 et Playwright complets ; sélecteurs documentés ; publication `v1.3.0` | Réalisé |


L’inventaire est réalisé et fusionné par la PR #74 au commit
`3ea36c0`. Il protège le livre, le tag et le brouillon de vente ; il documente
aussi la route non exposée de suppression d’un brouillon d’achat.
Désactivations, révocations et transitions métier restent hors du dialogue
renforcé.

Le correctif des champs est réalisé et fusionné par la PR #76 au commit
`90633e1`. Les variables partagées couvrent les thèmes clair et sombre ainsi
que les états survol, focus et désactivation.

Le dialogue renforcé est réalisé et fusionné par la PR #77 au commit
`aef2038`. Il est partagé par les suppressions du livre, du tag et du
brouillon de vente. L’égalité stricte, les espaces, la casse, la
réinitialisation, le focus, l’erreur HTTP et la double soumission sont couverts
par des tests unitaires et un parcours Playwright dédié.

La correction de recette est fusionnée par la PR #78 au commit `d0b44e2`.
La suite validée comprend les tests frontend, le parcours de confirmation répété
et vingt-deux parcours Chromium.

La version `v1.3.0` est publiée le 15 septembre 2026 sur le commit
`9f6bc6f`. La recette finale est validée avec vingt-deux parcours Chromium.
La release contient les archives Windows AMD64, Linux ARM64 et Linux ARMv7,
`BUILD-INFO.txt` et `SHA256SUMS`. Les sommes SHA-256 téléchargées sont validées.

La `v1.3.0` est clôturée. Le prochain incrément actif est le contrat, la menace
et la migration des couvertures de livre pour `v1.4.0`.


Les quatre incréments de `v1.3.0` sont réalisés. La recette finale comprend
vingt-deux parcours Chromium sans test ignoré. Le tag `v1.3.0` pointe sur le
commit `9f6bc6f` et la release publique contient les archives Windows AMD64,
Linux ARM64 et Linux ARMv7, ainsi que `BUILD-INFO.txt` et `SHA256SUMS`.

La version `v1.3.0` est clôturée. Le prochain incrément actif est le cadrage
technique des couvertures de livre de la version `v1.4.0`.

## Backlog v1.4.0 — couvertures de livre

Le bucket MinIO sera privé, la base conservera une clé d’objet et l’application
servira les images. Le traitement retenu est un recadrage centré 2:3 en
800 × 1200 pixels. La compatibilité Windows AMD64 et Linux AMD64 demeure obligatoire. Le worker
séparé est distribué comme image OCI Linux AMD64 ; sous Windows AMD64, il est
supervisé par l’exécutable natif de l’application.

| Ordre | Incrément | Livrable vérifiable | État |
|---|---|---|---|
| 1 | Fondation asynchrone | Contrat de menace, états, outbox SQLite, topologie API/JetStream/worker/MinIO et infrastructure Docker | Réalisé |
| 2 | Upload et stockage source | Client MinIO testable, bucket privé, JPEG/PNG réels, limites et source temporaire | Réalisé |
| 3 | Messagerie et worker | Publisher outbox, stream JetStream, consommateur durable, reprises et idempotence | Réalisé |
| 4 | Traitement et variantes | Master 2:3, JPEG, WebP, miniatures, plafond de pixels, image OCI Linux AMD64 et exécutable Windows AMD64 | Réalisé |
| 5 | Cycle de vie | Lecture, remplacement sans interruption, rétention, nettoyage compensé et absence d’orphelins | Réalisé |
| 6 | Interface et fallback | Upload admin, progression PENDING/FAILED, relance et image par défaut centralisée | Réalisé |
| 7 | Stabilisation et release | Sécurité, observabilité, Go/FTS5, Playwright, restauration et release `v1.4.0` | Réalisé |

La fondation asynchrone est validée et fusionnée par la PR #83 au commit
`892a6fa`. L’upload sécurisé et le stockage temporaire sont validés et fusionnés
par la PR #84 au commit `c25f054`. Ils couvrent la validation JPEG/PNG réelle,
les limites, le stockage MinIO privé, la transaction couverture/outbox/audit,
la compensation et la route multipart documentée. L’incrément 3 est réalisé et fusionné par la PR #85 au commit
`a74c15b` : baux récupérables, publisher avec acquittement, consumer durable,
reprises bornées et worker idempotent.

L’incrément 4 est réalisé et fusionné par la PR #86 au commit `5c8d80a`.
Il livre le recadrage 2:3, cinq objets JPEG/WebP, la compensation MinIO, le
pipeline d’intégration, l’image OCI Linux AMD64 et la supervision native Windows
AMD64. L’incrément 5 est réalisé et fusionné par la PR #87 : lecture privée,
remplacement, rétention et nettoyage compensé. L’incrément 6 est réalisé et fusionné par la PR #88 : formulaire d’upload,
statut asynchrone, relance et couverture par défaut commune à la liste et au
formulaire. Les tests frontend, Playwright et la recette complète sont validés
sur cet incrément. La recette finale est validée sur `a5025e3` (PR #90). Le tag annoté `v1.4.0`
pointe sur ce commit. La PR #91 limite les archives à Windows AMD64 et Linux
AMD64. Les workflows des exécutables et de l’image worker Linux AMD64 ont réussi
(runs `35606968932` et `35606963706`). La release publique contient les deux
archives, `BUILD-INFO.txt` et `SHA256SUMS` ; les sommes SHA-256 ont été
vérifiées. `v1.4.0` est clôturée.

Le traitement asynchrone par worker Go est retenu pour `v1.4.0`. NATS
JetStream assure la livraison persistante et une transactional outbox SQLite
empêche la perte d’un travail entre la transaction métier et la publication.
MinIO, NATS et l’initialisation du bucket sont dockerisés. Le master normalisé
est conservé ; la source brute suit une rétention avant suppression.

## Fonctionnalité post-v1.4 — modération NSFW des couvertures

La modération locale des couvertures est réalisée et sa recette finale est
validée. Le suivi fonctionnel et opérationnel est conservé dans
`docs/FEATURE_NSFW_COVER_MODERATION.md`.

| US | État | Preuve |
|---|---|---|
| Quarantaine, décision locale et création conditionnelle | Réalisé | `book_submissions`, worker et modèle isolé |
| Refus, revue root et reprise | Réalisé | États `REJECTED`, `REVIEW_REQUIRED`, `FAILED` |
| Remplacement sécurisé et affichage public | Réalisé | Modération avant stockage, route `READY` active |
| Purge des sources | Réalisé | File SQLite, baux, reprises et test d’idempotence |

## Questions métier à cadrer sans étendre silencieusement le périmètre

- Solde net après retour : conserver la distinction entre vente brute, retours,
  encaissements et remboursements ; décider de l’affichage commercial attendu.
- Consommation des avoirs sur une nouvelle vente : l’émission est livrée, mais
  imputation, utilisation partielle et solde d’avoir restent une extension à cadrer.
- Multidevise : hors périmètre de cette version, conformément à la validation
  de XOF comme devise unique. Toute extension devra définir le traitement de
  l’historique avant implémentation.

## Backlog v1.5.0 — taxonomie multilingue du catalogue

Le contrat détaillé est défini dans `docs/CATALOGUE_TAXONOMY_V1_5.md`.

| Ordre | Incrément | État |
|---|---|---|
| 1 | Fondation, seed et migrations | Réalisé — migration 029 et seed AR/FR/EN |
| 2 | CRUD catégories et éditeurs | Réalisé — lecture, mutations root-only, audit et OpenAPI |
| 3 | Relations livre-catégories et éditeur principal | Réalisé — migration 030, persistance atomique, validation des références actives, OpenAPI et tests HTTP propriétaire |
| 4 | Tags relationnels | Réalisé — migration 031, associations atomiques, filtre `tagId`, migration CSV compatible, OpenAPI et tests HTTP |
| 5 | Interface publique et administration | Réalisé — sélecteurs accessibles, soumissions multipart, filtre `tagId`, traduction publique AR/FR/EN avec repli legacy et tests navigateur |
| 6 | Stabilisation et release | À développer |

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
