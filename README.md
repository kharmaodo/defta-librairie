# Defta Librairie

Le suivi des douze priorités du projet est centralisé dans [BACKLOG.md](BACKLOG.md). Ce fichier distingue les fonctions livrées des travaux restant à finaliser.

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

Les routes de lecture du catalogue restent publiques. Les routes de gestion appliquent systématiquement l'authentification JWT puis les règles suivantes :

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

Le cycle est `DRAFT → RECEIVED` ou `DRAFT → CANCELLED`. La réception augmente les stocks, crée les mouvements `ENTRY` et inscrit les audits correspondants dans une transaction atomique.

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

La suppression fonctionnelle utilise le statut `DISABLED` afin de préserver l’historique commercial. Les contraintes empêchent le rattachement d’un client à une librairie inexistante et les index préparent la recherche par nom, téléphone ou e-mail. Le rattachement facultatif aux ventes est implémenté par la migration 014. L’action **Historique** de l’écran Clients ouvre les ventes rattachées au client, même désactivé, avec pagination et filtres de période et de statut.

Le CRUD client est accessible aux rôles `OWNER_LIBRARY` et `SUPER_ADMIN_ROOT`. Le propriétaire reste limité aux clients de sa librairie ; le root peut préciser `libraryId`. La recherche couvre la référence, le nom, le téléphone et l’e-mail. Chaque mutation contrôle la version et produit un audit de type `CUSTOMER`.

- `GET /api/manage/customers?q=&status=&libraryId=&offset=0&limit=30` ;
- `POST /api/manage/customers` ;
- `GET /api/manage/customers/{id}` ;
- `PUT /api/manage/customers/{id}` ;
- `DELETE /api/manage/customers/{id}?version={version}` ;
- `POST /api/manage/customers/{id}/reactivate?version={version}`.

La désactivation est logique et conserve le client ainsi que ses rattachements aux ventes. Plusieurs clients peuvent porter le même nom ; chacun reçoit une référence générée au format `C-AAAAMMJJ-XXXXXXXX`.

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

SQLite interdit de retourner davantage d'exemplaires qu'il n'en a été vendu, en tenant compte des retours antérieurs déjà finalisés. Une finalisation vide est refusée. Les règlements de retour acceptent espèces, mobile money, carte ou avoir selon la résolution choisie, sans pouvoir dépasser le montant total du retour. La vue `customer_return_balances` expose le montant traité, le reste et l'état `PENDING`, `PARTIALLY_SETTLED` ou `SETTLED`.

Le second incrément expose la gestion métier des retours :

- `GET /api/manage/customer-returns` liste les retours avec pagination et filtres `status`, `saleId`, `customerId`, `from`, `to` et, pour le root seulement, `libraryId` ;
- `POST /api/manage/customer-returns` crée un brouillon à partir des identifiants de lignes d'une vente confirmée ; les titres et prix sont toujours relus côté serveur ;
- `GET /api/manage/customer-returns/{id}` consulte un retour dans le périmètre de la librairie connectée ;
- `PUT /api/manage/customer-returns/{id}` remplace les lignes d'un brouillon avec contrôle optimiste par `version` ;
- `POST /api/manage/customer-returns/{id}/complete` finalise le retour et restitue atomiquement les quantités au stock ;
- `POST /api/manage/customer-returns/{id}/cancel` annule un brouillon sans modifier le stock.

La finalisation produit un mouvement `ENTRY` par livre, des audits `UPDATE_INVENTORY` et un audit `COMPLETE_CUSTOMER_RETURN`. Les transitions répétées, versions obsolètes et dépassements des quantités vendues retournent `409 Conflict`. Les retours d'une autre librairie restent invisibles (`404 Not Found`).

Les retours finalisés peuvent ensuite être réglés, en une ou plusieurs fois, avec les routes suivantes :

- `GET /api/manage/customer-returns/{id}/settlements` liste les règlements du retour avec les filtres `method` et `status` ;
- `POST /api/manage/customer-returns/{id}/settlements` émet un remboursement `CASH`, `MOBILE_MONEY`, `CARD` ou un avoir `CREDIT_NOTE` ;
- `GET /api/manage/customer-returns/{id}/settlement-balance` expose le total, le montant réglé, le reste et l'état financier ;
- `POST /api/manage/return-settlements/{id}/void` annule un règlement avec son numéro de version et un motif obligatoire.

La méthode doit correspondre à la résolution du retour : `CREDIT_NOTE` pour un avoir, et une méthode monétaire pour `REFUND`. Le cumul des règlements actifs ne peut jamais dépasser le total du retour. L'annulation conserve la ligne avec le statut `VOIDED`, rétablit le solde disponible et produit les audits `ISSUE_RETURN_SETTLEMENT` et `VOID_RETURN_SETTLEMENT`.

Le tableau de bord `/admin` liste les retours avec leurs filtres et leur pagination. Pour chaque retour finalisé, l'action **Règlements** affiche le total traité, le reste à rembourser et l'historique conservé. Elle permet d'émettre un remboursement ou un avoir compatible avec la résolution choisie, puis d'annuler un règlement avec un motif obligatoire.

L'action **Nouveau retour** sélectionne une vente confirmée et les quantités réellement reçues. Le brouillon obtenu peut être finalisé pour restituer atomiquement le stock ou annulé sans mouvement. Le root choisit d'abord la librairie, ce qui limite immédiatement les ventes proposées.

SQLite contrôle que la vente et la caisse appartiennent à la même librairie, que la caisse est active, que la vente est confirmée et que le cumul des règlements ne dépasse jamais son total. La vue `sale_payment_balances` calcule le montant payé, le reste à payer et l’état financier `UNPAID`, `PARTIALLY_PAID` ou `PAID`. Un règlement annulé conservera sa ligne avec le statut `VOIDED` pour assurer la traçabilité.

### Retours fournisseurs

La migration `017_create_supplier_returns.sql` pose la fondation des retours vers les fournisseurs. Un retour est obligatoirement rattaché à un achat `RECEIVED`, au fournisseur et à la même librairie. Ses lignes référencent les lignes réellement réceptionnées et reprennent le livre, le titre et le coût unitaire historiques.

Le cycle est `DRAFT → SHIPPED` ou `DRAFT → CANCELLED`. SQLite refuse les lignes étrangères à l'achat, les brouillons vides lors de l'expédition et le cumul de quantités supérieur à la quantité reçue. Les brouillons concurrents réservent les quantités disponibles ; leur annulation les libère. L’expédition diminue atomiquement le stock et produit des mouvements `EXIT` ainsi que les audits associés.

Le CRUD des brouillons est exposé par `GET|POST /api/manage/supplier-returns`, `GET|PUT /api/manage/supplier-returns/{id}` et `POST /api/manage/supplier-returns/{id}/cancel`. La liste accepte `status`, `purchaseId`, `supplierId`, `from`, `to`, `offset`, `limit` et, pour le root, `libraryId`. Chaque création, modification et annulation produit un audit dédié et respecte le contrôle optimiste par `version`.

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

Chaque incrément part de `develop` et revient dans `develop` par pull request. Les anciennes branches de réécriture sont déjà fusionnées.

### Expédition des retours fournisseurs

`POST /api/manage/supplier-returns/{id}/ship` accepte `{"version":1}`. Seul un brouillon de la librairie autorisée peut être expédié. Une transaction unique enregistre l’état `SHIPPED`, diminue les stocks, crée les mouvements `EXIT` et les audits `UPDATE_INVENTORY` et `SHIP_SUPPLIER_RETURN`. Un stock insuffisant retourne `409 supplier_return_insufficient_stock` et annule toutes les écritures. Les versions obsolètes et transitions répétées sont refusées. Sauvegarder la base avant les tests locaux.

Le test `TestSupplierReturnShipInsufficientStockRollsBack` utilise une base temporaire et les migrations réelles. Il vérifie le refus pour stock insuffisant, y compris après une première ligne traitée, et exige que le brouillon, les quantités, versions, dates, mouvements et audits restent inchangés. Exécution ciblée : `go test -tags fts5 ./internal/services -run TestSupplierReturnShipInsufficientStockRollsBack -count=1 -v`.

Le test d’expédition couvre aussi l’isolation entre librairies, les versions obsolètes, une expédition valide et le refus d’une répétition sans seconde sortie de stock ni audit supplémentaire. Ces contrôles utilisent exclusivement la base temporaire de test.

### Tableau de bord des retours fournisseurs

La section Retours fournisseurs de `/admin` propose liste paginée, filtre de statut, création depuis un achat réceptionné, modification des quantités et du motif d’un brouillon, annulation et expédition confirmée. Les retours terminaux sont consultables en lecture seule. Le root sélectionne la librairie ; les propriétaires restent limités à leur périmètre JWT. Les contrôles de quantité et de version restent réalisés par le serveur. Après une expédition, actualiser la section Stocks pour consulter les nouvelles quantités. Sauvegarder SQLite avant les essais métier.

### Coût moyen pondéré — première étape

La migration `018_add_inventory_average_cost.sql` ajoute `book_inventory.average_unit_cost`. Le CMP est recalculé dans la transaction de réception : (stock avant × CMP avant + quantité reçue × coût unitaire) / stock après. Quand le stock avant est nul, le coût de la réception devient le CMP. Un coût gratuit connu vaut zéro ; un coût inconnu vaut NULL. Les stocks historiques positifs conservent un CMP inconnu, même après réception, plutôt que d’estimer leur valorisation.

La migration 018 couvre les réceptions. Les coûts figés des ventes et des retours, les règles de valorisation des entrées manuelles et les statistiques de marge sont également implémentés et décrits ci-dessous. Les tests couvrent la moyenne pondérée, le stock vide, le coût nul connu, le coût historique inconnu et la réception transactionnelle. Aucune marge historique n’est reconstituée.

### Coûts figés des ventes

La migration `019_freeze_sale_cost.sql` ajoute `sale_lines.unit_cost_snapshot`, nullable. La confirmation copie le CMP courant dans chaque ligne dans la même transaction que la sortie de stock et les audits. Les ventes historiques restent sans coût connu ; aucune estimation rétroactive n'est appliquée. Le coût reste consultable en SQL et n'est pas encore exposé par l'API.

L'annulation restitue le stock au coût figé, en recalculant sa moyenne avec le stock présent. Un coût de sortie inconnu rend la valorisation résultante inconnue. Les coûts figés restent conservés après annulation. Un échec de confirmation annule aussi les écritures de coût. Les tests vérifient le gel du coût, sa conservation après changement du CMP, la revalorisation à l'annulation et le rollback pour stock insuffisant.

Sauvegarder SQLite avant de redémarrer pour appliquer les migrations. La valorisation des ajustements manuels et les statistiques de marge sont décrites dans les sections suivantes.

### Valorisation des retours clients

La finalisation d'un retour client restitue chaque livre au coût figé de sa ligne de vente (`sale_lines.unit_cost_snapshot`). Le CMP est recalculé avec le stock présent, dans la transaction qui enregistre les quantités, mouvements et audits. Le prix de vente et le montant du remboursement ne servent pas à valoriser le stock. Un coût historique inconnu rend le CMP résultant inconnu ; zéro reste un coût connu. Le coût de la vente d'origine n'est pas modifié.

Aucune migration supplémentaire n'est nécessaire après 018 et 019. Les retours déjà finalisés ne sont pas recalculés rétroactivement. Les tests couvrent la moyenne pondérée, les coûts inconnus ou nuls, le refus d'une finalisation répétée et le rollback du stock, du CMP et du statut en cas d'échec d'audit. Les statistiques de marge sont disponibles dans l’API et le tableau de bord.

### Ajustements manuels et CMP

Les entrées manuelles et corrections de quantité à la hausse ne fournissent aucun coût d'achat dans l'API actuelle : elles rendent le CMP inconnu (NULL). Les sorties et corrections à la baisse conservent le CMP. Un stock vidé conserve son ancien CMP à titre historique ; une prochaine réception sur stock nul initialise le CMP au coût reçu. Aucun coût ni marge historique n'est estimé. Les tests ajoutés couvrent ces quatre cas.

### Fondation des statistiques commerciales

`CommercialStatisticsRepository.Summary` calcule les ventes brutes, annulations, retours clients, ventes nettes et coûts connus pour une librairie explicite et un intervalle [from, to). Le service contrôle les droits sur la librairie avant la requête ; l’endpoint est `GET /api/manage/statistics`.

Les dates utilisées sont confirmed_at, cancelled_at et completed_at. Une annulation sur une période ultérieure ne réécrit donc pas les ventes de la période initiale. Les coûts proviennent des lignes de vente figées. Si une ligne d'événement présente un coût inconnu, netMargin reste null et unknownCostEvents indique le nombre de lignes concernées. knownCost représente uniquement la partie connue et ne doit pas être présenté comme le coût total lorsque des coûts manquent. Une période vide donne des totaux nuls. Ces indicateurs décrivent l'activité commerciale, pas les encaissements. La migration 021 empêche une double restitution par annulation et retour d’une même vente.

Tests : `go test -tags fts5 ./internal/repositories -run TestCommercialStatisticsEventsAndIsolation -count=1 -v`. Les achats, écarts de retours fournisseurs, contrôles d’accès et écran de statistiques sont également implémentés.

### API des statistiques commerciales

`GET /api/manage/statistics?from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z` retourne les indicateurs de la période [from, to). Les deux dates RFC3339 sont obligatoires. Un propriétaire utilise sa librairie JWT ; un libraryId différent est refusé (403). Le root doit préciser libraryId (400 si absent). Une période invalide ou un filtre répété retourne 400. Une authentification valide et le changement du mot de passe initial sont requis. Les réponses portent Cache-Control: no-store.

La réponse contient grossSales, cancellations, customerReturns, netSales, knownCost, unknownCostEvents et netMargin. netMargin vaut null lorsque des coûts manquent. Une librairie sans événements renvoie des totaux nuls. L’API expose également les achats et écarts fournisseurs décrits ci-dessous. Les encaissements restent suivis séparément dans les paiements ; ils ne sont pas assimilés aux ventes.

Après sauvegarde, application du patch et tests Go, redémarrer le serveur pour charger la nouvelle route. Exécuter `go test -tags fts5 ./internal/services -run TestCommercialStatisticsAuthorizationAndDates -count=1 -v` puis les suites normales et race.

### Écran des statistiques commerciales

La section Statistiques commerciales de `/admin` affiche les ventes brutes, annulations, retours clients, ventes nettes et marge commerciale. La période initiale va du premier jour du mois à aujourd'hui en UTC ; la date de fin choisie est incluse. Le propriétaire consulte sa librairie et le root doit en sélectionner une. Les résultats sont masqués dès qu'un filtre change, pendant le chargement et en cas d'erreur. Une marge inconnue affiche Indisponible avec le nombre de lignes sans coût. Les sessions expirées affichent une invitation à se reconnecter.

Vérification locale : recharger `/admin` après redémarrage du serveur ; contrôler propriétaire et root, période inversée, journée unique, période vide, coûts manquants et session expirée. Les montants utilisent F CFA comme le reste de l'interface actuelle. Le paramétrage de devise reste à développer.


### Statistiques des achats et retours fournisseurs

L’API et l’écran ajoutent `receivedPurchases`, `supplierReturns` et `netPurchases`.
Les achats RECEIVED sont comptés à received_at ; les retours SHIPPED à shipped_at,
au montant fournisseur total_amount. Même périmètre de librairie et intervalle
[from,to) que les ventes. Brouillons et annulations sont exclus. Les achats nets
peuvent être négatifs si la période contient des retours d’achats antérieurs.
Ces indicateurs ne modifient pas la marge commerciale et ne mesurent pas les
paiements. La valorisation CMP et les écarts des retours fournisseurs sont implémentés
et décrits ci-dessous ; aucun écart historique n’est estimé.

Validation : `go test -tags fts5 ./internal/repositories -run TestCommercialStatistics -count=1 -v`,
puis les suites Go et race. Dans /admin, comparer une réception et une expédition
à leurs périodes, vérifier une période vide et la sélection de librairie root.

### Coûts figés des retours fournisseurs

La migration `020_freeze_supplier_return_cost.sql` ajoute le coût unitaire de sortie
`unit_cost_snapshot` aux lignes de retour fournisseur. L’expédition fige le CMP
courant dans la transaction qui sort le stock, change le statut et écrit les audits.
Le CMP du stock restant est conservé, même si la quantité devient nulle.
Les retours historiques gardent un coût inconnu ; aucune reconstitution n’est faite.

Les lignes retournées par l’API exposent `unitCostSnapshot`, `inventoryCost`
(quantité × coût figé) et `costVariance` (montant fournisseur − coût du stock sorti).
Un écart positif signifie que le montant fournisseur excède la valeur du stock sorti ;
un écart négatif signifie l’inverse. Il ne représente pas un remboursement reçu.
Un coût inconnu donne trois valeurs null ; un coût nul connu reste zéro.
Les brouillons et retours annulés restent sans coût de sortie.
Les montants fournisseur et la marge commerciale existante ne changent pas.
Les écarts sont affichés dans le détail des retours expédiés et les statistiques de période.

Exemple : 2 livres retournés à 1 000 F CFA chacun, CMP de 800 F CFA :
montant fournisseur 2 000, coût du stock sorti 1 600, écart +400 F CFA.
Un changement ultérieur du CMP ne modifie pas ces valeurs figées.

Sauvegarder SQLite avant le redémarrage qui applique la migration 020.
Test ciblé : `go test -tags fts5 ./internal/services -run TestSupplierReturnShipInsufficientStockRollsBack -count=1 -v`.

### Affichage des coûts des retours fournisseurs

Dans `/admin` → Retours fournisseurs → Détails d’un retour expédié, chaque ligne
présente sa quantité, le montant fournisseur, le CMP figé, la valeur du stock sorti
et l’écart (montant fournisseur − valeur du stock sorti), en F CFA.
Un écart positif porte le signe +, un écart négatif conserve son signe −.
Un coût nul connu affiche zéro ; un coût inconnu affiche « Indisponible ».
L’écran affiche les valeurs figées renvoyées par l’API, sans utiliser le CMP actuel.
Le tableau défile horizontalement sur petit écran. Les brouillons restent modifiables
et les retours annulés ne présentent pas de valorisation d’expédition.
Aucune migration supplémentaire n’est nécessaire après la migration 020.

Validation : ouvrir un retour expédié connu et un historique sans coût, vérifier
un coût nul, les signes des écarts, puis rouvrir un brouillon et un retour annulé.
Vérifier les périmètres propriétaire et root et recharger la page pour le nouveau JS.

### Écarts fournisseurs dans les statistiques de période

L’API `/api/manage/statistics` et le tableau de bord incluent la valorisation des
retours fournisseurs `SHIPPED`, à leur date `shipped_at`, dans le même périmètre
JWT et l’intervalle `[from,to)` que les autres indicateurs.

- `supplierReturnKnownCost` : somme des quantités × CMP figé pour les lignes connues.
- `supplierReturnUnknownCostLines` : nombre de lignes expédiées sans coût figé.
- `supplierReturnInventoryCost` : valeur totale du stock sorti, ou `null` si un coût manque.
- `supplierReturnCostVariance` : montant fournisseur − valeur totale du stock sorti,
  ou `null` si un coût manque. Ce montant peut être positif, nul ou négatif.

Une période sans retours donne zéro, avec une valorisation complète. Les coûts
historiques inconnus ne sont jamais remplacés par le CMP actuel. Si des coûts
manquent, l’écran affiche « Indisponible » pour le total et l’écart, le nombre de
lignes concernées et la partie connue explicitement présentée comme partielle.
La marge commerciale reste indépendante : les écarts fournisseurs n’y sont pas ajoutés.
Les achats nets et les montants fournisseur existants ne changent pas.

Aucune migration après 020. Tests ciblés :
`go test -tags fts5 ./internal/repositories -run TestCommercialStatistics -count=1 -v`.
Redémarrer le serveur et recharger `/admin` : vérifier période vide, coûts connus,
nuls ou mixtes, écarts positifs/négatifs, dates limites et sélection de librairie root.

### Revue de cohérence du cycle commercial — septembre 2026

La revue ciblée porte sur les réceptions, les sorties de vente, les annulations,
les retours clients/fournisseurs, la propagation des coûts inconnus et les statistiques
par date d’événement. Les calculs existants utilisent le CMP à la réception, le coût
figé à la sortie et à la restitution, et conservent les écarts fournisseurs séparés
de la marge commerciale. Cette revue ne constitue pas une validation comptable des
encaissements et remboursements.

Deux défauts identifiés sont corrigés :

- La migration `021_guard_sale_return_cycle.sql` interdit d’annuler une vente ayant
  un retour client finalisé (`409 sale_has_completed_returns`). Elle interdit aussi
  de finaliser un retour dont la vente n’est plus confirmée (`422 sale_unavailable`).
  Ces contrôles SQLite participent aux transactions existantes : en cas de refus,
  le stock, le CMP, les versions, les dates, les mouvements et les audits sont annulés.
  Un brouillon de retour lié à une vente annulée peut encore être annulé lui-même.
- La liste des retours fournisseurs ferme le curseur des retours avant de charger
  leurs lignes, ce qui évite l’attente d’une seconde connexion lorsque le pool est
  limité à une connexion. Le test utilise un délai maximal de deux secondes.

La migration n’altère pas les données historiques. Contrôle en lecture seule pour
repérer une vente annulée ayant déjà un retour finalisé :

```sql
SELECT s.id AS sale_id, s.library_id, r.id AS return_id,
       s.cancelled_at, r.completed_at
FROM sales s JOIN customer_returns r ON r.sale_id=s.id
WHERE s.status='CANCELLED' AND r.status='COMPLETED';
```

Si cette requête renvoie des lignes, examiner les mouvements et les audits avant
une correction métier ; ne pas recalculer automatiquement le stock historique.

Tests de régression sur bases temporaires :
`go test -tags fts5 ./internal/services -run 'TestCommercialCyclePreventsDoubleRestock|TestSupplierReturnShipInsufficientStockRollsBack' -count=1 -v`.
Sauvegarder la base avant de redémarrer le serveur pour appliquer la migration 021.

### Cohérence des encaissements et remboursements

La migration `022_guard_payments_refunds.sql` ajoute trois contrôles transactionnels :

- Une vente ayant des paiements `RECORDED` ne peut pas être annulée :
  `409 sale_has_recorded_payments`.
- Le cumul des remboursements monétaires `ISSUED` de tous les retours d’une vente
  ne peut pas dépasser ses paiements `RECORDED` : `409 refund_exceeds_payments`.
  Le plafond propre au montant de chaque retour reste également appliqué.
- Annuler un paiement ne peut pas rendre les encaissements restants inférieurs aux
  remboursements déjà émis : `409 payment_has_issued_refunds`.

Le contrôle des remboursements concerne CASH, MOBILE_MONEY et CARD. CREDIT_NOTE
reste un avoir distinct, soumis au plafond du retour et à sa résolution existante.
Un enregistrement VOIDED ne participe plus aux cumuls actifs. Les montants des
ventes restent bruts : cette étape ne redéfinit pas le reste à payer après retour.
Les règles sont exécutées par SQLite dans les transactions des opérations et audits.

**Annuler un enregistrement n’est pas rembourser de l’argent.** Ne pas annuler un
paiement réel pour contourner le refus d’annulation d’une vente : utiliser le retour
client et le remboursement adapté. Les annulations servent aux corrections de saisie.

La migration n’altère pas l’historique. Contrôles en lecture seule :

```sql
SELECT s.id AS sale_id, s.library_id, SUM(p.amount) AS active_payments
FROM sales s JOIN payments p ON p.sale_id=s.id AND p.status='RECORDED'
WHERE s.status='CANCELLED' GROUP BY s.id,s.library_id;

WITH refunds AS (
  SELECT r.sale_id,SUM(rs.amount) AS refunded
  FROM return_settlements rs JOIN customer_returns r ON r.id=rs.return_id
  WHERE rs.status='ISSUED' AND rs.method IN ('CASH','MOBILE_MONEY','CARD')
  GROUP BY r.sale_id
), paid AS (
  SELECT sale_id,SUM(amount) AS received FROM payments
  WHERE status='RECORDED' GROUP BY sale_id
)
SELECT s.id AS sale_id,s.library_id,f.refunded,COALESCE(p.received,0) AS received
FROM refunds f JOIN sales s ON s.id=f.sale_id LEFT JOIN paid p ON p.sale_id=f.sale_id
WHERE f.refunded>COALESCE(p.received,0);
```

Examiner les pièces et audits si ces requêtes retournent des lignes. Aucune correction
rétroactive automatique n’est effectuée.
Sauvegarder SQLite avant le redémarrage qui applique la migration 022.
Tests : `go test -tags fts5 ./internal/services -run 'TestPaidSaleCancellationRollback|TestRefundLimitedByRecordedPayments' -count=1 -v`.

### Montant remboursable avant saisie

`GET /api/manage/customer-returns/{id}/settlement-balance` ajoute `refundableAmount` :
le minimum entre le reste à régler de ce retour et les paiements RECORDED de sa vente
moins tous les remboursements monétaires ISSUED des retours de cette vente, borné à zéro.
Le calcul est lu dans une seule requête SQLite. Un brouillon, un retour annulé ou une
vente non confirmée donne zéro. Pour CREDIT_NOTE, le champ est null : le plafond de
l’avoir reste `remainingAmount`. Les champs existants gardent leur signification.

Dans Règlements, « Remboursable maintenant » affiche ce plafond pour REFUND. Le bouton
est désactivé si le plafond est nul ou indisponible. Le solde est relu à l’ouverture
du formulaire, dont le montant proposé et le maximum utilisent ce plafond.
Après émission ou annulation, les montants sont actualisés. Les réponses tardives
à une ancienne sélection sont ignorées et un chargement échoué masque le solde.
La migration 022 conserve le contrôle transactionnel final en cas de concurrence.

Aucune migration supplémentaire. Redémarrer puis recharger `/admin`.
Test : `go test -tags fts5 ./internal/services -run 'TestRefundableBalanceAcrossReturns|TestReturnSettlementLifecycleBalanceIsolationAndAudit' -count=1 -v`.
Vérifier un retour sans encaissement, un paiement partiel, plusieurs retours de la
même vente, un retour soldé et un avoir ; contrôler propriétaire et root.

### Recette intégrée vente → paiement → retour → remboursement

Le test `TestCommercialLifecycleAcceptance` utilise une base temporaire, les migrations
réelles et les services métier. Il ne lit pas `.env` et n’utilise pas la base de travail.
Il couvre le parcours suivant avec des dates fixes :

| Étape | Stock | Encaissé brut | Remboursé | Remboursable sur le retour |
|---|---:|---:|---:|---:|
| Stock initial : 10 livres, CMP 1 000, prix 1 500 F CFA | 10 | 0 | 0 | — |
| Vente confirmée de 4 livres | 6 | 0 | 0 | — |
| Premier paiement de 1 000 | 6 | 1 000 | 0 | — |
| Retour d’un livre finalisé le lendemain | 7 | 1 000 | 0 | 1 000 |
| Premier remboursement de 700 | 7 | 1 000 | 700 | 300 |
| Complément de paiement de 5 000 | 7 | 6 000 | 700 | 800 |
| Solde du remboursement de 800 | 7 | 6 000 | 1 500 | 0 |

Sur les deux jours : ventes nettes 4 500, coût net 3 000, marge commerciale 1 500,
encaissements moins remboursements 4 500 F CFA. Le CMP reste 1 000.
Le test contrôle aussi les périodes séparées (le retour ne réécrit pas la veille),
les droits propriétaire/root, les coûts figés, les soldes, les mouvements et
l’absence d’audits supplémentaires après refus d’opérations répétées ou invalides.
Cette recette vérifie les services et la base ; elle ne remplace pas la recette HTTP
et visuelle du tableau de bord.

```bash
go test -tags fts5 ./internal/services -run '^TestCommercialLifecycleAcceptance$' -count=1 -v
go test -race -tags fts5 ./internal/services -run '^TestCommercialLifecycleAcceptance$' -count=1 -v
```

Cet incrément ajoute uniquement un test et sa documentation : aucune migration ni
modification du comportement de production.

### Recette HTTP du cycle commercial

`TestCommercialHTTPLifecycle` utilise `httptest`, les routes commerciales enregistrées
par `registerCommercialHTTPRoutes`, les vrais handlers/services/repositories,
les migrations SQLite et les middlewares JWT, session, mot de passe et rôles.
La base, les utilisateurs, les sessions et le secret JWT sont créés uniquement pour
le test. Aucun appel réseau externe ni accès à `.env` ou à la base de travail.

Le scénario crée une vente de 6 000 F CFA, encaisse 1 000, retourne un livre de
1 500, rembourse 700, encaisse les 5 000 restants et rembourse les 800 restants.
Les réponses JSON et statuts HTTP sont contrôlés, ainsi que le plafond remboursable,
le stock final de 7 livres, le CMP de 1 000 avec tolérance flottante, les ventes
nettes de 4 500 et la marge de 1 500 F CFA.

Les contrôles d’accès couvrent absence de jeton, signature invalide, changement de
mot de passe requis, accès croisé entre propriétaires, sélection de librairie root
et session révoquée. Les remboursements excessifs, répétitions et annulations
incompatibles doivent être refusés. Les routes de login/refresh et le rendu visuel
ne font pas partie de cette recette : les JWT sont signés localement pour le test.

```bash
go test -tags fts5 ./cmd -run '^TestCommercialHTTPLifecycle$' -count=1 -v
go test -race -tags fts5 ./cmd -run '^TestCommercialHTTPLifecycle$' -count=1 -v
```

L’enregistrement des routes a seulement été regroupé dans `cmd/main.go` : URL,
méthodes HTTP et protections restent identiques. Le lancement historique
`go run -tags fts5 ./cmd/main.go` reste compatible. Aucune migration supplémentaire.


### Historique des achats d’un client

`GET /api/manage/customers/{id}/sales` renvoie `results`, `total`, `offset`
(défaut 0) et `limit` (défaut 30, maximum 100). Chaque résultat contient
`id`, `reference`, `status`, `totalAmount`, `createdAt` et les dates de
confirmation/annulation lorsqu’elles existent.

```bash
curl --get "http://localhost:8080/api/manage/customers/$CUSTOMER_ID/sales" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  --data-urlencode 'from=2026-09-01T00:00:00Z' \
  --data-urlencode 'to=2026-10-01T00:00:00Z' \
  --data-urlencode 'offset=0' --data-urlencode 'limit=10'
```

Sans `status`, seules les ventes `CONFIRMED` et `CANCELLED` sont incluses.
Les filtres explicites acceptent aussi `DRAFT`, présenté comme non finalisé.
La période RFC3339 utilise une borne `from` incluse et une borne `to` exclue,
sur la confirmation (création pour un brouillon). Le tri est décroissant par
cette date puis par identifiant. Dans l’écran, les jours sont interprétés en UTC
et le jour de fin est inclus. Les montants sont les montants bruts d’origine,
avant retours et remboursements ; ils ne représentent pas le reste à payer.

Seul le rattachement `customer_id` est utilisé : un nom identique ne suffit pas.
Le propriétaire accède uniquement aux clients de sa librairie ; le root accède
au client désigné, sans paramètre `libraryId`. Un client absent ou inaccessible
renvoie 404. Les filtres invalides, inconnus ou répétés renvoient 400
`invalid_customer_history`. Les réponses portent `Cache-Control: no-store`.
Aucune migration supplémentaire n’est nécessaire.

Validation de cet incrément : `go test -tags fts5 ./...`, puis vérifier dans
Clients → Historique les clients actifs/désactivés, les filtres, la pagination
et un historique vide. Les tests dédiés portent le préfixe `TestCustomerHistory`.


### Alertes métier

`GET /api/manage/alerts` fournit une situation actuelle, sans période :
`results` (kind, resourceId, label, detail), `total`, `offset` (défaut 0),
`limit` (défaut 30, entre 1 et 100). Le filtre `kind` accepte :

- `OUT_OF_STOCK` : quantité nulle ;
- `LOW_STOCK` : quantité positive inférieure ou égale au seuil ;
- `DRAFT_PURCHASE` : achat en brouillon, pas une commande envoyée ;
- `DISABLED_SUPPLIER` : fournisseur désactivé, même sans achat en cours.

Sans `kind`, les quatre catégories sont incluses. Les livres supprimés sont
exclus ; les achats réceptionnés et annulés ne sont pas en attente. Un brouillon
auprès d’un fournisseur désactivé et le fournisseur lui-même forment deux
alertes distinctes. Le tri suit l’ordre des catégories ci-dessus, puis le
libellé et l’identifiant. Total et page proviennent du même instantané SQLite.

Le propriétaire est limité à sa librairie. Root doit fournir `libraryId` ;
une librairie sans résultat donne une liste vide. Les filtres inconnus,
répétés ou invalides donnent 400 ; un autre périmètre propriétaire donne 403.
Les réponses portent `Cache-Control: no-store`.

```bash
curl --get http://localhost:8080/api/manage/alerts \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  --data-urlencode 'kind=DRAFT_PURCHASE' \
  --data-urlencode 'offset=0' --data-urlencode 'limit=10'
```

Dans `/admin`, le tableau **Alertes métier** offre sélection de librairie
pour root, filtre de catégorie et pagination. Cliquer sur **Actualiser** après
une mutation ; l’heure de consultation est affichée. Il ne s’agit pas d’un
système de notifications automatiques. Aucune nouvelle migration.

Validation : `go test -tags fts5 ./...`, puis recette navigateur propriétaire
et root, bibliothèque vide, filtres, pagination et actualisation après une
réception ou une modification de stock. Tests dédiés : `TestBusinessAlerts`.


### Exports CSV

`GET /api/manage/exports/{kind}` accepte `stocks`, `sales`, `purchases`,
`suppliers` ou `audit`. Depuis `/admin`, utiliser **Exports CSV** : ses filtres
sont indépendants des autres tableaux. Le fichier comprend toutes les lignes
filtrées, jusqu’à 10 000 ; au-delà, 422 `export_too_large` demande de restreindre
les filtres. Un résultat vide produit les en-têtes seuls, sans erreur.

| Filtre | Signification |
|---|---|
| `libraryId` | Obligatoire pour root sur les quatre exports métier ; propriétaire limité à sa librairie. Interdit pour audit. |
| `status` | Stocks : OUT_OF_STOCK, LOW_STOCK, IN_STOCK ; ventes : DRAFT, CONFIRMED, CANCELLED ; achats : DRAFT, RECEIVED, CANCELLED ; fournisseurs : ACTIVE, DISABLED. Interdit pour audit. |
| `from`, `to` | RFC3339, début inclus et fin exclue. Date de modification pour stocks, de création pour ventes/achats/fournisseurs, d’événement pour audit. |
| `q` | Sous-chaîne littérale du titre de livre, de la référence vente/achat, du nom fournisseur ou de l’identifiant de ressource audit. Insensibilité à la casse ASCII selon SQLite. |
| `action`, `resourceType` | Égalité exacte, uniquement sur audit. |

Les jours choisis dans l’écran sont UTC, avec jour de fin inclus. Les filtres
inconnus, répétés ou invalides sont rejetés avec 400 `invalid_export_filter`.
Les requêtes n’acceptent pas de pagination : aucun résultat n’est silencieusement
tronqué. Une seule requête SELECT lit chaque export.

L’audit respecte les droits de consultation existants : seuls ses propres
événements pour un propriétaire, journal global pour root. Il exporte identité
de l’acteur, action, ressource, résultat et date ; les instantanés JSON et les
adresses IP sont omis. Ce n’est pas un audit filtré par librairie.

Le format utilise UTF-8 avec BOM, séparateur `;`, fins de ligne CRLF et
échappement CSV des guillemets, séparateurs et sauts de ligne. Les cellules
susceptibles de devenir des formules reçoivent une apostrophe protectrice ;
elles peuvent donc différer du texte stocké. Les montants sont bruts, avant
retours/remboursements, avec point décimal. Un CMP inconnu est une cellule vide.
Les livres supprimés sont exclus du stock ; les fournisseurs désactivés restent
exportables. Les réponses utilisent `Cache-Control: no-store` et un nom fixe.

```bash
curl --fail-with-body --get http://localhost:8080/api/manage/exports/sales \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  --data-urlencode 'status=CONFIRMED' \
  --data-urlencode 'from=2026-09-01T00:00:00Z' \
  --data-urlencode 'to=2026-10-01T00:00:00Z' \
  --output sales.csv
```

Validation : `go test -tags fts5 ./...`. Les tests `TestCSV` et `TestSafeCSVCell`
couvrent les droits, les cinq contenus, les filtres, les bornes 10 000/10 001,
les caractères arabes et les formules. Recette navigateur : télécharger chaque
type, vérifier un filtre et un fichier vide, comparer propriétaire et root,
puis ouvrir un fichier arabe dans le tableur. Aucune nouvelle migration.


### Paramétrage de la librairie

La migration `023_create_library_settings.sql` initialise chaque librairie
existante et future : XOF, seuil par défaut 5, coordonnées/logo/texte vides,
version 1. Elle ne modifie ni les montants ni les seuils des livres existants.
La création d’un livre prend désormais le seuil configuré dans la transaction.

`GET /api/manage/library-settings` et `PUT /api/manage/library-settings`
sont réservés aux rôles métier. Root fournit `libraryId` ; le propriétaire
est limité à sa librairie active. Le nom affiché provient du référentiel de
librairies existant et continue d’être modifié par son écran actuel.

PUT remplace les champs éditables et exige la version lue par GET :

```json
{
  "currency": "XOF",
  "address": "Dakar",
  "phone": "+221 …",
  "email": "contact@example.com",
  "logoData": "",
  "defaultLowStockThreshold": 5,
  "printFooter": "Merci de votre visite",
  "version": 1
}
```

Succès : 204. Version dépassée : 409 `version_conflict` ; données invalides :
400 `invalid_settings` ; autre librairie propriétaire : 403 ; librairie absente
ou désactivée pour le propriétaire : 404. GET utilise `Cache-Control: no-store`.
Chaque modification écrit `UPDATE_LIBRARY_SETTINGS` dans la même transaction.
Les octets du logo sont omis de l’audit.

Adresse et texte d’impression : 500 caractères chacun ; téléphone : 40 ;
e-mail : 254. Seuil entier entre 0 et 1 000 000. Logo encodé en URL de données
PNG/JPEG, 128 Kio maximum et dimensions au plus 1 024 × 1 024, décodé et validé
par le serveur. Une chaîne vide supprime le logo. La politique d’images autorise
les données embarquées pour son affichage ; SVG et URL externes ne sont pas
acceptés comme logo.

Dans `/admin`, **Paramétrage de la librairie** permet de charger et enregistrer
ces champs. Les reçus de vente et bons d’achat chargent les paramètres de la
librairie du document, indépendamment de la sélection du formulaire. Les
coordonnées sont celles de la consultation, sans instantané historique ; les
montants et lignes de vente conservent leurs valeurs existantes. XOF reste fixe :
aucune conversion ni changement d’étiquette des montants historiques.

Validation : `go test -tags fts5 ./...`, puis vérifier en propriétaire et root
les coordonnées, logo, conflit de version, nouveau livre (seuil configuré),
livre existant (seuil conservé), aperçu et impression vente/achat. Le test
`TestLibrarySettingsLifecycle` couvre défauts, isolation, validations, audit,
concurrence et application du seuil. Le changement de devise reste au backlog.


### Contrat OpenAPI et consultation locale

Le contrat **OpenAPI 3.0.3** est `static/openapi.json`, servi à
`http://localhost:8080/static/openapi.json`. Il couvre les 96 opérations
`/api/` enregistrées dans `cmd/main.go`, les schémas JSON, paramètres, droits,
codes d’erreur par famille, cookies de renouvellement, exports CSV et XOF.
Les pages HTML et ressources statiques ne sont pas des opérations de ce contrat.
Référence du format : https://spec.openapis.org/oas/v3.0.3.html.

Ouvrir `http://localhost:8080/static/api-docs.html` pour consulter et filtrer les
opérations, afficher les schémas et les exemples curl. Cette page utilise les
ressources locales et le contrat JSON ; elle n’envoie aucune opération métier.
Le fichier est importable dans les outils compatibles OpenAPI/Swagger.

`docs/API_EXAMPLES.md` contient un exemple par opération, la préparation des
variables et les variantes JSON/cookie. Adapter les identifiants, versions et
identifiants de librairie : les exemples sont indépendants, pas un script de
recette à exécuter en bloc. Ils couvrent aussi les mutations et suppressions.

```bash
python3 scripts/check-openapi.py
go test -tags fts5 ./cmd -run TestOpenAPI -count=1 -v
go test -tags fts5 ./...
```

Le contrôle Python sans dépendance vérifie la correspondance des routes,
les références, paramètres et exemples de requête. Ce contrôle structurel
n’est pas un validateur exhaustif de la norme OpenAPI. Les tests Go comparent
également les champs documentés à ceux des principaux modèles JSON.
À chaque modification d’une route ou d’un modèle, actualiser le contrat,
les exemples curl et la documentation associée. Le test doit échouer si une
route API est ajoutée, retirée ou renommée sans mise à jour du contrat.

Recette : démarrer l’application, ouvrir la page, filtrer `payments`, développer
une opération et un schéma, puis télécharger le JSON. Aucune migration ajoutée.
La priorité 9 est clôturée après validation de XOF comme devise unique ; la
priorité 10 est validée et fusionnée (PR #27).


### Contrôles de santé

Les sondes publiques ne nécessitent pas de JWT :

| Route | Succès | Indisponibilité |
|---|---|---|
| `GET /api/health/live` | 200, `{"status":"alive"}` | Dépend uniquement de la capacité du serveur à répondre. |
| `GET /api/health/ready` | 200, `{"status":"ready"}` | 503, `{"status":"not_ready","reason":"database_unavailable"}` ou motif `shutting_down`. |

La sonde ready lit `libraries` et `schema_migrations`, avec un contexte de
2 secondes au plus. Elle exige au moins une migration enregistrée ; le démarrage
applique toutes les migrations avant d’ouvrir le serveur HTTP. Elle ne renvoie
ni chemin SQLite, ni détail d’erreur. Les réponses portent `Cache-Control: no-store`.

Lors d’un SIGINT/SIGTERM, ready passe non prêt avant `Shutdown`. Le serveur
ferme ensuite l’écoute et termine les requêtes en cours : une nouvelle sonde
peut donc rencontrer un refus de connexion plutôt qu’un JSON 503. Live reste
indépendant de SQLite tant que la requête peut être traitée.

```bash
curl --fail-with-body -sS http://localhost:8080/api/health/live
curl --fail-with-body -sS http://localhost:8080/api/health/ready
python3 scripts/check-openapi.py
go test -tags fts5 ./internal/handlers -run TestHealthChecks -count=1 -v
go test -tags fts5 ./...
```

Utiliser live pour observer le processus, ready pour décider de lui envoyer du
trafic. La lecture ne teste pas les écritures, l’espace disque, l’intégrité
complète ou les restaurations. Ne pas déplacer la base active pour simuler une
panne : les tests utilisent une base temporaire, fermeture de connexion et
saturation du pool avec expiration du contexte. Aucune nouvelle migration.


### Métriques HTTP et logs structurés

`GET /api/admin/metrics` est réservé à SUPER_ADMIN_ROOT, avec session active et
mot de passe changé. Il retourne `startedAt`, `uptimeSeconds`, `inFlight`,
`completed` et `routes`. Chaque groupe contient le pattern de route, le statut,
le nombre de requêtes, les octets écrits et les durées cumulée/maximale en secondes.
La requête de métriques elle-même est encore en cours lors de son instantané.

Les compteurs sont en mémoire et repartent de zéro à chaque démarrage. Les
routes non reconnues partagent `unmatched` ; après 1 024 groupes distincts,
les nouveaux groupes sont cumulés sous `overflow` (statut 0), soit 1 025 groupes
maximum. Les métriques ne fournissent pas de percentiles et ne constituent pas
un journal d’audit. Elles couvrent aussi catalogue, fichiers statiques et sondes.

Au démarrage, slog configure la sortie standard en JSON. Les messages existants
émis par le logger standard sont également encodés en JSON. Chaque requête
terminée produit un événement `http_request` avec `request_id`, `route`, `status`,
`bytes`, `duration_ms` et `aborted`. Le pattern déclaré contient les emplacements
comme `{id}`, pas leur valeur. Aucun header d’authentification, corps JSON,
paramètre URL, adresse IP ou agent utilisateur n’est ajouté à ces événements.
Les erreurs 5xx et interruptions sont de niveau ERROR, les autres de niveau INFO.

Une panique continue de se propager vers le serveur HTTP ; l’observation libère
le compteur en cours et indique `aborted=true`. Un statut déjà envoyé reste
conservé ; sinon l’événement comptabilise 500, sans garantir qu’une réponse 500
a été reçue par le client. Les diagnostics de panique du serveur Go restent
sous sa responsabilité. `ResponseController` peut accéder au writer original.

```bash
curl --fail-with-body -sS http://localhost:8080/api/admin/metrics \
  -H "Authorization: Bearer $TOKEN"
python3 scripts/check-openapi.py
go test -tags fts5 ./internal/middleware -run 'TestHTTPObservability|TestMetricsRootGuard' -count=1 -v
go test -race -tags fts5 ./internal/middleware
go test -tags fts5 ./...
```

Recette : générer une requête catalogue, consulter les compteurs avec root,
vérifier le refus avec un propriétaire, puis comparer X-Request-ID de la réponse
et request_id du log. Redémarrer pour constater la remise à zéro. Aucune nouvelle
migration. La collecte externe, la rotation/rétention des fichiers de logs et
la restauration testée restent à organiser pour l’exploitation.


### Restauration SQLite vers un nouveau fichier

La procédure complète est dans [docs/SQLITE_RESTORE.md](docs/SQLITE_RESTORE.md).
Elle couvre préparation, maintenance, bascule par DB_PATH, recette et retour
arrière. Le script restaure une sauvegarde vers un nom neuf, vérifie intégrité,
clés étrangères et schéma, puis affiche l’empreinte du fichier restauré. Il
n’arrête pas le serveur et ne remplace jamais la base active.

```bash
python3 scripts/test-restore-db.py
# Préparation d’une restauration ; adapter les chemins, destination inexistante.
python3 scripts/restore-db.py ./data/backups/SAUVEGARDE.db \
  --output ./data/restores/NOUVEAU_FICHIER.db
```

Créer le répertoire parent au préalable. Python 3.10+ avec SQLite/FTS5 est requis
sur Linux/WSL. La recette automatisée utilise uniquement des bases temporaires.
Aucune migration ni route HTTP n’est ajoutée. Ne modifier DB_PATH qu’en suivant
la procédure de maintenance et conserver l’ancienne base pour le retour arrière.


### Accessibilité des dialogues de l’administration

Les 20 dialogues de `/admin`, y compris le retour fournisseur créé en JavaScript,
ont un titre accessible. Les boutons × portent le nom « Fermer », les messages
d’erreur utilisent `role="alert"` et les en-têtes de tableaux indiquent leur
portée de colonne. Le lien « Aller au contenu principal » apparaît au clavier
et les contrôles disposent d’un indicateur de focus visible.

```bash
python3 scripts/check-admin-accessibility.py
node --check static/js/admin-supplier-returns.js
```

La [recette clavier](docs/FRONTEND_ACCESSIBILITY.md) couvre ouverture, fermeture,
retour de focus, erreurs et impressions. Le contrôle Python est structurel ;
il ne remplace pas les tests navigateur/lecteur d’écran. Les erreurs globales
et les tests navigateur automatisés restent à finaliser dans la priorité 12.
Aucune migration ni modification de l’API.

Les statistiques, alertes, historiques clients, exports et paramètres utilisent
le client HTTP commun décrit dans [FRONTEND_HTTP.md](docs/FRONTEND_HTTP.md).
Ses tests sans dépendance s’exécutent avec `node --test scripts/test-admin-http.cjs`.

Le client HTTP commun couvre également les clients, l’approvisionnement,
les paiements/caisses et les retours clients/fournisseurs, avec des messages
français pour les refus métier. La recette est détaillée dans
[FRONTEND_HTTP.md](docs/FRONTEND_HTTP.md).

Le découpage du tableau de bord commence par le journal d’audit dans
`admin-audit.js`. Le fonctionnement et la recette sont décrits dans
[FRONTEND_MODULES.md](docs/FRONTEND_MODULES.md).

La gestion des sessions est extraite dans `admin-sessions.js` : liste, filtres,
pagination et révocations. Sa recette complète figure dans
[FRONTEND_MODULES.md](docs/FRONTEND_MODULES.md).

La gestion des propriétaires est extraite dans `admin-owners.js` (liste,
formulaires et actions administratives). Les sélecteurs de librairie restent
coordonnés par le tableau de bord. Voir [FRONTEND_MODULES.md](docs/FRONTEND_MODULES.md).

La gestion des livres est extraite dans `admin-books.js` : recherche, pagination,
formulaires, historique et suppression. Les interactions avec le stock et les
tags sont décrites dans [FRONTEND_MODULES.md](docs/FRONTEND_MODULES.md).

La gestion du stock est extraite dans `admin-inventory.js`. La recette des
mouvements, seuils et historiques est décrite dans
[FRONTEND_MODULES.md](docs/FRONTEND_MODULES.md).

La gestion des ventes est extraite dans `admin-sales.js`, avec ses catalogues
livres/clients, formulaires, transitions et impression. Voir la recette dans
[FRONTEND_MODULES.md](docs/FRONTEND_MODULES.md).
