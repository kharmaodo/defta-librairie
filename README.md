# Defta Librairie

Le suivi des douze priorités du projet est centralisé dans [BACKLOG.md](BACKLOG.md). Ce fichier distingue les fonctions livrées des travaux restant à finaliser.

La refonte progressive du dashboard d’administration prévue pour `v1.1.0` est
décrite dans [docs/ADMIN_UI_V1_1.md](docs/ADMIN_UI_V1_1.md).
La feuille de route de l’expérience dashboard `v1.2.0` est définie dans
[docs/ROADMAP_V1_2.md](docs/ROADMAP_V1_2.md).
Les correctifs `v1.3.0` et la gestion des couvertures prévue pour `v1.4.0`
sont cadrés dans
[docs/ROADMAP_V1_3_V1_4.md](docs/ROADMAP_V1_3_V1_4.md).

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

- Go 1.26.0 ;
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

Les variables disposent d’une valeur par défaut sauf les secrets explicitement signalés. `JWT_SECRET` reste obligatoire ; les identifiants MinIO et NATS le deviennent lorsque `COVERS_ENABLED=true` :

| Variable | Valeur par défaut | Description |
|---|---:|---|
| `PORT` | `8080` | Port HTTP |
| `DB_PATH` | `./data/defta.db` | Chemin de la base SQLite |
| `PAGE_SIZE` | `30` | Nombre de résultats par page |
| `VERSION` | `0.1.0-dev` | Version affichée ; fournir la version publiée en production |
| `BUILD_DATE` | `unknown` | Date de construction affichée |
| `JWT_SECRET` | aucune | Secret de signature, minimum 32 octets, obligatoire pour démarrer le serveur |
| `JWT_ISSUER` | `defta-librairie` | Émetteur JWT attendu |
| `JWT_AUDIENCE` | `defta-librairie-web` | Audience JWT attendue |
| `JWT_ACCESS_TTL_SECONDS` | `900` | Durée de l'access token, maximum 24 heures |
| `JWT_REFRESH_TTL_SECONDS` | `604800` | Durée du refresh token opaque, 7 jours par défaut |
| `AUTH_RATE_LIMIT_REQUESTS` | `10` | Nombre de requêtes login/refresh autorisées par IP et par fenêtre |
| `AUTH_RATE_LIMIT_WINDOW_SECONDS` | `60` | Fenêtre du rate limit d'authentification |
| `AUTH_COOKIE_SECURE` | `false` | Mettre à `true` derrière HTTPS pour le cookie de refresh du navigateur |
| `COVERS_ENABLED` | `false` | Active l’upload MinIO et la publication asynchrone des couvertures |
| `MINIO_ENDPOINT` | `localhost:9000` | Adresse MinIO sans préfixe HTTP |
| `MINIO_ACCESS_KEY` | aucune | Identifiant MinIO, obligatoire lorsque les couvertures sont activées |
| `MINIO_SECRET_KEY` | aucune | Secret MinIO, obligatoire lorsque les couvertures sont activées |
| `MINIO_USE_SSL` | `false` | Active TLS pour la connexion MinIO |
| `MINIO_BUCKET_COVERS` | `book-covers` | Bucket privé des sources et variantes |
| `COVER_MAX_BYTES` | `5242880` | Taille maximale d’une source, 5 Mio |
| `COVER_MAX_PIXELS` | `24000000` | Plafond de pixels après lecture des dimensions |
| `MINIO_SOURCE_RETENTION_HOURS` | `24` | Rétention prévue de la source brute après traitement |
| `NATS_URL` | `nats://localhost:4222` | Adresse du serveur NATS |
| `NATS_USER` | aucune | Utilisateur NATS ; doit être fourni avec le mot de passe |
| `NATS_PASSWORD` | aucune | Mot de passe NATS ; doit être fourni avec l’utilisateur |
| `NATS_COVERS_STREAM` | `BOOK_COVERS` | Stream JetStream persistant des couvertures |
| `NATS_COVERS_SUBJECT` | `book.covers.process.v1` | Sujet versionné publié par l’outbox |
| `NATS_COVERS_CONSUMER` | `cover-worker-v1` | Consommateur durable réservé au worker |
| `COVER_WORKER_MAX_DELIVER` | `5` | Nombre maximal de livraisons du travail à traiter |

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
VERSION=1.4.0
BUILD_DATE=2026-09-21
JWT_SECRET=
JWT_ISSUER=defta-librairie
JWT_AUDIENCE=defta-librairie-web
JWT_ACCESS_TTL_SECONDS=900
JWT_REFRESH_TTL_SECONDS=604800
AUTH_RATE_LIMIT_REQUESTS=10
AUTH_RATE_LIMIT_WINDOW_SECONDS=60
AUTH_COOKIE_SECURE=false

# Couvertures de livre v1.4
COVERS_ENABLED=false
MINIO_END…15069 tokens truncated…e reconnecter.

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
méthodes HTTP et protections restent identiques. Le lancement doit cibler le
package complet avec `go run -tags fts5 ./cmd`.


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

Les tags sont extraits dans `admin-tags.js`, avec leurs suggestions pour les
livres. La recette est décrite dans [FRONTEND_MODULES.md](docs/FRONTEND_MODULES.md).

L’authentification utilise désormais le client HTTP commun, avec renouvellement
partagé et reprise limitée après 401. Voir [FRONTEND_HTTP.md](docs/FRONTEND_HTTP.md)
pour les règles et la recette des sessions.

Une première suite Chromium teste l’authentification avec un serveur et une base
SQLite temporaires. Installation et exécution : [BROWSER_TESTS.md](docs/BROWSER_TESTS.md).

La suite Chromium couvre aussi le cycle livre, stock, confirmation et annulation
d’une vente, avec vérification de la restitution exacte du stock.

Le parcours d’approvisionnement couvre deux réceptions et vérifie le coût moyen
pondé par le coût et la marge d’une vente confirmée.

Le parcours de paiement crée une caisse et règle une vente successivement en
espèces, mobile money et carte. Il vérifie les montants payés, le reste à payer
et les états non payé, partiellement payé et payé.

Le parcours de retour client vérifie sur une vente encaissée la restitution du
stock, un remboursement en espèces, un avoir et leurs traces d’audit.

Le parcours de retour fournisseur vérifie une expédition liée à un achat
réceptionné, sa sortie de stock, sa justification et sa valorisation figée.

Le parcours des exports télécharge les CSV des stocks, ventes, achats,
fournisseurs et audits, puis vérifie l’impression des reçus de vente et d’achat.

La recette d’accessibilité navigateur vérifie le lien d’évitement, le focus
visible et confiné dans un dialogue, Échap, le retour au déclencheur et les
erreurs annoncées.

La procédure de validation et de mise en production est consolidée dans
[DELIVERY.md](docs/DELIVERY.md). Le contrôle complet s’exécute avec
`./scripts/check-delivery.sh` avant de taguer une release.

Pour les imports de couvertures v1.7.0, la rétention, les retenues légales
réservées au root et le gate Go/React/Chromium/Docker sont décrits dans
[RELEASE_V1_7_0.md](docs/RELEASE_V1_7_0.md).

Le [parcours de rattachement manuel SAFE](docs/MANUAL_COVER_MATCHING.md) permet
une recherche de trois suggestions sans candidat OCR, avec résolution humaine
et rejet persistant des propositions.


### Diagnostic OCR expérimental v1.7.1

Le [cœur CPU et CLI de comparaison](services/ocr-experimental/README.md) permet de tester Tesseract arabe et les candidats sur une copie SQLite en lecture seule, bornée par bibliothèque. Il n’active pas le nouveau moteur dans le worker Go. La qualité sur les couvertures réelles reste à mesurer avant activation.

Le [service FastAPI OCR interne](services/ocr-experimental/README.md#service-fastapi-interne-us-1713) partage ce cœur CPU et fournit un contrat multipart borné, des sondes de santé et un conteneur sans accès au catalogue. Son [adaptateur Go US-1714](docs/OCR_GO_ADAPTER_V1_7_1.md) est disponible derrière un flag désactivé par défaut, avec persistance transactionnelle et reprise bornée.

Le [matching de qualité US-1715](docs/OCR_MATCHING_QUALITY_V1_7_1.md) utilise le même flag expérimental, off par défaut : tokens utiles de tout le texte, corrections non ambiguës et classement FTS5/BM25 isolé par bibliothèque. Le gain sur images réelles reste à valider lors de la recette.

L’[US-1716](docs/OCR_ACCEPTANCE_V1_7_1.md) est clôturée sur décision du propriétaire pour la livraison du comparateur Go, du gate qualité/mémoire et du protocole de rollback. L’OCR arabe reste expérimental, avec flag désactivé par défaut. La recette sur couvertures réelles est suivie séparément avant activation générale ; la clôture ne valide ni la qualité arabe ni les seuils proposés.

US-1717 est clôturée après revue de la [recherche manuelle SAFE et de sa recette automatisée](docs/MANUAL_COVER_MATCHING.md) : trois suggestions maximum, aperçu privé, rejet persistant et remplacement confirmé. L’ancienne couverture reste disponible si le traitement de la nouvelle échoue ; cette recherche ne dépend pas du flag OCR expérimental.
