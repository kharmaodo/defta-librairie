# Publication de la version 1.1.0

Cette procédure crée une release traçable à partir de `develop`, après réussite
du contrôle final sur le commit exact à taguer.

## 1. Valider le candidat

```sh
git switch develop
git pull --ff-only
git status --short
./scripts/check-delivery.sh
git rev-parse --verify HEAD
```

Le statut doit être vide. Conserver le SHA affiché.

## 2. Construire l’artefact

```sh
mkdir -p dist
CGO_ENABLED=1 go build -trimpath -tags fts5 \
  -o dist/defta-librairie-v1.1.0-linux-amd64 ./cmd
sha256sum dist/defta-librairie-v1.1.0-linux-amd64 \
  > dist/defta-librairie-v1.1.0-linux-amd64.sha256
```

Au lancement, fournir au minimum `VERSION=1.1.0`,
`BUILD_DATE=2026-09-14`, un `JWT_SECRET` issu du gestionnaire de secrets et
`AUTH_COOKIE_SECURE=true`. Ne jamais incorporer `.env`, la base SQLite ou une
sauvegarde dans l’artefact.

## 3. Taguer et publier

Vérifier que `HEAD` est toujours le SHA validé, puis :

```sh
git tag -a v1.1.0 -m "Defta Librairie 1.1.0"
git show --no-patch --decorate v1.1.0
git push origin v1.1.0
```

Créer la release GitHub `v1.1.0` avec les notes de `CHANGELOG.md`, le binaire
et son fichier SHA-256. Le tag publié ne doit jamais être déplacé.

Les exécutables Windows AMD64 et Raspberry Pi sont construits à partir du tag
par `.github/workflows/release-binaries.yml`. La procédure et le contenu des
archives sont détaillés dans `RELEASE_ARTIFACTS.md`.

## 4. Déployer et vérifier

Sauvegarder puis tester la base avec `SQLITE_RESTORE.md`. Après démarrage,
contrôler les sondes, la version, les métriques et les smoke tests de
`DELIVERY.md`.

## Retour arrière

Arrêter l’écriture, conserver la base courante pour analyse et restaurer le
dernier couple compatible connu : ancien binaire et sauvegarde testée.
Documenter l’heure, le SHA, la sauvegarde employée et la décision de reprise.
