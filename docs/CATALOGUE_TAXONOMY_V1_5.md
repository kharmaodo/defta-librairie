# Contrat de release v1.5.0 — taxonomie du catalogue

## Décision

La release introduit un catalogue multilingue arabe, français et anglais.

- Les **catégories** sont un référentiel métier commun et stable.
- Les **tags** restent des mots-clés propres à chaque librairie.
- Un livre représente une **édition commercialisée** et possède un éditeur principal.

Les catégories et éditeurs sont désactivables, jamais supprimés lorsqu’ils sont
déjà liés à un livre. Seul `SUPER_ADMIN_ROOT` peut les créer, modifier ou
désactiver ; `OWNER_LIBRARY` dispose d’un accès en lecture seule et les
sélectionne dans les formulaires de livres.

## Modèle cible

```text
categories(id, code, ar, fr, en, active)
publishers(id, code, ar, fr, en, active)
defta(..., publisher_id)
book_categories(book_id, category_id, is_primary)
library_tags(id, library_id, name, normalized_name, ...)
book_tags(book_id, tag_id)
```

Une édition a exactement un éditeur principal. Les coéditions restent hors
périmètre ; elles pourront être introduites plus tard avec une relation
spécialisée sans dégrader le modèle courant.

Un livre possède une catégorie principale et peut avoir des catégories
secondaires. La contrainte garantit une seule catégorie principale par livre.

## Seed de démarrage

La migration ajoute les catégories et éditeurs de référence avec
`INSERT OR IGNORE`. Elle est idempotente et ne dépend pas d’identifiants
numériques : les codes normalisés (`fiqh`, `hadith`, `tajwid`, etc.) sont
stables, les libellés étant traduits dans `ar`, `fr` et `en`.

Le seed est appliqué à chaque nouvelle base SQLite. Dans l’instance
multi-librairie, le référentiel est commun ; une activation par librairie ne
sera ajoutée que si le besoin métier est confirmé.

## Migration compatible

1. Créer les tables et semer les références sans supprimer les colonnes legacy.
2. Ajouter `publisher_id` nullable et les relations de catégories/tags.
3. Migrer les anciennes valeurs `categorie` et `tags` lorsque leur
   correspondance est non ambiguë ; tracer les lignes non résolues.
4. Servir temporairement les champs legacy et les nouvelles relations pendant
   la période de compatibilité.
5. Basculer l’interface, OpenAPI, FTS et exports vers le modèle relationnel.
6. Supprimer les champs legacy seulement dans une release ultérieure dédiée.

## Incréments

| Ordre | Incrément | Critère de validation |
|---|---|---|
| 1 | Fondation, seed et migrations | Réalisé — migration 029, seed idempotent de 29 catégories et 14 éditeurs AR/FR/EN |
| 2 | CRUD catégories et éditeurs | Réalisé — lecture authentifiée, mutations root-only, désactivation, audit et OpenAPI |
| 3 | Relations livre | Réalisé — migration 030, édition atomique, validation des références actives, OpenAPI et tests HTTP propriétaire |
| 4 | Tags relationnels | Réalisé — migration 031, associations atomiques, filtre `tagId`, migration CSV compatible, OpenAPI et tests HTTP |
| 5 | Interface publique/admin | Réalisé — sélecteurs accessibles, soumissions multipart, filtre `tagId`, traduction publique AR/FR/EN avec repli legacy et tests navigateur |
| 6 | Stabilisation | Go/FTS5, Playwright, export, OpenAPI, recette de migration et release |

## Critères de clôture

- Un nouveau client reçoit le seed sans doublon.
- Les libellés AR/FR/EN sont retournés sans dépendre de l’identifiant numérique.
- Une édition possède un éditeur principal actif.
- Une seule catégorie principale est autorisée par livre.
- Les catégories secondaires et tags enrichissent la recherche sans doublon.
- Aucune donnée catalogue existante n’est perdue durant la migration.
- Les contrôles Go, OpenAPI, accessibilité, Playwright et livraison sont verts.
