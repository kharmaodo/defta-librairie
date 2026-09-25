# Limites de ressources — v1.6

Ce document suit l’US-4 du backlog OWASP. Il consolide les limites applicatives
existantes et ajoute la borne de recherche catalogue afin que les essais de
charge mesurent des protections explicites.

## Limites applicatives

| Surface | Limite | Réponse attendue | Preuve |
|---|---:|---|---|
| Recherche publique `/api/books?q=` | 256 caractères Unicode | `400 invalid_query` | test HTTP handler |
| Recherche administrée `/api/manage/books?q=` | 256 caractères Unicode | `400 invalid_query` | test HTTP handler |
| Pagination API | `limit` maximum 100 | limite normalisée à 100 | tests handlers |
| Corps JSON métier | 256 Kio | requête invalide | `decodeOwnerJSON` / `decodeAuthJSON` |
| Couverture | taille configurée, défaut 5 Mio | rejet avant stockage | validateur de couverture |
| Couverture | 24 millions de pixels par défaut | rejet avant traitement | validateur de couverture |
| Export CSV | plafond métier de lignes | `422 export_too_large` | service/export HTTP |

## Règles de recette

- Vérifier la limite de recherche avec des caractères ASCII et multioctets.
- Vérifier que le refus arrive avant un accès à SQLite ou au service de livre.
- Garder les limites de pagination compatibles : une valeur trop grande est
  plafonnée, sans exposer un export ou une liste non bornée.
- Exécuter les uploads lourds uniquement dans un bucket de test.
- Mesurer les exports au plafond et juste au-delà sans enregistrer de données
  réelles dans les rapports.

## Commandes ciblées

```sh
go test -tags fts5 ./internal/handlers -run 'TestNormalizeAPISearch|TestCatalogueHandlersRejectOversizedSearchBeforeDatabaseAccess' -count=1
go test -tags fts5 ./internal/covers ./internal/services ./internal/handlers
jq empty static/openapi.json
```

## Décisions restantes

Les seuils de débit, de simultanéité, de délai et de mémoire doivent être
établis sur l’environnement isolé dans les US suivantes. Aucun test de charge
ne doit contourner une limite existante ni être dirigé contre la production.
