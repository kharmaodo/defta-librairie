# Contrat technique — modération locale des couvertures

## Décision

Les couvertures sont contrôlées par un modèle NSFW local, exécuté dans un conteneur séparé. Aucune image n'est envoyée à un prestataire externe.

Un livre n'est créé dans `defta` qu'après une décision `APPROVED`. Une image refusée, ambiguë ou non analysable ne crée donc aucun livre visible ni aucune couverture active.

## Périmètre

- Validation de format, taille et dimensions avant stockage.
- Stockage de la source dans un bucket MinIO privé de quarantaine.
- Décision asynchrone par worker.
- Revue manuelle pour les résultats ambigus.
- Création atomique du livre approuvé, puis envoi au pipeline existant de variantes de couverture.
- Audit et métriques de la décision.

Hors périmètre initial : analyse de texte, reconnaissance faciale, entraînement du modèle avec les données métier et exposition de la source refusée.

## Architecture

```text
Dashboard
  -> API de soumission
  -> MinIO /quarantine (privé)
  -> SQLite outbox
  -> worker de modération
  -> conteneur modèle local
  -> APPROVED : création du livre + pipeline existant
  -> REJECTED / REVIEW_REQUIRED : aucun livre créé
```

Le conteneur de modèle ne reçoit que l'octet de l'image et retourne une décision structurée : classe, score, version du modèle et durée d'inférence. Il ne possède ni accès à SQLite, ni accès à MinIO, ni identifiants applicatifs.

## États

| État | Signification | Création du livre |
|---|---|---|
| `PENDING_SCAN` | Source stockée, tâche à publier ou à traiter | Non |
| `SCANNING` | Worker en cours d'analyse | Non |
| `APPROVED` | Image conforme à la politique | Oui |
| `REJECTED` | Violation confirmée de la politique | Non |
| `REVIEW_REQUIRED` | Score ambigu ou analyse indécidable | Non, en attente d'une décision humaine |
| `FAILED` | Erreur technique ou délai dépassé | Non, réessai possible |

Une soumission expire après une durée de rétention définie ; sa source de quarantaine est alors supprimée.

## Politique de décision

Les seuils ne sont pas figés dans le code : ils sont versionnés dans la configuration et calibrés avec un jeu de tests représentatif.

- score sous le seuil d'acceptation : `APPROVED` ;
- score au-dessus du seuil de refus : `REJECTED` ;
- intervalle entre les deux : `REVIEW_REQUIRED` ;
- indisponibilité du modèle, délai dépassé ou réponse invalide : `FAILED` et aucune création de livre.

Le système applique une politique de blocage par défaut : une absence de décision positive n'autorise jamais la création.

## Données et sécurité

Une table `book_submissions` contient les champs du futur livre, le statut, le hash SHA-256 de la source, la référence de quarantaine, la version du modèle, le score, la décision et les dates. Elle ne rend aucune soumission publique.

La table d'outbox existante ou une outbox dédiée assure la publication fiable de `book.cover.moderate.v1`.

- Bucket de quarantaine distinct du bucket de variantes.
- Accès MinIO limité au worker concerné.
- URL de lecture de quarantaine jamais exposée au navigateur.
- La source de quarantaine n’est exposée par aucune route HTTP. Sa purge différée et rejouable après rejet ou expiration reste l’US de clôture explicitement suivie dans `FEATURE_NSFW_COVER_MODERATION.md`.
- Journal d'audit sans image ni contenu sensible : identifiant, acteur, décision, version du modèle, motif générique et horodatage.
- Limites de taille, décodage réel de l'image et plafond de pixels conservés avant l'inférence.
- Idempotence par identifiant de soumission et hash de source.

## API et expérience d'administration

La création avec image devient une soumission multipartite dédiée. La réponse contient un identifiant de soumission et l'état `PENDING_SCAN`, jamais un livre tant que la modération n'est pas acceptée.

L'administration affiche l'état et un motif générique :

- `APPROVED` : « Création du livre finalisée. »
- `REJECTED` : « L'image ne respecte pas la politique de contenu. Le livre n'a pas été créé. »
- `REVIEW_REQUIRED` : « L'image nécessite une vérification avant création du livre. »
- `FAILED` : « La vérification n'a pas abouti. Réessayez ultérieurement. »

Un agent habilité peut approuver ou refuser une soumission en revue ; cette action est auditée. Les propriétaires ne voient que leurs propres soumissions.

## Observabilité

Mesurer séparément :

- volumes par décision ;
- durée de mise en file et d'inférence ;
- taux d'échec, de réessai et de revue ;
- version du modèle en service ;
- volume d'objets de quarantaine et suppressions en retard.

Aucune métrique ne contient l'image, son titre ou une donnée sensible inutile.

## User stories proposées

1. **Soumettre un livre avec couverture** — l'administrateur envoie les métadonnées et l'image ; le livre reste absent du catalogue pendant l'analyse.
2. **Approuver automatiquement une image conforme** — le worker crée le livre et déclenche la génération des variantes.
3. **Refuser une image non conforme** — la soumission est auditée, l'image est supprimée et aucun livre n'est créé.
4. **Traiter un cas ambigu** — un agent habilité décide manuellement, avec traçabilité complète.
5. **Reprendre après indisponibilité** — les tâches échouées sont réessayées sans doublon et sans création non autorisée.
6. **Purger la quarantaine** — les sources expirées ou refusées sont supprimées de façon vérifiable.

## Critères d'acceptation de release

- Aucun livre n'est créé sans décision `APPROVED`.
- Une source refusée ou expirée n'est pas accessible par HTTP public ou authentifié.
- Une même tâche rejouée ne crée pas deux livres.
- Les seuils et la version du modèle sont visibles dans la configuration et les audits.
- Les tests couvrent acceptation, refus, ambiguïté, indisponibilité du modèle, reprise, isolation par librairie et purge.
- Le modèle tourne dans son propre conteneur sans privilège, avec réseau et permissions minimaux.
