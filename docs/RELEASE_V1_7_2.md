# Livraison v1.7.2 — correctif de sécurité

Publiée le 2 octobre 2026 : tag annoté immuable `v1.7.2` sur
`92536cfdab6be8851be642c926b1a84cfc1746ab`. Audit #203, préparation #204
et correctif du formulaire catégories/tags #205 fusionnés ; recette locale
confirmée par l'opérateur. [Empreintes et preuves](RELEASE_ARTIFACTS.md).
La v1.7.1 reste inchangée. Le contrôle du transport en production reste distinct.

## Périmètre et preuves disponibles

L'[audit](SECURITY_AUDIT_V1_7_2.md) et l'[inventaire des 128 routes](SECURITY_ROUTES_V1_7_2.md)
décrivent les correctifs et leurs limites : Go 1.26.7+, x/crypto 0.56.0,
CSP sans unsafe-inline, polices locales, CSRF signé, protection des statiques,
DTO public et erreurs génériques, verrouillage progressif, HTTPS/HSTS en mode
production explicitement configuré. Pas de changement de migration ni de
politique métier/OCR : le flag expérimental reste false par défaut.

Le commit final de l'audit `83bbfd704bd268359b626c835099658a3df4e87a` avait
six workflows verts, 33 tests navigateur et 125 tests frontend réussis.
[CI de sécurité vérifiée](https://github.com/kharmaodo/defta-librairie/actions/runs/36995274047).
Ces preuves ne valident pas un futur commit de préparation ou tag : relever
et contrôler le SHA exact après fusion. govulncheck signalait 0 vulnérabilité
atteignable et 1 avis module-only openpgp non importé ; les exceptions gosec
locales sont documentées dans l'audit. Aucune certification ASVS ni qualité
arabe réelle revendiquée.

## Gate du candidat exact

`Release v1.7.2 gate` réutilise le gate complet v1.7.1 : Go/FTS5, race, vet,
125 tests frontend et tous les parcours navigateur, build Vite reproductible,
restauration SQLite, OCR Python/CPU/conteneur privé, adaptateur, matching
synthétique et intégration privée MinIO/JetStream sur volumes jetables.
Il ajoute govulncheck v1.8.0, gosec v2.29.0, staticcheck v0.8.1 et les tests
contre l'inclusion de secrets/bases dans les archives. `Application security`
reste un contrôle indépendant. Les gates historiques restent disponibles.

Le job `packages` du gate v1.7.2 construit aussi les archives Windows amd64 et Linux amd64 sans les publier. Il utilise le même script `scripts/build-release-packages.sh` que la publication, puis vérifie le runtime Go et la plateforme compilée dans chaque binaire, les trois polices locales et leurs licences, les métadonnées et les empreintes SHA-256.

Sur un checkout propre du SHA exact avec Go 1.26.7+, Node, Chromium,
Python/dépendances OCR, Tesseract arabe et Docker Compose :

```sh
go install golang.org/x/vuln/cmd/govulncheck@v1.8.0
go install github.com/securego/gosec/v2/cmd/gosec@v2.29.0
go install honnef.co/go/tools/cmd/staticcheck@v0.8.1
sh scripts/check-release-v1.7.2.sh
```

Le script ne lance pas les services d'intégration ; le workflow les prépare
et exécute ensuite les tests correspondants. Conserver SHA et liens CI verts.
Le changement de SHA impose de valider à nouveau le candidat. Ne pas faire
de reset/clean sur un checkout contenant des modifications utilisateur.

## Recette locale Linux/WSL sur une copie

La recette v1.7.1 déjà réalisée ne remplace pas celle du correctif. Garder le
relais Windows 127.0.0.1 → ::1 qui fonctionne ; aucune règle portproxy ni
élévation n'est nécessaire pour ce jalon. `PUBLIC_ORIGIN` doit rester vide
pour cette recette HTTP locale. Le `.env` existant reste privé et intact.

1. Relever branche/SHA et `git status --short`. Si le checkout contient du
   travail, conserver ce travail et utiliser un checkout séparé. Préparer
   les variables privées dans ce contexte sans les afficher ni les committer.
2. Sauvegarder la base active ; restaurer dans un nouveau fichier, sans toucher
   à la base active ni à ses sidecars. Exemple depuis le dépôt :

```sh
DEFTA_RECETTE_DIR=$(mktemp -d "$PWD/data/recette-v1.7.2-XXXXXX")
DEFTA_BACKUP_DIR="$DEFTA_RECETTE_DIR/backup" bash scripts/backup-db.sh
# Reprendre le chemin de sauvegarde affiché, puis :
python3 scripts/restore-db.py <SAUVEGARDE_VERIFIEE> \
  --output "$DEFTA_RECETTE_DIR/defta-restored.db"
```

3. Arrêter l'ancien processus applicatif, puis lancer le candidat depuis son
   checkout (templates et static de la même révision), avec ses variables privées :

```sh
DB_PATH="$DEFTA_RECETTE_DIR/defta-restored.db" PUBLIC_ORIGIN= \
OCR_EXPERIMENTAL_ENABLED=false VERSION=1.7.2 BUILD_DATE=2026-10-02 \
go run -tags fts5 ./cmd
```

BUILD_DATE doit correspondre à la date réelle du build utilisé ; si la
publication a lieu un autre jour, reprendre BUILD-INFO.txt. La copie SQLite
seule n'isole pas MinIO/JetStream : pour les essais qui écrivent (uploads,
ventes, paiements, retours), utiliser des services et identifiants de recette
séparés. Ne pas envoyer les workers d'une copie sur les ressources actives.

4. Depuis PowerShell avec le relais en marche :

```powershell
curl.exe --noproxy "*" -I http://127.0.0.1:8080/login
curl.exe --noproxy "*" http://127.0.0.1:8080/api/health/live
curl.exe --noproxy "*" http://127.0.0.1:8080/api/health/ready
curl.exe --noproxy "*" -I http://127.0.0.1:8080/static/
curl.exe --noproxy "*" -i http://127.0.0.1:8080/api/admin/owners
```

Attendus : santé alive/ready, CSP stricte (sans unsafe-inline ni Google Fonts),
static/ 404, API admin anonyme 401. HSTS et Secure ne sont pas attendus sur
HTTP local. Dans le navigateur : login, refresh/logout, catalogue et auteur
correct, modes carte/table sans statut interne, sessions, couvertures/import,
recherche manuelle et remplacement, exports/impression, thèmes et mobile.
Consigner erreurs console/CSP éventuelles et le résultat de chaque parcours.

## Production : contrôle distinct

Avant d'utiliser PUBLIC_ORIGIN : domaine/certificat et proxy sur le même
hôte que le backend, conformément à [l'exemple nginx](../deploy/nginx-security.conf.example).
Vérifier HTTPS pour tous les sous-domaines avant HSTS includeSubDomains.
Le backend écoute alors 127.0.0.1 ; seuls TLS direct ou X-Forwarded-Proto
HTTPS depuis le proxy loopback sont reconnus. Une topologie de proxy sur
un autre hôte/conteneur doit être étudiée explicitement avant activation.

Contrôler HTTP→HTTPS, TLS 1.2/1.3, HSTS, CSP, Secure/HttpOnly/SameSite du
refresh cookie et refus des mutations cookie sans CSRF. Ne pas tester de
mutations destructrices sur les données réelles. Aucun domaine/certificat
réel n'a été vérifié ici. Examiner les anciens journaux : si la configuration
contenant des secrets a été enregistrée, restreindre l'accès et renouveler
JWT/MinIO/NATS concernés selon la procédure opérateur. Ne pas copier ces
journaux dans une issue publique et ne pas envoyer de secrets pour vérification.

## Procédure de publication — exécutée pour v1.7.2

Les commandes ci-dessous décrivent la procédure déjà exécutée ; ne pas recréer
ni remplacer le tag v1.7.2 existant. Pour une prochaine version, adapter tag et SHA.
Relever le SHA final de develop, preuves CI et recette. Le tag annoté doit
référencer ce SHA exact, pas le SHA de départ ni un ancien commit de v1.7.1 :

```sh
git tag -a v1.7.2 <SHA_FINAL_VALIDE> -m "Defta Librairie 1.7.2 — sécurité"
git show --no-patch --decorate v1.7.2
git push origin v1.7.2
gh workflow run release-binaries.yml --ref develop -f tag=v1.7.2
```

Le tag v1.7.2 déclenche le worker Linux AMD64 avec SBOM/provenance.
Le workflow binaire produit les archives Windows/Linux, SHA256SUMS et
BUILD-INFO.txt. Ce dernier est aussi embarqué dans chaque archive, avec
VERSION, BUILD_DATE, SOURCE_COMMIT et GO_VERSION. Le vérificateur lit le
Go réellement compilé via `go version -m` sans exécuter le binaire, exige
Go >=1.26.7, contrôle version/SHA, actifs/police/licence et absence de données
privées, chemins traversants, liens ou fichiers spéciaux. La publication
n'écrase plus les assets existants ; après échec partiel, inspecter les assets
publiés avant reprise, sans les remplacer automatiquement.

Après téléchargement indépendant :

```sh
sha256sum -c SHA256SUMS
python3 scripts/check-release-packages.py --version 1.7.2 \
  --commit <SHA_FINAL_VALIDE> defta-librairie-1.7.2-windows-amd64.zip \
  defta-librairie-1.7.2-linux-amd64.tar.gz
```

Reporter hashes, liens CI, digest worker, BUILD-INFO et résultat de vérification
dans [RELEASE_ARTIFACTS.md](RELEASE_ARTIFACTS.md) seulement après publication
réelle. Un BUILD-INFO n'est pas une signature cryptographique. Le service OCR
Python reste séparé des archives et sous les limites déjà documentées.

## Retour arrière

Aucune migration nouvelle dans ce patch. Conserver snapshots SQLite et
état MinIO/JetStream cohérents ; ne pas écraser la base active avec la copie
de recette. L'arrêt d'un candidat laisse les données originales intactes si
la recette a utilisé les ressources séparées prescrites. Une remise en service
de v1.7.1 réintroduit l'ancien runtime et les problèmes de l'audit : préférer un
correctif en avant, et encadrer tout retour temporaire selon le risque réel.
Le rollback OCR reste le flag false ; aucune activation générale par ce jalon.

## État vérifié au 2 octobre 2026

| Preuve | État |
|---|---|
| Audit #203 fusionné | Confirmé : c094673 |
| Gates sur le SHA tagué `92536cf` | Quatre workflows post-fusion verts ; liens dans RELEASE_ARTIFACTS.md |
| Recette locale v1.7.2 / copie restaurée / ressources séparées | Confirmée par l’opérateur, catégories/tags retestés après #205 |
| TLS réel et traitement des anciens secrets/journaux | À vérifier selon déploiement |
| Tag et artefacts v1.7.2 / hashes / digest | Publiés et archives téléchargées/vérifiées ; preuves dans RELEASE_ARTIFACTS.md |
