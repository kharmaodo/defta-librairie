# Checklist de stabilisation — v1.5.0 taxonomie du catalogue

Cette checklist clôture les incréments 1 à 5 : référentiels multilingues,
relations de livres, tags relationnels et interfaces.

## Migration et compatibilité

- [ ] Démarrer une base SQLite existante : migrations 029 à 031 appliquées sans erreur.
- [ ] Vérifier l’idempotence des migrations et le seed AR/FR/EN.
- [ ] Vérifier la reprise des valeurs legacy `categorie`, `editeur` et `tags`.
- [ ] Confirmer que les champs legacy restent lisibles pendant la transition.

## Contrats et sécurité

- [ ] `jq empty static/openapi.json`
- [ ] `go test -tags fts5 ./cmd`
- [ ] Les mutations de catégories/éditeurs restent root-only.
- [ ] Les tags d’une autre librairie sont refusés.

## Recette applicative

- [ ] Créer/modifier un livre avec catégories, éditeur et tags.
- [ ] Soumettre un nouveau livre avec couverture et taxonomie.
- [ ] Vérifier le filtre admin `tagId`.
- [ ] Vérifier les libellés publics : arabe, puis français, anglais et legacy.

## Suites automatisées

```bash
go test -tags fts5 ./...
node --test scripts/test-admin-books.cjs
npx playwright test
./scripts/check-delivery.sh
```

## Sortie de release

- [x] Backlog et changelog v1.5.0 mis à jour.
- [x] Procédure de release suivie sur le commit candidat (`6bd5785`, tag `v1.5.0`).
- [x] Artefacts publiés et sommes SHA-256 consignés dans `docs/RELEASE_ARTIFACTS.md`.


## Publication

- Tag annoté : `v1.5.0` → `6bd578521b55c15321a8cc8ecdd2374b3388b474`.
- Archives Windows/Linux AMD64 : workflow `release-binaries.yml` #`36126652435` réussi.
- Image worker AMD64 : workflow `cover-worker-image.yml` #`36126741822` réussi.
- La vérification indépendante par `sha256sum -c SHA256SUMS` demeure reproductible et doit être archivée avec la preuve de déploiement.
