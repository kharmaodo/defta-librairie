# Exécutables multiplateformes de la version 1.2.0

Les exécutables sont publiés comme assets de la release GitHub associée au tag
immuable `v1.2.0`. Le tag référence le code source ; la release porte les archives
binaires et leurs sommes de contrôle.

## Cibles publiées

| Archive | Système cible | Architecture |
|---|---|---|
| `defta-librairie-1.2.0-windows-amd64.zip` | Windows 10/11 64 bits | AMD64 |
| `defta-librairie-1.2.0-linux-arm64.tar.gz` | Raspberry Pi OS 64 bits | ARM64, Raspberry Pi 3/4/5 |
| `defta-librairie-1.2.0-linux-armv7.tar.gz` | Raspberry Pi OS 32 bits | ARMv7, Raspberry Pi 2/3/4 |

Chaque archive contient l’exécutable, `templates/`, `static/`, `.env.example` et
le README. Ces répertoires doivent rester à côté de l’exécutable : l’application
charge les templates et les ressources statiques au démarrage.

Le Raspberry Pi Zero première génération et le Raspberry Pi 1 utilisent ARMv6
et ne sont pas couverts par ces artefacts.

## Publication de `v1.2.0`

Après fusion du workflow dans `develop`, ouvrir l’onglet **Actions**, sélectionner
**Release cross-platform binaries**, choisir **Run workflow**, conserver
`v1.2.0`, puis lancer l’exécution. Le workflow extrait le code du tag existant,
compile avec les toolchains CGO adaptées et crée ou complète la release GitHub.

La même opération peut être déclenchée avec GitHub CLI :

```sh
gh workflow run release-binaries.yml --ref develop -f tag=v1.2.0
gh run watch
```

## Vérification après téléchargement

Comparer l’archive téléchargée au fichier `SHA256SUMS` :

```sh
sha256sum -c SHA256SUMS
```

Sous Windows PowerShell :

```powershell
Get-FileHash .\defta-librairie-1.2.0-windows-amd64.zip -Algorithm SHA256
```

Avant le démarrage, copier `.env.example` vers `.env`, fournir un `JWT_SECRET`
d’au moins 32 octets et créer une sauvegarde de toute base existante. Sous Linux,
conserver le droit d’exécution du binaire extrait ; au besoin :

```sh
chmod +x defta-librairie
```
