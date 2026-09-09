# Backlog de référence — Defta Librairie

État consolidé le 9 septembre 2026 sur `develop`, commit `08cf01f`
(fusion des alertes métier). Les douze priorités et leurs éléments
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
| 8 | Exports | CSV des stocks, ventes, achats, fournisseurs et audit | Partiel | Cinq exports et formulaire implémentés ; tests Go, recette navigateur et fusion à valider. Audit : événements propres au propriétaire, journal global pour root, selon les droits existants. |
| 9 | Paramétrage de librairie | Devise, coordonnées, logo, seuil par défaut, informations d’impression | Partiel | Nom et description existent dans l’administration ; créer le paramétrage complet et son utilisation dans les écrans/impressions. Le seuil actuel est par livre. |
| 10 | Documentation API | Contrat OpenAPI/Swagger et exemples complets `curl` | Partiel | Exemples et routes présents dans le README ; fournir un contrat OpenAPI complet, sa consultation et les exemples manquants. |
| 11 | Exploitation | Health checks enrichis, métriques, logs structurés et stratégie de restauration | Partiel | Sauvegardes SQLite contrôlées et arrêt gracieux existants. Ajouter les contrôles de santé, métriques, logs structurés et procédure de restauration testée. |
| 12 | Qualité frontend | Découpage du JavaScript, messages d’erreur globaux, accessibilité et tests navigateur | Partiel | Modules JavaScript métier déjà séparés. Harmoniser les erreurs, revoir l’accessibilité et intégrer les tests navigateur. Les recettes Go/HTTP ne couvrent pas le rendu visuel. |

## Incrément en validation : exports CSV (priorité 8)

Les alertes métier sont validées et fusionnées. Le nouvel incrément fournit
les cinq exports, des filtres propres au formulaire, UTF-8 avec BOM, séparateur
point-virgule et neutralisation des formules. Limite de 10 000 lignes, avec
refus explicite au-delà plutôt que troncature. Tests Go et recette navigateur
restent à valider avant fusion.

Les exports métier sont limités à la librairie autorisée. L’audit conserve
sa règle existante : événements du propriétaire connecté ; accès global root.
Il ne prétend pas reconstituer un audit par librairie à partir des ressources.

## Ordre de poursuite

Après validation de la priorité 8, poursuivre les lignes ouvertes 9, 10,
11 et 12 dans l’ordre du tableau. Les tests et la
qualité des nouveaux écrans s’appliquent à chaque incrément, sans attendre la ligne 12.

Pour les alertes de « commandes en attente », préciser la règle opérationnelle
avant implémentation : le modèle actuel des achats utilise DRAFT, RECEIVED et
CANCELLED ; il ne faut pas inventer un état SENT ou ORDERED inexistant.

## Questions métier à cadrer sans étendre silencieusement le périmètre

- Solde net après retour : conserver la distinction entre vente brute, retours,
  encaissements et remboursements ; décider de l’affichage commercial attendu.
- Consommation des avoirs sur une nouvelle vente : l’émission est livrée, mais
  imputation, utilisation partielle et solde d’avoir restent une extension à cadrer.
- Devise : décider si elle est fixe par librairie et de l’effet d’un changement sur
  les données historiques avant d’ajouter le paramétrage de la priorité 9.

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
| Exploitation existante | `scripts/backup-db.sh`, `cmd/main.go` |

## Mise à jour après chaque validation

Actualiser la ligne concernée après validation et fusion, citer les nouveaux
éléments de preuve et garder les travaux non terminés explicites. Les paragraphes
historiques du README décrivent les mécanismes ; ce backlog fait référence pour
l’état d’avancement et l’ordre des priorités.
