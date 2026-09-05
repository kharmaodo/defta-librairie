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
├── data/
│   ├── catalogue.seed.db       # Catalogue initial local et ignoré par Git
│   └── defta.db                # Base privée d’exécution, ignorée par Git
├── internal/
│   ├── config/                  # Configuration
│   ├── database/                # Accès SQLite et recherche FTS5
│   ├── handlers/                # Pages HTML et API
│   └── models/                  # Modèle Book
├── static/
│   ├── css/style.css
│   └── js/main.js
├── scripts/backup-db.sh         # Sauvegarde SQLite cohérente et contrôlée
└── templates/                   # Templates Go RTL
```

`data/catalogue.seed.db` est un seed local facultatif contenant uniquement le catalogue historique initial. Il n'est jamais publié. Si `data/defta.db` est absent, l'application le copie automatiquement lorsqu'il existe, puis applique les migrations. Sans seed, une base vide est créée normalement. Les deux fichiers sont ignorés par Git afin de protéger le catalogue privé, les comptes, hashes de mots de passe, sessions et audits.

Créer localement le seed depuis une sauvegarde validée, sans le commiter :

```bash
cp data/defta.db.backup-YYYYMMDD-HHMMSS data/catalogue.seed.db

git check-ignore -v data/catalogue.seed.db
```

### Sauvegarde obligatoire avant intervention

Avant un `git pull`, un changement de branche, l'application d'un stash, une migration ou un test modifiant les données, créer une sauvegarde SQLite cohérente :

```bash
set -a
. ./.env
set +a

./scripts/backup-db.sh
```

Le script utilise l'API de sauvegarde de SQLite, contrôle `PRAGMA integrity_check` et affiche le SHA-256 du fichier placé dans `data/backups/`. Ce répertoire est exclu de Git.

Lors de chaque démarrage sur une base existante, le serveur crée également une sauvegarde cohérente par `VACUUM INTO` dans `data/backups/` et contrôle son intégrité avant toute migration. Une impossibilité de sauvegarder interrompt le démarrage : aucune migration n'est alors exécutée. Le seed local utilisé lors d'une toute première initialisation n'est pas sauvegardé avant sa copie, puisqu'il reste lui-même inchangé.

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
| `AUTH_RATE_LIMIT_REQUESTS` | `10` | Nombre de requêtes login/refresh autorisées par IP et par fenêtre |
| `AUTH_RATE_LIMIT_WINDOW_SECONDS` | `60` | Fenêtre du rate limit d'authentification |
| `AUTH_COOKIE_SECURE` | `false` | Mettre à `true` derrière HTTPS pour le cookie de refresh du navigateur |

Créer la configuration locale, qui reste ignorée par Git, puis générer un secret propre à l'environnement :

```bash
cp .env.example .env
sed -i "s|^JWT_SECRET=.*$|JWT_SECRET=$(openssl rand -base64 48)|" .env
chmod 600 .env
```

Contenu de référence de `.env.example` :

```dotenv
PORT=8080
DB_PATH=./data/defta.db
PAGE_SIZE=30
VERSION=0.2.0-dev
BUILD_DATE=2026-09-01
JWT_SECRET=
JWT_ISSUER=defta-librairie
JWT_AUDIENCE=defta-librairie-web
JWT_ACCESS_TTL_SECONDS=900
JWT_REFRESH_TTL_SECONDS=604800
AUTH_RATE_LIMIT_REQUESTS=10
AUTH_RATE_LIMIT_WINDOW_SECONDS=60
AUTH_COOKIE_SECURE=false
```

Ne jamais commiter `.env`, une sauvegarde de ce fichier, ni une valeur réelle de `JWT_SECRET`.

Sous Linux ou WSL, si `.env` a été modifié sous Windows, supprimer les retours chariot avant le lancement avec `sed -i 's/\r$//' .env`. Le chargeur neutralise également ces fins de ligne pour éviter qu'une valeur telle que `PORT=8080\r` soit transmise au serveur HTTP.

### Durcissement HTTP

Les endpoints `login` et `refresh` partagent une limite en mémoire par adresse IP. Un dépassement retourne `429 Too Many Requests` avec `Retry-After`. Le serveur ajoute également un `X-Request-ID`, désactive la mise en cache des réponses d'authentification et applique des en-têtes CSP, anti-framing, MIME sniffing, permissions et referrer. `SIGINT` et `SIGTERM` déclenchent un arrêt gracieux de 10 secondes avant la fermeture SQLite.

La route du catalogue est volontairement exacte (`GET /{$}`). Une URL inconnue, notamment sous `/api/`, retourne donc `404 Not Found` au lieu d'être rendue par erreur comme une page HTML du catalogue.

## Migrations SQLite

Les migrations embarquées sont appliquées automatiquement au démarrage, dans l'ordre et dans une transaction. La table `schema_migrations` conserve leur version et leur checksum. Une base vide est initialisée avec le catalogue `defta`, son index FTS5, puis les tables d'identité et de sécurité ; les anciennes bases restent migrées sans recréer leurs données.

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
| `GET` | `/api/admin/owners?q=...&status=...&libraryStatus=...&offset=0&limit=30` | Rechercher et paginer les propriétaires et leurs librairies |
| `POST` | `/api/admin/owners` | Créer atomiquement un propriétaire et sa librairie |
| `GET` | `/api/admin/owners/{id}` | Consulter un propriétaire |
| `PATCH` | `/api/admin/owners/{id}` | Modifier le compte, le mot de passe ou la librairie |
| `DELETE` | `/api/admin/owners/{id}` | Désactiver le compte et la librairie, puis révoquer ses sessions |
| `POST` | `/api/admin/owners/{id}/unlock` | Déverrouiller un compte bloqué après des échecs de connexion |
| `POST` | `/api/admin/owners/{id}/reactivate` | Réactiver atomiquement un compte et sa librairie désactivés |

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

La liste `GET /api/manage/books` accepte également `q`, `offset` et `limit`. Si `q` est renseigné, le backend utilise FTS5 et classe les livres par pertinence ; une expression FTS invalide bascule vers une recherche `LIKE`. Les deux chemins appliquent le même filtre de librairie issu du JWT.

```bash
curl -fsS 'http://localhost:8080/api/manage/books?q=fiqh&offset=0&limit=10' \
  -H "Authorization: Bearer $OWNER_TOKEN" | jq .
```

| Méthode | Route | Action |
|---|---|---|
| `GET` | `/api/manage/books?offset=0&limit=30&libraryId=...` | Lister les livres autorisés |
| `POST` | `/api/manage/books` | Créer un livre |
| `GET` | `/api/manage/books/{id}` | Consulter un livre autorisé |
| `GET` | `/api/manage/books/{id}/history?offset=0&limit=30` | Consulter l'historique commercial autorisé du livre |
| `PUT` | `/api/manage/books/{id}` | Remplacer les données, prix, tags et statut |
| `DELETE` | `/api/manage/books/{id}` | Supprimer logiquement un livre |

### Gestion des stocks

Chaque livre possède un état de stock versionné et un seuil d'alerte. Tous les changements produisent un mouvement immuable. Un `OWNER_LIBRARY` reste limité aux livres de sa librairie ; le `SUPER_ADMIN_ROOT` peut préciser `libraryId`. Une sortie qui rendrait le stock négatif est refusée et les écritures concurrentes utilisent le champ `version`.

| Méthode | Route | Fonction |
|---|---|---|
| `GET` | `/api/manage/inventory?status=LOW_STOCK&offset=0&limit=30&libraryId=...` | Lister le stock autorisé |
| `GET` | `/api/manage/books/{id}/inventory` | Consulter le stock d'un livre |
| `POST` | `/api/manage/books/{id}/inventory/entries` | Enregistrer une entrée positive |
| `POST` | `/api/manage/books/{id}/inventory/exits` | Enregistrer une sortie positive |
| `PUT` | `/api/manage/books/{id}/inventory` | Ajuster le stock à une quantité absolue |
| `PATCH` | `/api/manage/books/{id}/inventory/threshold` | Modifier le seuil de stock faible |
| `GET` | `/api/manage/books/{id}/inventory/movements` | Consulter l'historique paginé |

Les entrées et sorties reçoivent `{ "quantity": 5, "reason": "...", "version": 1 }`. L'ajustement reçoit `{ "quantity": 12, "reason": "inventaire physique", "version": 2 }`. Le seuil reçoit `{ "lowStockThreshold": 3, "version": 3 }`. Une version périmée répondra `409 inventory_version_conflict` et une sortie excessive `409 insufficient_stock`.

L'historique des mouvements est paginé avec `offset` et `limit` (maximum 100), trié du plus récent au plus ancien. Une modification du seuil incrémente également la version du stock et écrit l'événement `UPDATE_INVENTORY_THRESHOLD` dans le journal d'audit, sans créer de faux mouvement de quantité.

La liste des stocks accepte `LOW_STOCK` (quantité positive inférieure ou égale au seuil), `OUT_OF_STOCK` (quantité nulle) et `IN_STOCK` (quantité supérieure au seuil). Sans filtre, elle retourne tous les stocks autorisés, en présentant d'abord les ruptures puis les alertes. Seul le root peut utiliser `libraryId` pour limiter la liste à une librairie précise.

Le tableau de bord affiche cette liste avec les mêmes filtres et codes visuels. Depuis une ligne, un utilisateur autorisé peut enregistrer une entrée, une sortie, un ajustement absolu ou un nouveau seuil, puis consulter l'historique immuable des mouvements. Après chaque opération, le stock et le journal d'audit sont actualisés sans rechargement complet de la page.

```bash
curl -fsS "http://localhost:8080/api/manage/books/$BOOK_ID/inventory" \
  -H "Authorization: Bearer $OWNER_TOKEN" | jq .

jq -n '{quantity:10,reason:"Réception fournisseur",version:1}' |
curl -fsS -X POST "http://localhost:8080/api/manage/books/$BOOK_ID/inventory/entries" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -H 'Content-Type: application/json' --data-binary @- | jq .
```

Les mises à jour utilisent le champ `version`. Une version périmée produit `409 Conflict` afin d'éviter l'écrasement silencieux d'une modification concurrente. Les suppressions logiques disparaissent également du catalogue public et de la recherche FTS5.

Chaque création, modification ou suppression conserve un instantané JSON du prix, du statut, des tags et de la version. L'historique reste consultable après une suppression logique ; un propriétaire ne peut toutefois consulter que les livres rattachés à sa propre librairie.

### Gestion des ventes

Une vente appartient à une seule librairie et contient une ou plusieurs lignes. Le prix et le titre du livre sont copiés dans la ligne afin de préserver la valeur commerciale au moment de la vente. Le cycle de vie autorisé est `DRAFT → CONFIRMED → CANCELLED`.

| Méthode | Route | Fonction |
|---|---|---|
| `GET` | `/api/manage/sales?status=CONFIRMED&from=...&to=...&offset=0&limit=30&libraryId=...` | Lister les ventes autorisées |
| `POST` | `/api/manage/sales` | Créer un brouillon avec ses lignes |
| `GET` | `/api/manage/sales/{id}` | Consulter une vente et ses lignes |
| `PUT` | `/api/manage/sales/{id}` | Modifier un brouillon versionné |
| `POST` | `/api/manage/sales/{id}/confirm` | Confirmer et déduire atomiquement le stock |
| `POST` | `/api/manage/sales/{id}/cancel` | Annuler et remettre atomiquement le stock |

Le propriétaire ne peut créer ou consulter que les ventes de la librairie portée par son JWT. Le root précise `libraryId` pour une création et peut filtrer la liste globale. Une confirmation vérifie toutes les quantités avant la moindre écriture : si une ligne manque de stock, la vente, les mouvements et les quantités restent inchangés. Une annulation n'est possible qu'après confirmation et crée les mouvements inverses. Les modifications utilisent `version` et les transitions répétées sont refusées.

Exemple de brouillon :

~~~json
{
  "customerName": "Client comptoir",
  "lines": [
    {"bookId": 470, "quantity": 2}
  ]
}
~~~

La création répond `201 Created`, génère une référence `V-AAAAMMJJ-XXXXXXXX` et calcule `totalAmount` depuis les prix actuels des livres. Une modification de brouillon remplace atomiquement ses lignes, recalcule le total et incrémente `version`. Les actions `CREATE_SALE` et `UPDATE_SALE` sont enregistrées dans l'audit.

La confirmation et l'annulation reçoivent `{"version": 2}`. Elles mettent à jour tous les stocks, créent un mouvement immuable par ligne et changent le statut de la vente dans une seule transaction SQLite. Une erreur de stock ou de concurrence annule donc l'ensemble de l'opération. Les audits associés sont `CONFIRM_SALE`, `CANCEL_SALE` et `UPDATE_INVENTORY`.

### Référentiel des tags

Les tags réutilisables sont définis par librairie. Leur unicité est insensible à la casse (`Fiqh` et `fiqh` représentent le même tag). Un propriétaire utilise toujours la librairie signée dans son JWT ; le root précise `libraryId` lors de la création.

| Méthode | Route | Action |
|---|---|---|
| `GET` | `/api/manage/tags?libraryId=...` | Lister les tags autorisés |
| `POST` | `/api/manage/tags` | Créer un tag dans la librairie autorisée |
| `PATCH` | `/api/manage/tags/{id}` | Renommer un tag autorisé |
| `DELETE` | `/api/manage/tags/{id}` | Supprimer un tag autorisé |

```bash
curl -fsS -X POST http://localhost:8080/api/manage/tags \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Fiqh"}' | jq .
```

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

Le mot de passe doit contenir au moins 12 caractères, avec au minimum une majuscule, une minuscule, un chiffre et un caractère spécial. Il est stocké avec Argon2id. Cette politique s'applique à chaque création, changement ou réinitialisation sans invalider les hashes existants lors de la connexion. Les variables ne doivent pas être ajoutées au fichier `.env` versionné ni écrites dans les journaux.

Tout propriétaire nouvellement créé, ou dont le mot de passe est réinitialisé par le root, reçoit un mot de passe temporaire. Le JWT porte alors `password_change_required=true`. Seuls `/api/auth/me`, `/api/auth/change-password`, les opérations de session, le refresh et la déconnexion restent accessibles ; les routes de gestion répondent `403 password_change_required` jusqu'au changement du mot de passe. Le tableau de bord ouvre automatiquement le formulaire obligatoire sans possibilité de le fermer.

Le root utilise la route dédiée `POST /api/admin/owners/{id}/reset-password` avec `{ "password": "..." }`. L'opération révoque toutes les sessions, remet à zéro les échecs de connexion, déverrouille un compte `LOCKED`, mais conserve un compte `DISABLED` dans cet état. Aucun mot de passe n'est écrit dans l'audit `RESET_LIBRARY_OWNER_PASSWORD`.

La migration `008_create_password_history.sql` conserve uniquement les hashes Argon2id des quatre mots de passe précédents. Avec le mot de passe courant, les cinq derniers secrets ne peuvent donc pas être réutilisés. Cette règle s'applique au changement autonome, à la réinitialisation d'un propriétaire par le root et à la commande locale `reset-root-password`. L'API répond `400 invalid_new_password` lorsqu'un mot de passe récent est proposé.

```bash
read -rsp 'Nouveau mot de passe temporaire : ' DEFTA_TEMP_PASSWORD
echo
jq -n --arg password "$DEFTA_TEMP_PASSWORD" '{password:$password}' |
curl -i -X POST "http://localhost:8080/api/admin/owners/$OWNER_ID/reset-password" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data-binary @-
unset DEFTA_TEMP_PASSWORD
```

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

L'access token contient uniquement les claims nécessaires : `sub`, `role`, `library_id`, `sid`, `iss`, `aud`, `iat`, `nbf`, `exp` et `jti`. Sa durée par défaut est de 15 minutes. Le middleware vérifie le `sid` dans SQLite à chaque requête protégée : une déconnexion, une réutilisation de refresh token, la désactivation d'un compte ou de sa librairie invalide donc immédiatement l'access token.

### Changement du mot de passe

`POST /api/auth/change-password` est accessible aux deux rôles authentifiés. La requête contient `currentPassword` et `newPassword` ; le nouveau secret doit compter au moins 12 caractères et être différent de l'ancien. Après succès, toutes les sessions de l'utilisateur sont révoquées, le cookie web est supprimé et un audit `PASSWORD_CHANGED` est créé.

```json
{
  "currentPassword": "ancien-mot-de-passe",
  "newPassword": "nouveau-mot-de-passe-2026"
}
```

### Rotation et déconnexion

La connexion renvoie également un `refreshToken` opaque aux clients API. Seul son hash SHA-256 est conservé dans SQLite. Chaque appel à `/api/auth/refresh` révoque le token présenté et en émet un nouveau. La réutilisation d'un ancien token révoque toute sa famille de session et crée un audit `REFRESH_TOKEN_REUSE`.

L'interface web envoie `X-Defta-Session: cookie` afin de recevoir le refresh token dans un cookie `HttpOnly`, `SameSite=Strict`, limité au chemin `/api/auth`. Le token n'est alors jamais retourné dans le JSON ni stocké dans `sessionStorage`. Les clients externes sans cet en-tête conservent le contrat JSON existant.

### Interface d'administration

Le serveur propose une interface responsive qui s'appuie exclusivement sur les API protégées :

| URL | Accès | Fonction |
|---|---|---|
| `/login` | Public | Connexion d'un `SUPER_ADMIN_ROOT` ou `OWNER_LIBRARY` |
| `/admin` | Session JWT | Tableau de bord adapté au rôle authentifié |

Le navigateur conserve uniquement l'access token dans `sessionStorage`. Il renouvelle automatiquement la session après un `401` grâce au cookie `HttpOnly`; chaque rotation remplace ce cookie. La déconnexion révoque le refresh token côté serveur, supprime le cookie et vide la session du navigateur.

- `SUPER_ADMIN_ROOT` voit la liste des propriétaires, des librairies et le catalogue global.
- `OWNER_LIBRARY` ne voit que les livres de la librairie portée par son JWT.

Le tableau de bord permet également :

- au root de créer, modifier et désactiver un propriétaire avec sa librairie ;
- au root de choisir la librairie destinataire lors de la création d'un livre ;
- aux deux rôles de créer, modifier et supprimer les livres autorisés ;
- de gérer le prix, le volume, le statut, la catégorie, les tags et la couverture ;
- de transmettre la version courante lors d'une modification afin de détecter les écritures concurrentes.
- au root de déverrouiller explicitement un propriétaire bloqué après plusieurs échecs de connexion.

`POST /api/admin/owners/{id}/unlock` remet le compte `LOCKED` à `ACTIVE`, réinitialise `failed_login_attempts`, efface `locked_until`, révoque ses anciennes sessions et crée un audit `UNLOCK_LIBRARY_OWNER`. L'opération retourne `409 Conflict` si le compte n'est pas verrouillé.

`POST /api/admin/owners/{id}/reactivate` remet un propriétaire `DISABLED` et sa librairie à `ACTIVE`, nettoie son verrouillage, révoque préventivement ses anciennes sessions et crée un audit `REACTIVATE_LIBRARY_OWNER`. Un compte absent retourne `404 Not Found` et un compte qui n'est pas désactivé retourne `409 Conflict`.

L'interface ne constitue pas une frontière de sécurité : les contrôles d'autorisation restent appliqués par le middleware et les services backend.

### Journal d'audit

`GET /api/audit-logs` fournit une lecture paginée des événements avec les paramètres `offset`, `limit`, `actor`, `action`, `resourceType`, `resourceId`, `success`, `from` et `to`. Les dates utilisent RFC 3339. Le root voit tous les événements et peut filtrer par acteur ; un propriétaire ne voit que ceux dont `actor_user_id` correspond au sujet signé de son JWT et ne peut pas contourner ce périmètre avec `actor`.

```bash
curl -fsS 'http://localhost:8080/api/audit-logs?action=LOGIN_FAILED&success=false&limit=30' \
  -H "Authorization: Bearer $TOKEN" | jq .
```

Le tableau de bord expose les mêmes filtres avec une pagination de 20 événements. L'API reste en lecture seule ; seul le root peut rechercher un nom d'acteur.

### Sessions actives

`GET /api/auth/sessions` liste les sessions actives et accepte `offset`, `limit`, `username`, `role`, `ipAddress` et `userAgent`. Le root dispose d'une vue globale et de tous les filtres. Un propriétaire reste limité à ses sessions, peut filtrer par IP ou appareil, mais ne peut pas utiliser `username` ou `role`. `DELETE /api/auth/sessions/{id}` révoque toute la famille correspondant à un appareil et masque les sessions d'un autre compte avec `404 Not Found`. `POST /api/auth/sessions/revoke-others` révoque atomiquement toutes les autres familles du compte authentifié sans interrompre sa session courante.

La réponse indique `currentSessionId` pour identifier la session utilisée par la requête. Révoquer cette session supprime également le cookie web et impose une nouvelle connexion. Une révocation ciblée crée un audit `SESSION_REVOKED` ; la déconnexion des autres appareils crée `OTHER_SESSIONS_REVOKED` avec le nombre de familles révoquées.

```bash
curl -fsS http://localhost:8080/api/auth/sessions \
  -H "Authorization: Bearer $TOKEN" | jq .

curl -i -X DELETE http://localhost:8080/api/auth/sessions/SESSION_ID \
  -H "Authorization: Bearer $TOKEN"

curl -fsS -X POST http://localhost:8080/api/auth/sessions/revoke-others \
  -H "Authorization: Bearer $TOKEN" | jq .
```

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

### Tableau de bord des ventes

Le tableau de bord `/admin` affiche les ventes accessibles au compte connecté. Il permet de filtrer par état et par période ; le `SUPER_ADMIN_ROOT` peut également sélectionner une librairie. Les actions proposées respectent le cycle métier :

- une vente `DRAFT` peut être confirmée et retire atomiquement les quantités du stock ;
- une vente `CONFIRMED` peut être annulée et restitue atomiquement les quantités ;
- une vente `CANCELLED` est terminale et ne propose plus d'action.

Après chaque transition, l'interface actualise ensemble les ventes, les stocks et le journal d'audit. La version courante de la vente est transmise au backend afin de détecter une modification concurrente.

Le bouton **Nouvelle vente** ouvre un brouillon composé d'un client facultatif et d'une à cent lignes. Chaque livre ne peut apparaître qu'une fois et sa quantité doit être positive. Le total affiché dans le navigateur est prévisionnel : le backend relit et fige toujours le titre et le prix courants lors de l'enregistrement. Seules les ventes `DRAFT` restent modifiables.

`DELETE /api/manage/sales/{id}` supprime uniquement une vente `DRAFT` et ses lignes explicitement dans une transaction, puis conserve un audit `DELETE_SALE`. Cette suppression ne dépend donc pas de l'activation des cascades SQLite. Une vente confirmée ou annulée retourne `409 Conflict` afin de préserver l'historique commercial. La liste restitue les lignes de chaque vente afin que le nombre d'articles et le formulaire de modification utilisent toujours les données enregistrées.

L'action **Détails** relit la vente depuis `GET /api/manage/sales/{id}` et affiche une fiche imprimable : référence, client, statut, dates, titres et prix figés, quantités et total. L'impression utilise les fonctions natives du navigateur et ne transmet aucune donnée à un service externe.

### Fournisseurs et achats

La migration `011_create_suppliers_purchases.sql` pose la fondation de l'approvisionnement avec trois tables :

- `suppliers` : fournisseurs actifs ou désactivés, uniques par nom dans une librairie ;
- `purchases` : bons d'achat `DRAFT`, `RECEIVED` ou `CANCELLED` ;
- `purchase_lines` : livres, quantités, coûts unitaires et titres figés.

Toutes les données sont rattachées à une librairie. Une contrainte composite interdit notamment d'associer un fournisseur d'une autre librairie à un achat. Les montants et quantités sont contrôlés par SQLite, les références sont uniques par librairie et les versions préparent la gestion des écritures concurrentes.

La migration `012_normalize_supplier_names.sql` ajoute une clé de nom normalisée. Elle garantit l'unicité Unicode calculée par Go, notamment pour empêcher des doublons comme `Éditions Defta` et `éditions defta`, que la collation SQLite `NOCASE` seule ne détecte pas.

Le cycle prévu est `DRAFT → RECEIVED` ou `DRAFT → CANCELLED`. La réception sera implémentée comme une transaction atomique qui augmentera les stocks, créera les mouvements `ENTRY` et inscrira les audits correspondants.

Le CRUD fournisseur est accessible à `OWNER_LIBRARY` et `SUPER_ADMIN_ROOT`. Le propriétaire ne voit que les fournisseurs de sa librairie ; le root peut utiliser `libraryId`. Chaque mutation utilise `version` et crée un audit. La suppression est logique (`DISABLED`) afin de conserver les achats historiques.

- `GET|POST /api/manage/suppliers` ;
- `GET|PUT /api/manage/suppliers/{id}` ;
- `DELETE /api/manage/suppliers/{id}?version={version}` ;
- `POST /api/manage/suppliers/{id}/reactivate?version={version}`.

Les bons d'achat sont gérés sous forme de brouillons sans modifier le stock. Le backend contrôle le fournisseur actif, vérifie que chaque livre appartient à la même librairie, fige son titre et recalcule les montants à partir des quantités et coûts unitaires. Les doublons de livre sont refusés.

- `GET /api/manage/purchases?status=&supplierId=&from=&to=&libraryId=&offset=0&limit=30` ;
- `POST /api/manage/purchases` ;
- `GET /api/manage/purchases/{id}` ;
- `PUT /api/manage/purchases/{id}` ;
- `DELETE /api/manage/purchases/{id}?version={version}`.

Seul un achat `DRAFT` peut être modifié ou supprimé. Les actions créent respectivement les audits `CREATE_PURCHASE`, `UPDATE_PURCHASE` et `DELETE_PURCHASE`.

La réception utilise `POST /api/manage/purchases/{id}/receive` avec la `version` dans le corps JSON. Dans une transaction unique, elle passe le bon à `RECEIVED`, augmente chaque stock, crée les mouvements `ENTRY`, les audits `UPDATE_INVENTORY` et l'audit `RECEIVE_PURCHASE`. Toute erreur annule l'ensemble de la réception.

Un brouillon peut être abandonné avec `POST /api/manage/purchases/{id}/cancel`. Cette transition vers `CANCELLED` crée l'audit `CANCEL_PURCHASE` sans modifier le stock. Une réception ou annulation répétée retourne `409 Conflict`.

Le tableau de bord `/admin` expose désormais les fournisseurs et les bons d'achat. Les formulaires utilisent les mêmes routes protégées ; après une réception, les achats et stocks sont relus depuis le serveur.

Le formulaire de bon d'achat accepte jusqu'à cent lignes dynamiques. Il calcule un total prévisionnel, refuse les livres en double et permet de retirer une ligne avant l'enregistrement ; le serveur conserve la validation finale des quantités, coûts et montants.

Pour le `SUPER_ADMIN_ROOT`, changer la librairie du formulaire filtre les fournisseurs et les livres proposés. Après une réception réussie, le tableau de bord recharge également les stocks afin d'afficher immédiatement les nouvelles quantités.

L’action **Détails** ouvre une fiche imprimable du bon d’achat avec son fournisseur, son état, ses dates, ses lignes et son total.

La liste des achats peut être filtrée par état, fournisseur et période. Le `SUPER_ADMIN_ROOT` peut en plus sélectionner une librairie. Les résultats sont paginés par dix afin de conserver un tableau de bord lisible lorsque l’historique grandit.

### Gestion des clients

La migration `013_create_customers.sql` crée le référentiel client propre à chaque librairie. Un client possède une référence stable et unique dans sa librairie, un nom, des coordonnées facultatives, une adresse, des notes, un statut et une version pour le contrôle des écritures concurrentes.

La suppression fonctionnelle utilisera le statut `DISABLED` afin de préserver l’historique commercial. Les contraintes empêchent le rattachement d’un client à une librairie inexistante et les index préparent la recherche par nom, téléphone ou e-mail. Le rattachement facultatif aux ventes sera ajouté après validation du CRUD client.

Le CRUD client est accessible aux rôles `OWNER_LIBRARY` et `SUPER_ADMIN_ROOT`. Le propriétaire reste limité aux clients de sa librairie ; le root peut préciser `libraryId`. La recherche couvre la référence, le nom, le téléphone et l’e-mail. Chaque mutation contrôle la version et produit un audit de type `CUSTOMER`.

- `GET /api/manage/customers?q=&status=&libraryId=&offset=0&limit=30` ;
- `POST /api/manage/customers` ;
- `GET /api/manage/customers/{id}` ;
- `PUT /api/manage/customers/{id}` ;
- `DELETE /api/manage/customers/{id}?version={version}` ;
- `POST /api/manage/customers/{id}/reactivate?version={version}`.

La désactivation est logique et conserve le client pour les futurs historiques de vente. Plusieurs clients peuvent porter le même nom ; chacun reçoit une référence générée au format `C-AAAAMMJJ-XXXXXXXX`.

Le tableau de bord `/admin` expose le référentiel client avec recherche, filtre de statut et pagination. Il permet la création, la modification, la désactivation et la réactivation. Pour le root, les filtres et le formulaire de création proposent uniquement les librairies actives.

La migration `014_attach_sales_to_customers.sql` ajoute un rattachement facultatif entre une vente et un client. Le backend n’accepte qu’un client `ACTIVE` de la même librairie et SQLite protège également cette isolation par des déclencheurs. `customerId` conserve le lien vers le référentiel tandis que `customerName` reste figé dans la vente afin que les reçus historiques ne changent pas après une modification du client.

Le formulaire de vente de `/admin` propose les clients actifs de la librairie sélectionnée. Choisir un client renseigne automatiquement le nom figé du reçu. L’option « Aucun · vente comptoir » conserve la saisie d’un nom libre et le changement de librairie réinitialise le client ainsi que les articles proposés.

### Paiements et caisse

La migration `015_create_payments_cash_registers.sql` pose la fondation des règlements. Les caisses sont isolées par librairie et peuvent être désactivées sans perdre leur historique. Une vente confirmée peut recevoir plusieurs paiements par espèces (`CASH`), mobile money (`MOBILE_MONEY`) ou carte (`CARD`).

Le CRUD des caisses est exposé aux deux profils de gestion. Un `OWNER_LIBRARY` agit uniquement sur les caisses de la librairie portée par son JWT ; le `SUPER_ADMIN_ROOT` peut utiliser `libraryId` pour cibler une librairie. Les modifications et changements d'état sont versionnés et audités.

| Méthode | Route | Fonction |
|---|---|---|
| `GET` | `/api/manage/cash-registers?q=...&status=ACTIVE&offset=0&limit=30&libraryId=...` | Lister les caisses autorisées |
| `POST` | `/api/manage/cash-registers` | Créer une caisse active |
| `GET` | `/api/manage/cash-registers/{id}` | Consulter une caisse autorisée |
| `PUT` | `/api/manage/cash-registers/{id}` | Renommer une caisse avec sa `version` |
| `DELETE` | `/api/manage/cash-registers/{id}?version=...` | Désactiver une caisse |
| `POST` | `/api/manage/cash-registers/{id}/reactivate?version=...` | Réactiver une caisse |

Les règlements sont ensuite manipulés depuis une vente confirmée :

| Méthode | Route | Fonction |
|---|---|---|
| `GET` | `/api/manage/sales/{id}/payments?method=CASH&status=RECORDED&offset=0&limit=30` | Consulter les règlements d'une vente |
| `POST` | `/api/manage/sales/{id}/payments` | Enregistrer un règlement partiel ou total |
| `GET` | `/api/manage/sales/{id}/payment-balance` | Calculer le payé et le reste à payer |
| `POST` | `/api/manage/payments/{id}/void` | Annuler un règlement avec sa `version` et un motif |

L'annulation ne supprime aucune ligne : le statut devient `VOIDED`, le solde de la vente est recalculé et l'opération est inscrite dans l'audit. Les références externes permettent d'identifier les transactions mobile money ou carte et sont uniques par librairie et méthode tant que le règlement reste actif.

Le tableau de bord `/admin` contient désormais deux espaces de trésorerie. Le premier gère les caisses actives ou désactivées. Le second sélectionne une vente confirmée, affiche son total, le montant encaissé et le reste à payer, puis permet d'ajouter ou d'annuler un règlement. Pour le root, le choix de la librairie limite automatiquement les ventes et les caisses proposées.

### Retours clients et remboursements

La migration `016_create_customer_returns.sql` pose la fondation des retours clients. Un retour appartient à une vente confirmée et contient les lignes réellement retournées, valorisées au prix figé lors de la vente. Il suit le cycle `DRAFT`, `COMPLETED` ou `CANCELLED` et choisit dès sa création une résolution `REFUND` ou `CREDIT_NOTE`.

SQLite interdit de retourner davantage d'exemplaires qu'il n'en a été vendu, en tenant compte des retours antérieurs déjà finalisés. Une finalisation vide est refusée. Les futurs règlements de retour acceptent espèces, mobile money, carte ou avoir selon la résolution choisie, sans pouvoir dépasser le montant total du retour. La vue `customer_return_balances` expose le montant traité, le reste et l'état `PENDING`, `PARTIALLY_SETTLED` ou `SETTLED`.

SQLite contrôle que la vente et la caisse appartiennent à la même librairie, que la caisse est active, que la vente est confirmée et que le cumul des règlements ne dépasse jamais son total. La vue `sale_payment_balances` calcule le montant payé, le reste à payer et l’état financier `UNPAID`, `PARTIALLY_PAID` ou `PAID`. Un règlement annulé conservera sa ligne avec le statut `VOIDED` pour assurer la traçabilité.

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
