# Recette de modération des couvertures

Cette recette valide le parcours complet : aucune image jointe ne crée un livre
avant la décision du modèle local.

## Préparation

Activer les couvertures et renseigner MinIO, NATS et le modèle local dans
`.env` :

```dotenv
COVERS_ENABLED=true
NATS_SUBMISSION_STREAM=BOOK_SUBMISSIONS
NATS_SUBMISSION_SUBJECT=book.submissions.moderate.v1
NATS_SUBMISSION_CONSUMER=book-submission-worker-v1
NSFW_MODERATION_ENDPOINT=http://nsfw-moderator:8090
```

Monter le modèle ONNX et son manifeste dans le conteneur `nsfw-moderator`,
puis vérifier :

```bash
curl --fail-with-body http://localhost:8090/health/ready
go run -tags fts5 ./cmd/main.go
```

## Cas approuvé

1. Dans `/admin`, créer un livre et joindre un JPEG ou PNG.
2. Vérifier le message de soumission : le livre ne doit pas encore figurer dans
   `GET /api/manage/books` ni dans la recherche publique.
3. Attendre le worker, puis vérifier l'état et l'audit :

```sql
SELECT id, moderation_status, moderation_score, moderation_model_version, created_book_id
FROM book_submissions ORDER BY created_at DESC LIMIT 1;

SELECT status, active FROM book_covers
WHERE book_id = (SELECT created_book_id FROM book_submissions ORDER BY created_at DESC LIMIT 1);
```

4. Une fois la couverture `READY`, rechercher le livre publiquement : la
   route `/api/books/{id}/cover?variant=large&format=jpeg` doit répondre
   `200 image/jpeg`.

## Cas rejeté et revue humaine

Soumettre une image qui provoque respectivement `UNSAFE` et `REVIEW`.
Vérifier :

- `REJECTED` ou `REVIEW_REQUIRED` dans `book_submissions` ;
- `created_book_id IS NULL` ;
- aucune ligne `defta` ni `book_covers` créée ;
- une ligne d'audit `DECIDE_BOOK_SUBMISSION_MODERATION`.

## Indisponibilité

Arrêter temporairement le conteneur de modération. La soumission doit terminer
en `FAILED`, sans créer de livre. Rétablir le conteneur et soumettre une
nouvelle image. Les échecs de promotion MinIO sont repris périodiquement ; ils
ne relancent pas l'analyse et ne recréent pas le livre.

Ne pas utiliser cette classification automatisée comme détecteur spécialisé de
contenus illégaux : elle déclenche une politique de rejet ou de revue humaine.
