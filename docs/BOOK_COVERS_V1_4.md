# Architecture des couvertures de livre — v1.4.0

## Décision

La version `v1.4.0` remplace la saisie d’une URL externe par un upload JPEG ou
PNG traité de façon asynchrone. L’API Go existante reste propriétaire du contrat
HTTP, de l’autorisation et des données métier. Un worker Go séparé produit les
variantes. MinIO conserve les objets et NATS JetStream transporte les travaux
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
- master conservé : `masters/{library_id}/{book_id}/{cover_id}.jpg` ;
- grande JPEG : `variants/{library_id}/{book_id}/{cover_id}/large.jpg` ;
- grande WebP : `variants/{library_id}/{book_id}/{cover_id}/large.webp` ;
- miniature JPEG : `variants/{library_id}/{book_id}/{cover_id}/thumb.jpg` ;
- miniature WebP : `variants/{library_id}/{book_id}/{cover_id}/thumb.webp`.

Le master normalisé 2:3 est conservé pour régénérer les formats futurs. La source
brute n’est supprimée qu’après traitement réussi et délai de rétention. La
grande variante cible 800 × 1200 pixels. Les dimensions exactes de la miniature
seront figées avec les tests de traitement.

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

Le worker aura une image OCI multiarchitecture `linux/amd64`, `linux/arm64`
et `linux/arm/v7`. Une dépendance native d’encodage WebP devra être construite
pour les trois cibles ; aucun assouplissement silencieux du format n’est permis.
Les limites CPU et mémoire seront documentées et testées sur Raspberry Pi.

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

## Critères de validation de la fondation

- la topologie et les responsabilités sont documentées ;
- le double-écriture SQLite/NATS est protégée par l’outbox ;
- les remplacements conservent l’ancienne couverture tant que la nouvelle
  n’est pas `READY` ;
- le master est conservé et la source brute possède une rétention explicite ;
- le compose n’emploie aucun tag `latest` et ne contient aucun secret réel ;
- MinIO, JetStream et leurs volumes sont configurés ;
- la matrice AMD64/ARM64/ARMv7 du worker est exigée ;
- le contrôle statique est intégré à la recette globale.
