# Defta Librairie

Catalogue web RTL de livres en arabe, développé en Go avec SQLite et son moteur de recherche plein texte FTS5.

L’application propose une recherche classée par pertinence sur les titres, auteurs, éditeurs, mots-clés et catégories. Elle expose une interface HTML responsive ainsi qu’une API JSON paginée.

## Fonctionnalités

- recherche plein texte SQLite FTS5 compatible avec les contenus arabes ;
- classement des résultats par pertinence (`rank`) ;
- fallback `LIKE` si l’index FTS5 est indisponible ou si la requête est invalide ;
- vues grille et tableau, avec préférence conservée dans le navigateur ;
- interface RTL responsive et accessible ;
- API JSON avec pagination par `offset` et `limit` ;
- pagination serveur de l'interface avec URLs partageables (`page`) ;
- configuration par variables d’environnement.

## Stack technique

- Go 1.24.4 ;
- `net/http` et `html/template` ;
- SQLite 3 avec FTS5 ;
- `github.com/mattn/go-sqlite3` avec CGO ;
- HTML, CSS et JavaScript sans framework frontend.

## Structure

```text
.
├── cmd/main.go                  # Point d’entrée HTTP
├── data/defta.db                # Catalogue SQLite
├── internal/
│   ├── config/                  # Configuration
│   ├── database/                # Accès SQLite et recherche FTS5
│   ├── handlers/                # Pages HTML et API
│   └── models/                  # Modèle Book
├── static/
│   ├── css/style.css
│   └── js/main.js
└── templates/                   # Templates Go RTL
```

## Prérequis

Le pilote SQLite utilise CGO. Il faut donc disposer de :

- Go `1.24.4` ou une version compatible ;
- GCC ou un autre compilateur C ;
- les bibliothèques de développement SQLite sur les systèmes qui ne les fournissent pas déjà.

Vérification rapide :

```bash
go version
gcc --version
```

Sous Debian ou Ubuntu :

```bash
sudo apt-get update
sudo apt-get install -y build-essential libsqlite3-dev
```

## Installation

```bash
git clone https://github.com/kharmaodo/defta-librairie.git
cd defta-librairie
git switch develop
go mod download
```

## Configuration

Toutes les variables sont optionnelles :

| Variable | Valeur par défaut | Description |
|---|---:|---|
| `PORT` | `8080` | Port HTTP |
| `DB_PATH` | `./data/defta.db` | Chemin de la base SQLite |
| `PAGE_SIZE` | `30` | Nombre de résultats par page |
| `VERSION` | `0.1.0-dev` | Version affichée dans le pied de page |
| `BUILD_DATE` | `unknown` | Date de construction affichée |
| `JWT_SECRET` | aucune | Secret de signature, minimum 32 octets, obligatoire pour démarrer le serveur |
| `JWT_ISSUER` | `defta-librairie` | Émetteur JWT attendu |
| `JWT_AUDIENCE` | `defta-librairie-web` | Audience JWT attendue |
| `JWT_ACCESS_TTL_SECONDS` | `900` | Durée de l'access token, maximum 24 heures |
| `JWT_REFRESH_TTL_SECONDS` | `604800` | Durée du refresh token opaque, 7 jours par défaut |

Exemple `.env` :

```dotenv
PORT=8080
DB_PATH=./data/defta.db
PAGE_SIZE=30
VERSION=0.2.0-dev
BUILD_DATE=2026-09-01
JWT_SECRET=remplacer-par-un-secret-aleatoire-d-au-moins-32-octets
JWT_ISSUER=defta-librairie
JWT_AUDIENCE=defta-librairie-web
JWT_ACCESS_TTL_SECONDS=900
JWT_REFRESH_TTL_SECONDS=604800
```

## Migrations SQLite

Les migrations embarquées sont appliquées automatiquement au démarrage, dans l'ordre et dans une transaction. La table `schema_migrations` conserve leur version et leur checksum.

La première migration de sécurité crée :

- `users` pour les profils `SUPER_ADMIN_ROOT` et `OWNER_LIBRARY` ;
- `libraries` et la relation avec leur propriétaire ;
- `refresh_sessions` pour la rotation et la révocation des sessions ;
- `audit_logs` pour les actions sensibles ;
- la réparation des triggers FTS5 historiques (`categorie`) ;
- les colonnes de propriété, d'audit et de versionnement sur `defta`.

Les livres historiques sont rattachés à la librairie système :

```text
00000000-0000-0000-0000-000000000001
```

### Politique d'autorisation

Les routes de lecture du catalogue restent publiques. Les futures routes de gestion appliquent systématiquement l'authentification JWT puis les règles suivantes :

| Profil | Périmètre autorisé |
|---|---|
| `SUPER_ADMIN_ROOT` | Toutes les librairies, tous les utilisateurs et tous les livres |
| `OWNER_LIBRARY` | Uniquement les livres, prix, statuts et tags de la librairie portée par son JWT |

Un propriétaire ne peut jamais choisir son périmètre avec un champ envoyé dans le corps de la requête. Le backend utilise le `library_id` signé dans le JWT et refuse tout accès croisé avec une réponse `403 Forbidden`.

### Administration des propriétaires

Ces routes exigent un JWT `SUPER_ADMIN_ROOT` :

| Méthode | Route | Action |
|---|---|---|
| `GET` | `/api/admin/owners` | Lister les propriétaires et leurs librairies |
| `POST` | `/api/admin/owners` | Créer atomiquement un propriétaire et sa librairie |
| `GET` | `/api/admin/owners/{id}` | Consulter un propriétaire |
| `PATCH` | `/api/admin/owners/{id}` | Modifier le compte, le mot de passe ou la librairie |
| `DELETE` | `/api/admin/owners/{id}` | Désactiver le compte et la librairie, puis révoquer ses sessions |

Exemple de création :

```bash
curl -fsS -X POST http://localhost:8080/api/admin/owners \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username":"owner-one",
    "email":"owner@example.com",
    "password":"Correct-Horse-2026",
    "library":{"name":"Librairie Une","description":"Catalogue du propriétaire"}
  }' | jq .
```

### Gestion des livres

Les mutations de livres exigent un JWT `SUPER_ADMIN_ROOT` ou `OWNER_LIBRARY`. Le root précise `libraryId` lors de la création ; pour un propriétaire, le backend utilise exclusivement la librairie signée dans son JWT.

| Méthode | Route | Action |
|---|---|---|
| `GET` | `/api/manage/books?offset=0&limit=30&libraryId=...` | Lister les livres autorisés |
| `POST` | `/api/manage/books` | Créer un livre |
| `GET` | `/api/manage/books/{id}` | Consulter un livre autorisé |
| `PUT` | `/api/manage/books/{id}` | Remplacer les données, prix, tags et statut |
| `DELETE` | `/api/manage/books/{id}` | Supprimer logiquement un livre |

Les mises à jour utilisent le champ `version`. Une version périmée produit `409 Conflict` afin d'éviter l'écrasement silencieux d'une modification concurrente. Les suppressions logiques disparaissent également du catalogue public et de la recherche FTS5.

```bash
curl -fsS -X POST http://localhost:8080/api/manage/books \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title":"Nouveau livre",
    "auteur":"Auteur",
    "price":2500,
    "volume":1,
    "status":"AVAILABLE",
    "tags":"arabe,fiqh",
    "categorie":"Sciences islamiques"
  }' | jq .
```

Avant le premier lancement sur une base existante, créer une sauvegarde :

```bash
cp data/defta.db "data/defta.db.backup-$(date +%Y%m%d-%H%M%S)"
```

Après le démarrage, contrôler les migrations :

```bash
sqlite3 -header -column data/defta.db \
  "SELECT version, name, applied_at FROM schema_migrations ORDER BY version;"
```

## Bootstrap du SUPER_ADMIN_ROOT

Le premier compte racine est créé par une commande locale contrôlée. Aucun endpoint public ne permet de créer ou de promouvoir un `SUPER_ADMIN_ROOT`.

Le mot de passe doit contenir au moins 12 caractères et il est stocké avec Argon2id. Les variables ne doivent pas être ajoutées au fichier `.env` versionné ni écrites dans les journaux.

```bash
export DEFTA_ROOT_USERNAME='kharmaodo'
export DEFTA_ROOT_EMAIL='root@example.com'
export DEFTA_ROOT_PASSWORD='une-valeur-longue-et-unique'

go run -tags fts5 ./cmd/main.go bootstrap-admin

unset DEFTA_ROOT_PASSWORD
```

Résultat attendu :

```text
SUPER_ADMIN_ROOT créé → username=kharmaodo id=<uuid>
```

La commande est volontairement non répétable. Un second lancement échoue avec :

```text
a SUPER_ADMIN_ROOT already exists
```

Contrôler le compte sans afficher son hash :

```bash
sqlite3 -header -column data/defta.db \
  "SELECT id, username, email, role, status, created_at FROM users;"
```

Contrôler son audit :

```bash
sqlite3 -header -column data/defta.db \
  "SELECT action, resource_type, resource_id, success, created_at
   FROM audit_logs WHERE action = 'BOOTSTRAP_SUPER_ADMIN';"
```

### Réinitialiser le mot de passe root

Cette commande locale fonctionne sans `JWT_SECRET`. Elle remplace le hash Argon2id, déverrouille le compte, remet les tentatives à zéro, révoque toutes ses sessions et écrit un audit.

```bash
read -rsp 'Nouveau mot de passe root : ' DEFTA_ROOT_NEW_PASSWORD
echo
export DEFTA_ROOT_NEW_PASSWORD
go run -tags fts5 ./cmd/main.go reset-root-password
unset DEFTA_ROOT_NEW_PASSWORD
```

Contrôler l'opération :

```bash
sqlite3 -header -column data/defta.db \
  "SELECT action, resource_id, success, created_at
   FROM audit_logs WHERE action = 'RESET_ROOT_PASSWORD'
   ORDER BY created_at DESC LIMIT 1;"
```

## Démarrage

La balise `fts5` est obligatoire pour compiler le pilote avec le moteur plein texte :

```bash
export JWT_SECRET="$(openssl rand -base64 48)"
go run -tags fts5 ./cmd/main.go
```

Puis ouvrir :

- interface : <http://localhost:8080> ;
- API : <http://localhost:8080/api/books?q=ديوان&offset=0&limit=10>.

La page de résultats accepte aussi `page` :

```text
http://localhost:8080/?q=Anonyme&page=2
```

Test HTTP :

```bash
curl --get 'http://localhost:8080/api/books' \
  --data-urlencode 'q=ديوان' \
  --data-urlencode 'offset=0' \
  --data-urlencode 'limit=5'
```

La propriété `total` contient le nombre total de correspondances, indépendamment de la taille de la page retournée.

## Authentification JWT

Obtenir un access token :

```bash
TOKEN=$(curl -fsS -X POST http://localhost:8080/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"kharmaodo","password":"VOTRE_MOT_DE_PASSE"}' \
  | jq -r '.accessToken')
```

Ne pas écrire un véritable mot de passe dans l'historique du terminal. Pour une validation interactive :

```bash
read -rsp 'Mot de passe : ' DEFTA_LOGIN_PASSWORD
echo
TOKEN=$(jq -n --arg username 'kharmaodo' --arg password "$DEFTA_LOGIN_PASSWORD" \
  '{username:$username,password:$password}' \
  | curl -fsS -X POST http://localhost:8080/api/auth/login \
      -H 'Content-Type: application/json' --data-binary @- \
  | jq -r '.accessToken')
unset DEFTA_LOGIN_PASSWORD
```

Consulter les claims de l'utilisateur connecté :

```bash
curl -fsS http://localhost:8080/api/auth/me \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Sans token ou avec un token invalide, l'endpoint retourne `401 Unauthorized` et l'en-tête `WWW-Authenticate: Bearer`.

L'access token contient uniquement les claims nécessaires : `sub`, `role`, `library_id`, `iss`, `aud`, `iat`, `nbf`, `exp` et `jti`. Sa durée par défaut est de 15 minutes.

### Rotation et déconnexion

La connexion renvoie également un `refreshToken` opaque. Seul son hash SHA-256 est conservé dans SQLite. Chaque appel à `/api/auth/refresh` révoque le token présenté et en émet un nouveau. La réutilisation d'un ancien token révoque toute sa famille de session et crée un audit `REFRESH_TOKEN_REUSE`.

```bash
LOGIN_RESPONSE=$(jq -n \
  --arg username 'kharmaodo' \
  --arg password "$DEFTA_LOGIN_PASSWORD" \
  '{username:$username,password:$password}' \
  | curl -fsS -X POST http://localhost:8080/api/auth/login \
      -H 'Content-Type: application/json' --data-binary @-)

TOKEN=$(printf '%s\n' "$LOGIN_RESPONSE" | jq -er '.accessToken')
REFRESH_TOKEN=$(printf '%s\n' "$LOGIN_RESPONSE" | jq -er '.refreshToken')
```

Renouveler puis remplacer les deux tokens :

```bash
REFRESH_RESPONSE=$(jq -n --arg token "$REFRESH_TOKEN" '{refreshToken:$token}' \
  | curl -fsS -X POST http://localhost:8080/api/auth/refresh \
      -H 'Content-Type: application/json' --data-binary @-)

TOKEN=$(printf '%s\n' "$REFRESH_RESPONSE" | jq -er '.accessToken')
REFRESH_TOKEN=$(printf '%s\n' "$REFRESH_RESPONSE" | jq -er '.refreshToken')
```

Déconnecter toute la famille de session :

```bash
jq -n --arg token "$REFRESH_TOKEN" '{refreshToken:$token}' \
  | curl -fsS -o /dev/null -X POST http://localhost:8080/api/auth/logout \
      -H 'Content-Type: application/json' --data-binary @-

unset TOKEN REFRESH_TOKEN LOGIN_RESPONSE REFRESH_RESPONSE
```

## Tester FTS5 directement

Vérifier que SQLite a été compilé avec FTS5 :

```bash
sqlite3 data/defta.db "SELECT sqlite_compileoption_used('ENABLE_FTS5');"
```

La commande doit retourner `1`.

Contrôler le nombre de lignes de la table source et de l’index :

```bash
sqlite3 data/defta.db <<'SQL'
SELECT 'defta', COUNT(*) FROM defta;
SELECT 'defta_fts', COUNT(*) FROM defta_fts;
SQL
```

Exécuter une recherche arabe classée par pertinence :

```bash
sqlite3 -header -column data/defta.db <<'SQL'
SELECT d.id, d.title, d.auteur, fts.rank
FROM defta_fts AS fts
JOIN defta AS d ON fts.rowid = d.id
WHERE defta_fts MATCH 'ديوان'
ORDER BY fts.rank
LIMIT 10;
SQL
```

Contrôler l’intégrité logique de l’index :

```bash
sqlite3 data/defta.db "INSERT INTO defta_fts(defta_fts) VALUES('integrity-check');"
```

## Tests automatisés

Les tests de la couche de données créent une base temporaire, alimentent l’index et vérifient la recherche, le score et le total paginé :

```bash
go test -tags fts5 ./...
```

Pour détecter les problèmes de concurrence :

```bash
go test -race -tags fts5 ./...
```

## API

### `GET /api/books`

| Paramètre | Obligatoire | Description |
|---|---|---|
| `q` | Non | Expression de recherche FTS5 ; vide pour obtenir tous les livres |
| `offset` | Non | Position de départ, minimum `0` |
| `limit` | Non | Taille de page ; utilise `PAGE_SIZE` si absent ou invalide, maximum `100` |

Exemple de réponse :

```json
{
  "results": [
    {
      "id": 5,
      "title": "ديوان طرفة",
      "auteur": "Anonyme",
      "editeur": null,
      "price": 0,
      "volume": 0,
      "status": null,
      "tags": null,
      "categorie": "Non classé",
      "coverUrl": null,
      "score": -4.449599289331961
    }
  ],
  "total": 9,
  "offset": 0,
  "limit": 5
}
```

## Workflow de contribution

Les nouvelles fonctionnalités partent de `develop` et sont proposées par pull request :

```bash
git switch develop
git pull --ff-only origin develop
git switch -c feature/nom-fonctionnalite
```

Avant de pousser :

```bash
gofmt -w ./cmd ./internal
go test -tags fts5 ./...
git diff --check
```

La branche de la présente réécriture est `feature/rewriting` et sa pull request doit cibler `develop`.
