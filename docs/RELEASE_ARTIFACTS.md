# Artefacts candidats de la version 1.4.0

Le tag immuable `v1.4.0` sera créé après la recette sur `develop`.
Aucun artefact de cette version ne doit être présenté comme publié avant le
succès des workflows et la vérification des sommes.

## Application

| Archive prévue | Système cible | Architecture |
|---|---|---|
| `defta-librairie-1.4.0-windows-amd64.zip` | Windows 10/11 64 bits | AMD64 |
| `defta-librairie-1.4.0-linux-arm64.tar.gz` | Raspberry Pi OS 64 bits | ARM64, Raspberry Pi 3/4/5 |
| `defta-librairie-1.4.0-linux-armv7.tar.gz` | Raspberry Pi OS 32 bits | ARMv7, Raspberry Pi 2/3/4 |

Le workflow `.github/workflows/release-binaries.yml` construit ces archives
depuis le tag. Chacune contient l’exécutable, `templates/`, `static/`,
`.env.example` et le README ; garder ces ressources à côté de l’exécutable.
Le binaire applicatif Windows AMD64 supervise le traitement des couvertures
lorsque `COVERS_ENABLED=true`. Les archives Raspberry Pi doivent être
vérifiées par un smoke test sur la cible avant déploiement.

Le Pi Zero première génération et le Pi 1 utilisent ARMv6 et ne sont pas
couverts. La construction locale Linux AMD64 est décrite dans `RELEASE.md` ;
le workflow ci-dessus ne publie pas d’archive Linux AMD64 de l’application.

## Worker Linux AMD64

Le workflow `.github/workflows/cover-worker-image.yml` construit pour `linux/amd64` depuis le
tag `v1.4.0` l’image `ghcr.io/kharmaodo/defta-cover-worker:v1.4.0`
et l’alias `:1.4`, avec SBOM et provenance. Le profil Compose `worker`
construit aussi cette image localement. Le déploiement doit référencer le
digest vérifié de l’image, avec la même base SQLite persistante que l’API et
l’accès aux services MinIO/NATS ; aucun secret n’est embarqué dans l’image.

## Publication et vérification

Après validation et publication du tag selon `RELEASE.md` :

```sh
gh workflow run release-binaries.yml --ref develop -f tag=v1.4.0
gh run list --workflow release-binaries.yml --limit 3
gh release view v1.4.0
release_dir=$(mktemp -d)
gh release download v1.4.0 --dir "$release_dir"
(cd "$release_dir" && sha256sum -c SHA256SUMS)
cat "$release_dir/BUILD-INFO.txt"
```

Sous PowerShell, comparer l’archive Windows à la ligne correspondante de
`SHA256SUMS` avec `Get-FileHash -Algorithm SHA256`. Vérifier le commit du tag,
le digest de l’image worker et la présence des ressources embarquées.

Avant démarrage, copier `.env.example` dans un `.env` local, renseigner
`JWT_SECRET` (au moins 32 octets), les variables MinIO et NATS si les
couvertures sont activées, et tester une sauvegarde SQLite. Sous Linux,
rétablir au besoin le droit d’exécution du binaire extrait.
