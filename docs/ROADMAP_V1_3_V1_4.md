# Feuille de route — v1.3.0 et v1.4.0

## Objectif

Les versions `v1.3.0` et `v1.4.0` poursuivent l’amélioration de l’interface
sans affaiblir les règles métier, l’isolation par librairie, l’accessibilité ni
les parcours Playwright existants.

- `v1.3.0` corrige la lisibilité des formulaires en thème clair et sécurise
  les suppressions définitives.
- `v1.4.0` remplace les URL externes de couverture par un upload JPEG/PNG
  traité par l’application et stocké dans un bucket MinIO privé.
- Le traitement d’image est confié dès `v1.4.0` à un worker Go asynchrone,
  alimenté par NATS JetStream et déployé séparément de l’API.

## Décisions validées

### Suppressions

La confirmation renforcée s’applique uniquement aux suppressions définitives.
Elle ne s’applique pas automatiquement aux transitions métier comme annuler,
désactiver, révoquer, finaliser, expédier ou réactiver.

L’utilisateur doit saisir exactement l’identifiant humain affiché :

- titre du livre ;
- nom du tag ou de la catégorie ;
- référence d’un brouillon supprimable ;
- autre nom ou référence stable identifié pendant l’inventaire.

La comparaison est sensible à la casse, sans suppression automatique des espaces.
Le bouton reste désactivé jusqu’à égalité stricte. La saisie et les erreurs sont
réinitialisées à chaque fermeture, et le dialogue annonce explicitement le
caractère irréversible de l’action.

### Couvertures

Le traitement retenu est un recadrage centré au ratio 2:3 et une sortie de
800 × 1200 pixels. Le bucket MinIO est privé. La base stocke une clé d’objet,
jamais les identifiants MinIO ni une URL signée temporaire.

La lecture passe par une route applicative contrôlée qui applique le périmètre
d’accès pertinent et des en-têtes de cache. Toutes les vues utilisent le même
helper de résolution afin de conserver une image par défaut identique.

## v1.3.0 — UI et actions destructrices

### Incrément 1 — contrat et inventaire

- Recenser toutes les commandes `DELETE` et leur sémantique réelle.
- Classer chaque action : suppression définitive, suppression logique ou
  transition métier.
- Protéger les identifiants DOM et les assertions Playwright existantes.
- Documenter le texte exact attendu pour chaque ressource supprimable.

### Incrément 2 — champs de formulaire

- Introduire `--input-border-color` dans les variables du thème clair et sombre.
- Employer la variable pour `input`, `select`, `textarea` et les contrôles
  équivalents dans tous les formulaires.
- Définir des états cohérents `:hover`, `:focus-visible`, `:disabled` et
  invalides, avec contraste vérifiable.
- Ajouter un contrôle structurel et un parcours Playwright couvrant les deux
  thèmes sans modifier artificiellement les tests métier.

### Incrément 3 — dialogue de suppression commun

- Ajouter un seul composant accessible réutilisable par les modules admin.
- Afficher le nom ou la référence attendue sans ambiguïté.
- Désactiver la confirmation jusqu’à égalité stricte.
- Restaurer le focus sur le déclencheur après annulation ou fermeture.
- Empêcher la double soumission et conserver l’affichage des erreurs HTTP.
- Migrer seulement les suppressions définitives recensées.

### Incrément 4 — stabilisation et release

- Tests unitaires JavaScript du composant et des cas casse/espaces/réouverture.
- Tests Playwright souris, clavier, focus, annulation et confirmation.
- Suite Go/FTS5, contrôles statiques et suite Playwright complète sans `skip`.
- Documentation des sélecteurs avant/après et préparation de `v1.3.0`.

## v1.4.0 — couvertures stockées dans MinIO

### Configuration

Variables obligatoires lorsque l’upload de couvertures est activé :

```dotenv
MINIO_ENDPOINT=minio.internal:9000
MINIO_ACCESS_KEY=xxxx
MINIO_SECRET_KEY=xxxx
MINIO_USE_SSL=true
MINIO_BUCKET_COVERS=book-covers
```

Les secrets ne sont ni journalisés ni renvoyés au navigateur. Le démarrage
valide la configuration et la disponibilité du bucket selon la politique
d’exploitation retenue.

### Modèle et compatibilité

- Ajouter une migration pour la clé d’objet de couverture.
- Définir explicitement le traitement des anciennes valeurs `coverUrl`.
- Maintenir une période de lecture compatible si nécessaire, sans accepter de
  nouvelles URL externes depuis le formulaire.
- Utiliser une clé `{book_id}/{uuid}.{ext}`.
- Garantir au maximum une couverture active par livre.

### Upload et validation

- Utiliser `multipart/form-data` et limiter le corps en amont à 5 Mio.
- Accepter uniquement JPEG et PNG.
- Vérifier les signatures JPEG `FF D8 FF` et PNG
  `89 50 4E 47 0D 0A 1A 0A`.
- Confirmer le format par décodage réel, rejeter les fichiers tronqués ou
  incohérents et imposer une limite de dimensions/pixels avant décodage complet.
- Ignorer le nom de fichier fourni pour construire la clé de stockage.
- Couvrir fichier vide, faux MIME, fausse extension, signature invalide, image
  corrompue, dépassement de taille et accès inter-librairie.

### Traitement asynchrone

L’API enregistre la source et une ligne de transactional outbox dans SQLite.
Le publisher transmet le travail à NATS JetStream. Un worker Go idempotent
effectue le recadrage centré, crée un master normalisé, redimensionne en
800 × 1200 et produit les variantes JPEG, WebP et miniatures.

Le message n’est acquitté qu’après stockage des variantes et passage de la
couverture à l’état `READY`. Les états `PENDING`, `PROCESSING`, `READY` et
`FAILED` sont observables. L’ancienne couverture reste active pendant un
remplacement. La source brute est temporaire, tandis que le master est conservé
pour permettre la régénération.

Le worker est distribué comme image OCI pour Linux AMD64, ARM64 et ARMv7. Toute
dépendance native, notamment l’encodeur WebP, doit être construite et testée pour
ces trois architectures.

### Remplacement et cohérence

MinIO et SQLite ne partagent pas de transaction atomique. Le remplacement suit
donc une compensation explicite :

1. traiter et téléverser le nouvel objet ;
2. enregistrer la nouvelle clé dans la transaction SQLite ;
3. supprimer l’ancien objet après validation de la transaction ;
4. supprimer le nouvel objet si la mise à jour SQLite échoue ;
5. journaliser et rendre récupérable l’échec de nettoyage de l’ancien objet.

La suppression d’un livre doit également définir et tester le nettoyage de sa
couverture, sans rendre la donnée métier incohérente si MinIO est indisponible.

### Lecture et fallback

- Servir la couverture par une route applicative sans exposer MinIO.
- Appliquer `Content-Type`, `ETag`, `Cache-Control`,
  `X-Content-Type-Options: nosniff` et une politique CSP compatible.
- Retourner ou afficher l’image par défaut centralisée lorsqu’aucune couverture
  n’est définie.
- Employer des miniatures adaptées aux listes si les mesures de performance le
  justifient.

### Séquence de livraison

1. contrat, menace, modèle d’états, outbox et infrastructure Docker ;
2. client MinIO, upload de source et configuration testable ;
3. publisher outbox, NATS JetStream et worker idempotent ;
4. validation complète, master, JPEG, WebP et miniatures ;
5. lecture, remplacement compensé, rétention et nettoyage ;
6. formulaire admin, progression, reprise et fallback commun ;
7. sécurité, observabilité, tests et release multiplateforme.

## Hors périmètre

- Bucket MinIO public ou identifiants MinIO dans le frontend.
- Formats SVG, GIF, WebP ou AVIF en entrée pour `v1.4.0`.
- Galerie de plusieurs images par livre.
- Éditeur manuel de zone de recadrage.
- Assouplissement d’une assertion existante pour faire passer la recette.

## Critères communs de recette

1. Aucun contrat HTTP existant n’est cassé sans migration documentée.
2. L’isolation par librairie est testée côté service et HTTP.
3. Les interactions sont accessibles au clavier et annoncent leurs erreurs.
4. Les tests Go avec FTS5 et la suite Playwright complète passent sans test ignoré.
5. `scripts/check-delivery.sh` couvre les nouveaux contrats.
6. `BACKLOG.md` est actualisé après chaque fusion confirmée dans `develop`.
