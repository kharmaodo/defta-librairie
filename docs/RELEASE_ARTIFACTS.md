# Artefacts publiés des releases

## Version 1.6.0 — 25 septembre 2026

Le tag annoté immuable `v1.6.0` référence le commit validé
`d850efbfebf1aee70903b46ad9ff1fcb2383c0b7`. Les publications ont été
déclenchées depuis `develop` et ont réussi le 25 septembre 2026.

### Application

| Archive publiée | Système cible | Architecture | Vérification |
|---|---|---|---|
| `defta-librairie-1.6.0-windows-amd64.zip` | Windows 10/11 64 bits | AMD64 | `SHA256SUMS` vérifié |
| `defta-librairie-1.6.0-linux-amd64.tar.gz` | Linux 64 bits | AMD64 | `SHA256SUMS` vérifié |

Le workflow `release-binaries.yml`, exécution `36162710304`, a publié les
archives, `BUILD-INFO.txt` et `SHA256SUMS`. La vérification locale
`sha256sum -c SHA256SUMS` a confirmé les deux archives.

### Worker Linux AMD64

Le build déclenché par le tag, exécution `36162701233`, a réussi pour
`ghcr.io/kharmaodo/defta-cover-worker:v1.6.0` sur `linux/amd64`, avec SBOM
et provenance. La publication manuelle de confirmation, exécution
`36162715257`, a également réussi. Le déploiement doit épingler le digest
vérifié de l’image et ne doit embarquer aucun secret.

### Vérification locale à archiver

```sh
release_dir=$(mktemp -d)
gh release download v1.6.0 --dir "$release_dir"
(cd "$release_dir" && sha256sum -c SHA256SUMS)
cat "$release_dir/BUILD-INFO.txt"
```

## Version 1.5.0 — 25 septembre 2026

Le tag annoté immuable `v1.5.0` référence le commit validé
`6bd578521b55c15321a8cc8ecdd2374b3388b474`. Les publications ont été
déclenchées depuis `develop` et ont réussi le 25 septembre 2026.

### Application

| Archive publiée | Système cible | Architecture | Digest GitHub |
|---|---|---|---|
| `defta-librairie-1.5.0-windows-amd64.zip` | Windows 10/11 64 bits | AMD64 | `sha256:51b20b991c0c4c3bf84e34e57ec1f2e4ce7ffd2cb968097deb6c30de2e72e3c0` |
| `defta-librairie-1.5.0-linux-amd64.tar.gz` | Linux 64 bits | AMD64 | `sha256:29d2c7f8d916af8df706c23a6bc8ebe35b584edee518a5bef3446094cad9104a` |

Le workflow `release-binaries.yml`, exécution `36126652435`, a publié ces
archives, `BUILD-INFO.txt` et `SHA256SUMS`. Le digest GitHub de
`SHA256SUMS` est
`sha256:2d35b26dc893d9650faa46aab8fa9c0266254c0f60ecb7d318cb4a5a5d986738`.

### Worker Linux AMD64

Le workflow `cover-worker-image.yml`, exécution `36126741822`, a publié
`ghcr.io/kharmaodo/defta-cover-worker:v1.5.0` pour `linux/amd64`, avec SBOM
et provenance. Son artefact de construction
`kharmaodo~defta-librairie~TR2B86.dockerbuild` porte le digest
`sha256:346787fe42d4414d0f483f95e051cc94d9aae2abdb2abf1af969ecddaf6d4f78`.
Le déploiement doit épingler le digest de l’image vérifié, sans embarquer de
secret dans l’image.

### Vérification locale à archiver

Les digests ci-dessus proviennent de la publication GitHub. La vérification
indépendante des archives reste reproductible ainsi :

```sh
release_dir=$(mktemp -d)
gh release download v1.5.0 --dir "$release_dir"
(cd "$release_dir" && sha256sum -c SHA256SUMS)
cat "$release_dir/BUILD-INFO.txt"
```

Consigner le résultat de ce contrôle avec les éléments de déploiement.

## Version 1.4.0


Le tag immuable `v1.4.0` référence le commit validé `a5025e39d936d87fb366576528c4618f3c2a1430`.
Les workflows de publication des archives et de l’image worker ont réussi le
21 septembre 2026. Les archives téléchargées ont passé le contrôle SHA-256.

## Application

| Archive publiée | Système cible | Architecture |
|---|---|---|
| `defta-librairie-1.4.0-windows-amd64.zip` | Windows 10/11 64 bits | AMD64 |
| `defta-librairie-1.4.0-linux-amd64.tar.gz` | Linux 64 bits | AMD64 |

Le workflow `.github/workflows/release-binaries.yml` construit ces archives
depuis le tag. Chacune contient l’exécutable, `templates/`, `static/`,
`.env.example` et le README ; garder ces ressources à côté de l’exécutable.
Le binaire applicatif Windows AMD64 supervise le traitement des couvertures
lorsque `COVERS_ENABLED=true`. L’archive Linux AMD64 peut héberger l’API
à côté du worker séparé.

Les archives Raspberry Pi OS ARM64 et ARMv7 sont exclues de cette publication
pour limiter le temps et les ressources de construction. La construction
locale Linux AMD64 est décrite dans `RELEASE.md`.

## Worker Linux AMD64

Le workflow `.github/workflows/cover-worker-image.yml` construit pour
`linux/amd64` depuis le tag `v1.4.0` l’image `ghcr.io/kharmaodo/defta-cover-worker:v1.4.0`
et l’alias `:1.4`, avec SBOM et provenance. Le profil Compose `worker`
construit aussi cette image localement. Le déploiement doit référencer le
digest vérifié de l’image, avec la même base SQLite persistante que l’API et
l’accès aux services MinIO/NATS ; aucun secret n’est embarqué dans l’image.

## Publication et vérification

Publication réalisée depuis le tag `v1.4.0` après la fusion du correctif du
workflow sur `develop` (PR #91). Pour vérifier à nouveau :

```sh
gh run list --workflow release-binaries.yml --limit 3
gh run list --workflow cover-worker-image.yml --limit 3
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
