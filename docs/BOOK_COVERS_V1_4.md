# Architecture des couvertures de livre — v1.4.0

## Décision

La version `v1.4.0` remplace la saisie d’une URL externe par un upload JPEG ou
PNG traité de façon asynchrone. L’API Go existante reste propriétaire du contrat
HTTP, de l’autorisation et des données métier. Un superviseur worker Go distinct produit les variantes dans le processus applicatif ; son extraction dans un exécutable séparé reste compatible avec les mêmes contrats. MinIO conserve les objets et NATS JetStream transporte les travaux
persistants.

Cette séparation fait partie de `v1.4.0`. Elle n’autorise aucun accès direct du
navigateur à MinIO ou à NATS.

## Topologie

| Composant | Responsabilité |
|---|---|
| API Go | Limite du corps, validation initiale, autorisation, stockage de la source, écriture de la couverture et de l’outbox |
| SQLite | Métadonnées, état de traitement et transactional outbox |
| NATS JetStream | Livraison persistante, acquittement, rejeu et temporisation des travaux |
| Worker Go | Validation complète, recadrage, redimensionnement, encodage, publication des variantes et changement d’état |
| MinIO | Bucket privé contenant sources, masters et variantes |
| Initialiseur MinIO | Création idempotente du bucket et refus de l’accès anonyme |

## Flux cohérent

1. L’API refuse un corps supérieur à 5 Mio avant lecture complète.
2. Elle accepte uniquement JPEG et PNG, vérifie les magic bytes, décode la
   configuration de l’image et applique le plafond de pixels.
3. Elle stocke la source sous `sources/{library_id}/{book_id}/{cover_id}.{ext}`.
4. Une transaction SQLite crée la couverture `PENDING` et l’événement outbox.
5. Le publisher de l’API publie l’événement sur JetStream puis marque l’outbox
   comme publiée. Toute ligne non publiée reste récupérable après redémarrage.
6. Le worker acquitte uniquement après publication de toutes les variantes et
   passage atomique de la couverture à `READY`.
7. Un échec transitoire provoque une nouvelle livraison avec temporisation. Après
   le plafond de tentatives, l’état devient `FAILED` et reste relançable.
8. Lors d’un remplacement, l’ancienne couverture `READY` reste active jusqu’à
   ce que la nouvelle soit prête.
9. Le nettoyage de l’ancienne version est différé et idempotent.

MinIO et SQLite ne partageant pas de transaction, chaque étape externe possède
une compensation explicite et observable.

## Modèle de données prévu

### `book_covers`

- `id` UUID ;
- `book_id` et `library_id` ;
- `status` : `PENDING`, `PROCESSING`, `READY` ou `FAILED` ;
- `source_object_key`, `master_object_key` ;
- clés des variantes ;
- format, largeur, hauteur et taille validés ;
- `error_code`, sans données sensibles ;
- timestamps, version et contrainte garantissant une seule couverture active.

### `cover_processing_outbox`

- `event_id` UUID unique ;
- `cover_id`, `book_id`, `library_id` ;
- type et version du schéma de message ;
- charge utile JSON ;
- `attempts`, `available_at`, `published_at`, `last_error`.

La migration initiale ajoute ces tables sans supprimer `coverUrl`. L’ancienne
valeur reste lisible pendant une période de transition, mais le formulaire
n’accepte plus de nouvelle URL dès l’activation de l’upload. Une migration
ultérieure ne supprime la colonne historique qu’après inventaire des données.

## Contrat de message

Sujet : `book.covers.process.v1`.

```json
{
  "schemaVersion": 1,
  "eventId": "uuid",
  "coverId": "uuid",
  "bookId": "uuid",
  "libraryId": "uuid",
  "sourceObjectKey": "sources/library/book/cover.jpg",
  "attempt": 1
}
```

Le `eventId` déduplique la livraison et le `coverId` rend la génération
idempotente. Les clés de sortie sont déterministes ; une livraison répétée ne
crée pas de nouvel objet.

## Objets et variantes

- source temporaire : `sources/{library_id}/{book_id}/{cover_id}.{ext}` ;
- master conservé : `masters/{library_id}/{book_id}/{cover_id}/master.jpg` ;
- grande JPEG : `variants/{library_id}/{book_id}/{cover_id}/large.jpg` ;
- grande WebP : `variants/{library_id}/{book_id}/{cover_id}/large.webp` ;
- miniature JPEG : `variants/{library_id}/{book_id}/{cover_id}/thumb.jpg` ;
- miniature WebP : `variants/{library_id}/{book_id}/{cover_id}/thumb.webp`.

Le master normalisé 2:3 est conservé pour régénérer les formats futurs. La source
brute n’est supprimée qu’après traitement réussi et délai de rétention. Les dimensions livrées et contrôlées sont 1000 × 1500 pour le master,
800 × 1200 pour la grande variante et 160 × 240 pour la miniature.

## Sécurité

- bucket privé et refus explicite de l’accès anonyme ;
- secrets uniquement par variables d’environnement ou gestionnaire de secrets ;
- isolation obligatoire par `library_id` dans la base et les clés d’objet ;
- nom original, extension et MIME déclarés non fiables ;
- JPEG : préfixe `FF D8 FF` ;
- PNG : signature `89 50 4E 47 0D 0A 1A 0A` ;
- taille maximale 5 Mio et plafond de pixels avant décodage complet ;
- aucune URL signée persistée ;
- lecture via une route applicative avec `Content-Type`, `ETag`,
  `Cache-Control` et `X-Content-Type-Options: nosniff` ;
- journaux sans contenu d’image, secret ni URL présignée.

## Résilience et observabilité

Les métriques distinguent outbox en attente, âge de la plus ancienne ligne,
travaux reçus/réussis/échoués, durée de traitement et nettoyages en attente.
Les logs corrèlent `request_id`, `event_id`, `cover_id`, `book_id` et
`library_id`.

L’API demeure disponible si NATS ou le worker est arrêté : l’upload devient
`PENDING` et l’outbox sera republiée. Une indisponibilité MinIO empêche en
revanche l’acceptation d’un nouvel upload sans affecter les autres fonctions.

## Docker et exploitation

`deploy/docker-compose.covers.yml` fournit MinIO, l’initialiseur du bucket et
NATS JetStream. Les images sont épinglées, les données utilisent des volumes
séparés et les services possèdent des health checks. Les ports d’administration
ne doivent pas être publiés en production.

Le worker séparé possède une image OCI non-root construite pour `linux/amd64`.
Le workflow utilise buildx, publie sur GHCR lors d’un tag `v1.4.*` et valide
cette plateforme sur chaque pull request concernée. Sous Windows AMD64, le
worker est supervisé dans l’exécutable natif de l’application lorsque
`COVERS_ENABLED=true`. L’image est accompagnée d’un SBOM et d’une provenance.
L’encodage WebP est écrit entièrement en Go et ne dépend pas de `libwebp`.

## Variables

```dotenv
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=
MINIO_SECRET_KEY=
MINIO_USE_SSL=false
MINIO_BUCKET_COVERS=book-covers
MINIO_SOURCE_RETENTION_HOURS=24
NATS_URL=nats://nats:4222
NATS_USER=
NATS_PASSWORD=
NATS_COVERS_STREAM=BOOK_COVERS
NATS_COVERS_SUBJECT=book.covers.process.v1
NATS_COVERS_CONSUMER=cover-worker-v1
COVER_MAX_BYTES=5242880
COVER_MAX_PIXELS=24000000
```

## Contrat d’upload livré par l’incrément 2

La route protégée `POST /api/manage/books/{id}/cover` accepte une requête
`multipart/form-data` contenant exactement un fichier nommé `cover`. Elle
répond `202 Accepted` avec une couverture à l’état `PENDING` et un en-tête
`Location`. Le propriétaire ne peut déposer une couverture que sur un livre
de sa librairie ; le super-administrateur conserve son périmètre existant.

L’API limite le corps avant sa lecture complète, vérifie JPEG/PNG par signature
réelle, cohérence MIME, décodage et plafond de pixels. Une source valide est
écrite dans MinIO avant la transaction SQLite qui crée la couverture, la ligne
d’outbox et l’audit `UPLOAD_BOOK_COVER`. Si cette transaction échoue, l’objet
source est supprimé par compensation. Une indisponibilité ou une désactivation
du stockage renvoie `503` sans affecter les autres fonctions.

Activation locale :

```dotenv
COVERS_ENABLED=true
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
MINIO_USE_SSL=false
MINIO_BUCKET_COVERS=book-covers
COVER_MAX_BYTES=5242880
COVER_MAX_PIXELS=24000000
```

Exemple :

```bash
curl --fail-with-body -i \
  -H "Authorization: Bearer $TOKEN" \
  -F "cover=@./couverture.jpg;type=image/jpeg" \
  http://localhost:8080/api/manage/books/$BOOK_ID/cover
```

Les réponses de validation distinguent notamment corps trop volumineux
(`413`), image invalide (`422`) et fonctionnalité indisponible (`503`).
Le contrat complet et ses schémas restent définis dans `static/openapi.json`.

## Messagerie et worker livrés par l’incrément 3

L’API réserve atomiquement les lignes d’outbox avec un bail récupérable après
arrêt brutal. Elle publie par lots sur JetStream avec `Nats-Msg-Id` égal à
`eventId`, puis renseigne `published_at` uniquement après acquittement du
serveur. Les erreurs sont replanifiées avec un backoff exponentiel borné.

Le consumer pull `NATS_COVERS_CONSUMER` est durable et utilise un acquittement
explicite. Une erreur transitoire provoque `NakWithDelay`. Un message invalide
ou une erreur permanente est terminé. Après `COVER_WORKER_MAX_DELIVER`, le
worker marque la couverture `FAILED` avant de terminer le message.

Chaque traitement réserve également la couverture avec
`processing_by`/`processing_until`. Un worker concurrent est refusé, un bail
expiré est récupérable et une couverture déjà `READY` est acquittée sans être
retraitée. La finalisation active la nouvelle couverture et désactive l’ancienne
dans une même transaction SQLite. Le processeur de pixels reste l’objet de
l’incrément 4.

Les tests optionnels `NATS_INTEGRATION=1` et
`COVER_PIPELINE_INTEGRATION=1` exercent respectivement les reprises du consumer
et le parcours MinIO → outbox → JetStream.

## Traitement et variantes livrés par l’incrément 4

Le processeur lit la source privée depuis MinIO, applique un recadrage centré
au ratio 2:3 sans étirement, puis génère le master JPEG, les grandes variantes
JPEG/WebP et les miniatures JPEG/WebP. Les clés sont déterministes et isolées
par librairie, livre et couverture.

Toutes les images sont produites avant la première écriture. Si une écriture
MinIO échoue, les objets déjà publiés par cette tentative sont supprimés en
ordre inverse. La base ne passe à `READY` qu’après les cinq écritures réussies.

Le publisher et le worker possèdent des superviseurs indépendants. Une
indisponibilité NATS ne bloque pas le démarrage HTTP ; chaque superviseur se
reconnecte et respecte l’arrêt gracieux de l’application.

Le test optionnel `COVER_PIPELINE_INTEGRATION=1` contrôle désormais le flux
complet : upload, outbox, JetStream, consommation, génération, présence des cinq
objets MinIO et activation de la couverture.

L’exécutable `cmd/cover-worker` permet un déploiement séparé. Son image est
non-root, en lecture seule, sans capability et avec `no-new-privileges`. Le
profil Compose `worker` attend MinIO, l’initialisation du bucket et NATS avant
de démarrer. Les secrets restent injectés au runtime et ne sont jamais copiés
dans l’image.

## Lecture sécurisée livrée par l’incrément 5

La route authentifiée `GET /api/manage/books/{id}/cover` diffuse uniquement
la couverture active à l’état `READY`. Le propriétaire reste limité aux livres
de sa librairie et le navigateur n’obtient jamais de clé MinIO ni d’URL signée.

Les paramètres `variant=master|large|thumb` et `format=jpeg|webp` sélectionnent
une clé déterminée par la base. Le master accepte uniquement JPEG. Sans
paramètre, la représentation `large/jpeg` est utilisée. Les réponses définissent
`Cache-Control: private, max-age=300`, un `ETag` déterministe et
`X-Content-Type-Options: nosniff`. `If-None-Match` permet une réponse
`304 Not Modified`.

Une couverture absente, inactive, non prête ou hors périmètre n’expose aucun
objet et produit une réponse contrôlée.

Lorsqu’une nouvelle couverture devient `READY`, la même transaction SQLite
enregistre chaque objet de l’ancienne couverture dans
`cover_object_cleanup_jobs`, désactive l’ancienne version puis active la
nouvelle. Si la mise en file échoue, toute la transaction est annulée et
l’ancienne couverture reste active. Le nettoyage MinIO est ainsi différé sans
fenêtre d’indisponibilité ni risque d’oubli silencieux.

Les travaux de nettoyage sont réservés par bail afin que plusieurs exécuteurs
ne suppriment jamais simultanément le même objet. Une suppression MinIO réussie
est acquittée dans SQLite. Un échec est replanifié avec un backoff exponentiel
borné et un message assaini ; un bail expiré redevient récupérable après un
arrêt brutal.

La source de la couverture active est conservée pendant la durée définie par
`MINIO_SOURCE_RETENTION_HOURS`, soit 24 heures par défaut. Le superviseur de
nettoyage démarre et s’arrête avec l’application sans bloquer le serveur HTTP.

Une réconciliation initiale puis périodique recrée de manière idempotente les
travaux absents pour les sources expirées et pour les objets des couvertures
`READY` inactives. Elle ne programme jamais les variantes de la couverture
active. Les clés déterministes, la compensation des écritures partielles et
cette réconciliation empêchent les objets connus de la base de rester
orphelins. Cette garantie ne constitue pas un parcours aveugle de tout le
bucket MinIO : seuls les objets appartenant au modèle SQLite sont concernés.

Les journaux `cover_cleanup_reconciled` et
`cover_cleanup_reconcile_failed` rendent la réparation observable sans
divulguer les clés ou les données des images.

## Critères de validation de la fondation

- la topologie et les responsabilités sont documentées ;
- le double-écriture SQLite/NATS est protégée par l’outbox ;
- les remplacements conservent l’ancienne couverture tant que la nouvelle
  n’est pas `READY` ;
- le master est conservé et la source brute possède une rétention explicite ;
- le compose n’emploie aucun tag `latest` et ne contient aucun secret réel ;
- MinIO, JetStream et leurs volumes sont configurés ;
- l’image du worker Linux AMD64 et l’exécutable natif Windows AMD64 sont exigés ;
- le contrôle statique est intégré à la recette globale.
