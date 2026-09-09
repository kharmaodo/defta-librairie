# Backlog de référence — Defta Librairie

État consolidé le 9 septembre 2026 sur `develop`, commit `b76734e`
(fusion de la recette HTTP commerciale). Les douze priorités et leurs éléments
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
| 2 | Gestion des clients | CRUD, recherche, historique des achats, rattachement aux ventes | Partiel | Historique paginé des achats depuis une fiche client. CRUD, recherche, désactivation/réactivation et rattachement aux ventes existent. |
| 3 | Paiements et caisse | Espèces, mobile money, carte, paiements partiels, reste à payer | Réalisé | Les modes de paiement sont enregistrés dans l’application ; aucune intégration à un prestataire de paiement n’est présumée. Le solde existant porte sur la vente brute. |
| 4 | Retours et remboursements | Retour client, restauration du stock, avoir, remboursement et audit | Réalisé | Le périmètre comprend l’émission d’avoirs, les remboursements partiels, leur audit et le plafond remboursable. La consommation d’un avoir sur une nouvelle vente n’est pas incluse implicitement. |
| 5 | Retours fournisseurs | Sortie de stock liée à un achat réceptionné et justification | Réalisé | Brouillons, expédition atomique, annulation, motif, coûts figés, écarts et affichage sont livrés. |
| 6 | Statistiques commerciales | Chiffre d’affaires, marge, ventes et achats par période | Réalisé | Ventes brutes/nettes, annulations, retours, achats nets, marge nullable et écarts fournisseurs disponibles. Les coûts inconnus sont signalés. |
| 7 | Alertes métier | Stock faible, rupture, commandes en attente, fournisseurs désactivés | Partiel | Stock faible et rupture existent dans les filtres et le tableau de stocks. Ajouter les alertes de commandes en attente et de fournisseurs désactivés, puis leur présentation commune. |
| 8 | Exports | CSV des stocks, ventes, achats, fournisseurs et audit | À développer | Les cinq exports avec filtres, isolation par librairie, encodage adapté aux contenus arabes et traitement sûr des cellules. |
| 9 | Paramétrage de librairie | Devise, coordonnées, logo, seuil par défaut, informations d’impression | Partiel | Nom et description existent dans l’administration ; créer le paramétrage complet et son utilisation dans les écrans/impressions. Le seuil actuel est par livre. |
| 10 | Documentation API | Contrat OpenAPI/Swagger et exemples complets `curl` | Partiel | Exemples et routes présents dans le README ; fournir un contrat OpenAPI complet, sa consultation et les exemples manquants. |
| 11 | Exploitation | Health checks enrichis, métriques, logs structurés et stratégie de restauration | Partiel | Sauvegardes SQLite contrôlées et arrêt gracieux existants. Ajouter les contrôles de santé, métriques, logs structurés et procédure de restauration testée. |
| 12 | Qualité frontend | Découpage du JavaScript, messages d’erreur globaux, accessibilité et tests navigateur | Partiel | Modules JavaScript métier déjà séparés. Harmoniser les erreurs, revoir l’accessibilité et intégrer les tests navigateur. Les recettes Go/HTTP ne couvrent pas le rendu visuel. |

## Prochain incrément : finir l’historique client (priorité 2)

Objectif : depuis la fiche d’un client, consulter ses ventes sans rechercher
manuellement chaque référence. « Achats du client » désigne ici les ventes de la
librairie à ce client, et non les achats auprès des fournisseurs.

Critères de finalisation proposés pour cet incrément :

1. Liste paginée des ventes rattachées par `customer_id`, avec référence, date,
   statut et montant ; filtres de période et de statut.
2. Les ventes comptoir sans client ne sont pas attribuées par rapprochement du nom.
3. L’historique reste disponible après désactivation du client ; les ventes
   annulées restent identifiables et les brouillons ne sont pas présentés comme
   des achats finalisés.
4. Accès propriétaire limité à sa librairie ; root autorisé dans le périmètre
   demandé ; tests d’isolation, pagination et historique vide.
5. Action depuis l’écran Clients, documentation et tests de non-régression.

Cette section fixe l’objectif de travail ; elle ne déclare pas ces ajouts réalisés.

## Ordre de poursuite

Après la consolidation documentaire, compléter la priorité 2, puis poursuivre
les lignes ouvertes 7, 8, 9, 10, 11 et 12 dans l’ordre du tableau. Les tests et la
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
| Clients | `internal/handlers/customers.go`, `internal/repositories/customer_repository.go`, `static/js/admin-customers.js`, `internal/models/sale.go` |
| Paiements et retours | `internal/repositories/payment_repository.go`, `internal/repositories/return_settlement_repository.go`, migrations 015, 016, 021 et 022 |
| Retours fournisseurs | `internal/repositories/supplier_return_shipping.go`, migration 020, `static/js/admin-supplier-returns.js` |
| Statistiques | `internal/repositories/commercial_statistics_repository.go`, `static/js/admin-statistics.js` |
| Recettes | `internal/services/commercial_lifecycle_acceptance_test.go`, `cmd/commercial_http_test.go` |
| Stock et alertes existantes | `internal/repositories/inventory_repository.go`, `templates/admin.html` |
| Exploitation existante | `scripts/backup-db.sh`, `cmd/main.go` |

## Mise à jour après chaque validation

Actualiser la ligne concernée après validation et fusion, citer les nouveaux
éléments de preuve et garder les travaux non terminés explicites. Les paragraphes
historiques du README décrivent les mécanismes ; ce backlog fait référence pour
l’état d’avancement et l’ordre des priorités.
