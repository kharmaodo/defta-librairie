# Intégrité métier concurrente — v1.6

Ce document couvre l’US-5 du backlog OWASP. Il vérifie les invariants métier
après des requêtes concurrentes sur des transitions qui modifient le stock ou
les montants.

## Scénario initial : confirmation de vente

Le test `TestCommercialHTTPConfirmsSaleOnlyOnceUnderConcurrency` crée une
vente brouillon de deux exemplaires sur un stock initial de dix, puis envoie
40 confirmations avec la même version de ressource.

Résultat obligatoire :

| Élément | Attendu |
|---|---:|
| Confirmations HTTP réussies | 1 |
| Conflits de version HTTP | 39 |
| Stock final | 8 |
| Décrément de stock | une seule fois |

Le test exécute les requêtes avec le même propriétaire et la même librairie :
il couvre l’intégrité de la transition, non le cloisonnement déjà traité par
l’US-3.

## Exécution

```sh
go test -race -tags fts5 ./cmd \
  -run TestCommercialHTTPConfirmsSaleOnlyOnceUnderConcurrency \
  -count=1
```

## Matrice à compléter

| Flux | Invariant concurrent à prouver |
|---|---|
| Paiement | un même paiement ne peut pas dépasser le solde ni être créé deux fois |
| Retour client | une ligne retournée ne peut pas restaurer le stock deux fois |
| Remboursement | le total remboursé ne dépasse jamais les encaissements actifs |
| Réception d’achat | une réception ne valorise et n’ajoute le stock qu’une seule fois |
| Retour fournisseur | une expédition ne retire le stock qu’une seule fois |
| Couverture | une promotion de variante ne remplace pas deux fois l’état actif |

Chaque nouveau scénario doit vérifier le statut HTTP, les invariants SQLite et
les événements d’audit utiles, avec une base de test jetable.

## Limites

Cette première preuve cible l’optimistic locking d’une vente dans une instance.
Les scénarios restants sont ajoutés séparément pour garder les échecs
diagnostiquables et les commits atomiques.
