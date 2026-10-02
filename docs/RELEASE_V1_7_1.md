# Release v1.7.1

Préparation du 1er octobre 2026, depuis `develop` après la fusion #200
(`59295af`). Ce SHA est le point de départ, pas le futur SHA tagué. Les
US-1712 à US-1717 sont livrées dans le périmètre expérimental accepté.
Le tag annoté et les artefacts ont été publiés le 2 octobre 2026 sur
`499825752cf1b306cb4d6bca0655fa9fc095c434`. Les [preuves d’artefacts](RELEASE_ARTIFACTS.md#version-171--2-octobre-2026)
consignent les CI, SHA256 et digest. Le déploiement reste à confirmer.
La procédure ci-dessous reste la référence de publication ; ne pas recréer le tag.

## Périmètre et limites

- OCR local CPU Tesseract arabe, cœur Python et service FastAPI privé.
- Adaptateur Go et matching de qualité sous `OCR_EXPERIMENTAL_ENABLED=false`
  par défaut ; comparateur et gate disponibles pour une recette ultérieure.
- Recherche manuelle SAFE : trois suggestions maximum, rejet persistant,
  confirmation humaine, isolation et ancienne couverture conservée jusqu’au
  succès différé du remplacement.
- Authentification JWT, rôles et politique NSFW existants conservés.

La CI technique et le corpus synthétique ne valident pas la qualité arabe sur
les couvertures réelles. Ne pas activer généralement l’OCR expérimental sur
la seule base d’une release verte. Voir [le suivi qualité et le rollback OCR](OCR_ACCEPTANCE_V1_7_1.md).

## Valider le commit candidat exact

Après fusion de la préparation, relever le SHA exact de `develop`. La CI
`Release v1.7.1 gate` doit réussir sur ce SHA : delivery Go/FTS5, race, vet,
frontend et navigateurs, build Vite reproductible, OpenAPI, restauration
SQLite, Python, conteneur CPU isolé, adaptateur réel, comparateur synthétique
et intégration privée MinIO/JetStream. L’ancien gate v1.7.0 reste disponible.

Sur un checkout propre avec Go, Node, Chromium, Docker Compose, Python et
les dépendances OCR de `services/ocr-experimental/requirements-dev.txt`,
Tesseract arabe et les polices DejaVu installés :

```sh
git fetch origin develop
git switch develop
git pull --ff-only origin develop
git status --short
git rev-parse HEAD
sh scripts/check-release-v1.7.1.sh
```

Le script réalise le gate technique ; les tests d’intégration MinIO/JetStream
sont exécutés ensuite par le workflow sur des volumes jetables. Conserver
le SHA, les liens CI et les résultats. Si le commit change, refaire les contrôles.
Un `.env` privé reste local et ne doit jamais être ajouté au commit ou à une archive.

## Préparer l’exploitation

Sauvegarder SQLite et vérifier la restauration sur une copie selon
[SQLITE_RESTORE.md](SQLITE_RESTORE.md). Les migrations sont additives et
immuables : 032/033 pour les imports/rétention, 034 pour la recherche manuelle,
035 pour les métadonnées et baux OCR. Une installation plus ancienne doit
appliquer toutes les migrations manquantes, sans modification des scripts.

Préserver la persistance SQLite, MinIO et JetStream et les droits du worker
sur la base et `data/backups`. Reprendre les variables de `.env.example`
sans écraser le `.env` existant. Le service OCR expérimental est facultatif
avec le flag off ; s’il est testé, conserver son réseau privé, ses limites
CPU/mémoire et son stockage temporaire. Aucun endpoint public d’inférence.

Sur une copie isolée avec les services requis, vérifier connexion propriétaire
et root, import SAFE, zéro/un/trois suggestions manuelles, rejet d’une solution,
annulation/confirmation de remplacement, nouvelle couverture READY et
ancienne conservée en cas d’échec. Vérifier aussi quarantaine root, refus
inter-bibliothèques, reprise sans doublon et retour au runner local avec flag off.
Consigner cette recette d’exploitation séparément des tests automatisés.

## Tag et artefacts après validation

Une fois le SHA final, la sauvegarde/restauration et la recette validés,
créer un tag annoté immuable sur le SHA exact, puis publier :

```sh
git tag -a v1.7.1 <SHA_FINAL_VALIDE> -m "Defta Librairie 1.7.1"
git show --no-patch --decorate v1.7.1
git push origin v1.7.1
gh workflow run release-binaries.yml --ref develop -f tag=v1.7.1
```

Le tag v1.7 déclenche le worker Linux AMD64 sur GHCR avec SBOM et provenance ;
le workflow des exécutables construit depuis le tag. Les archives attendues
sont `defta-librairie-1.7.1-windows-amd64.zip` et
`defta-librairie-1.7.1-linux-amd64.tar.gz`, avec `BUILD-INFO.txt` et `SHA256SUMS`.
Le service OCR Python reste un conteneur séparé construit depuis la même
révision ; il n’est pas embarqué dans les archives Go.

Télécharger les artefacts et vérifier `sha256sum -c SHA256SUMS`. Consigner le
digest du worker `ghcr.io/kharmaodo/defta-cover-worker:v1.7.1`, son SBOM et sa
provenance dans [RELEASE_ARTIFACTS.md](RELEASE_ARTIFACTS.md) seulement après
publication effective. Épingler le digest vérifié au déploiement. Définir
`VERSION=1.7.1` et `BUILD_DATE` d’après `BUILD-INFO.txt` au lancement.
Les archives ne contiennent ni `.env`, ni base, ni sauvegarde, ni image privée.

## Retour arrière

Pour désactiver uniquement l’OCR expérimental, remettre le flag à false et
redémarrer selon [le guide adaptateur](OCR_GO_ADAPTER_V1_7_1.md) ; laisser les
baux expirer et reprendre le runner local. Ne pas supprimer la migration 035,
les résultats, les candidats ou les audits.

Pour revenir à une version antérieure, arrêter les nouveaux imports et les
workers, préserver la base et les objets pour diagnostic, puis restaurer
ensemble des artefacts et une sauvegarde SQLite compatibles. Consigner SHA,
heure, sauvegarde et objets concernés. Ne jamais déplacer un tag publié.
