# Journal des versions

## [1.4.0] — 2026-09-21

Gestion des couvertures de livre par upload JPEG/PNG dans le dashboard.

### Couvertures et traitement asynchrone

- Validation du fichier réel, de sa taille et de ses dimensions avant stockage dans un bucket MinIO privé.
- Écriture transactionnelle des métadonnées, de l’audit et d’une outbox SQLite ; publication fiable sur NATS JetStream.
- Worker Go produisant un master recadré au format 2:3 et des variantes JPEG, WebP et miniatures.
- Remplacement sans interruption de la couverture active, lecture authentifiée et nettoyage différé des sources et anciennes variantes.
- Statuts PENDING, PROCESSING, READY et FAILED visibles dans l’administration, relance conditionnelle et image par défaut commune au formulaire et à la liste.

### Livraison et qualité

- Déploiement local MinIO/NATS/worker via Docker Compose ; image de worker Linux AMD64 et supervision native Windows AMD64.
- Tests Go/FTS5, frontend, Playwright et parcours d’intégration optionnel avec MinIO et NATS.
- Tag `v1.4.0` publié sur le commit validé `a5025e3` ; archives Windows AMD64 et Linux AMD64 accompagnées de `BUILD-INFO.txt` et `SHA256SUMS` vérifiées.
- Image worker Linux AMD64 publiée sur GHCR avec SBOM et provenance.

## [1.3.0] — 2026-09-15

Amélioration de la lisibilité des formulaires et sécurisation des suppressions
définitives dans le dashboard d’administration, sans modification des contrats
HTTP ni des règles métier.

### Interface d’administration

- Bordures des champs visibles dans les thèmes clair et sombre grâce à des
  variables CSS partagées.
- États de survol, focus, désactivation et invalidité harmonisés dans les
  dialogues, filtres, recherches et formulaires.
- Dialogue de suppression commun, accessible et cohérent avec le dashboard.
- Confirmation par saisie exacte du titre du livre, du nom du tag ou de la
  référence du brouillon de vente.
- Comparaison sensible à la casse et aux espaces, remise à zéro après fermeture
  et restitution du focus au déclencheur.

### Sécurité et qualité

- Double soumission bloquée et erreurs serveur annoncées dans le dialogue.
- Désactivations, révocations et transitions métier conservées hors du flux de
  suppression définitive.
- Inventaire des actions destructrices et contrat de sélecteurs documentés.
- Tests unitaires JavaScript, contrôles statiques et parcours Playwright dédiés.
- Vingt-deux parcours Chromium sans retry, `skip` ni `fixme`.
- Archives Windows AMD64, Raspberry Pi OS ARM64 et ARMv7 préparées avec sommes
  SHA-256.

## [1.2.0] — 2026-09-15

Amélioration progressive de l’expérience du dashboard d’administration, sans
modification des contrats HTTP ni des règles métier.

### Expérience du dashboard

- Synthèse métier avec quatre indicateurs, période explicite et accès direct aux rubriques.
- États vide, chargement et erreur accessibles pour la vue d’ensemble.
- Ergonomie responsive validée à 390, 768, 1024 et 1440 pixels.
- Thème sombre suivant le système, avec choix manuel accessible et persistant.
- Respect de `prefers-reduced-motion` et maintien des repères de focus.
- Chargement initial mesuré et lectures simultanées identiques mutualisées.

### Qualité

- Réponses obsolètes de la liste des ventes ignorées lors des actualisations concurrentes.
- Matrice automatisée couvrant structure, mobile, thème, clavier, performance et métier.
- Vingt parcours Chromium sans retry, `skip` ni `fixme`.
- Contrôle de couverture navigateur intégré à la recette finale.
- Archives Windows AMD64, Raspberry Pi OS ARM64 et ARMv7 préparées avec sommes SHA-256.

## [1.1.0] — 2026-09-14

Refonte visuelle ciblée du tableau de bord d’administration, sans modification
des règles métier ni des contrats HTTP.

### Interface d’administration

- Header modernisé et hiérarchie visuelle adaptée à un dashboard.
- Navigation latérale organisée en sept thématiques et dix-neuf rubriques.
- Sous-menus accessibles au clavier avec état actif synchronisé au défilement.
- Navigation mobile hors-canvas avec fond obscurci et restitution du focus.
- Feuille `admin.css` non minifiée, commentée et structurée par composants.
- Footer conservé sans modification de contenu.

### Qualité

- Sélecteurs métier historiques conservés et nouvelles ancres documentées.
- Treize parcours Chromium, dont le contrat complet du dashboard desktop/mobile.
- Contrôle statique des groupes, liens, ancres, footer et sections CSS ajouté au delivery.
- Archives exécutables Windows AMD64, Raspberry Pi OS ARM64 et ARMv7 publiées
  avec leurs ressources d’exécution et sommes SHA-256.

## [1.0.0] — 2026-09-14

Première version fonctionnelle complète du périmètre défini dans `BACKLOG.md`.

### Fonctionnalités

- Authentification JWT, sessions révocables et rôles root/propriétaire cloisonnés.
- Gestion des librairies, propriétaires, livres, tags, clients et fournisseurs.
- Stock audité, alertes métier et valorisation au coût moyen pondéré.
- Achats, réceptions, ventes, paiements partiels et trois moyens de paiement.
- Retours clients, remboursements, avoirs et retours fournisseurs.
- Statistiques, cinq exports CSV et impressions paramétrées.
- OpenAPI, sondes, métriques, logs et restauration SQLite testée.
- Interface modulaire, erreurs harmonisées et onze parcours Chromium.

### Limites connues du périmètre

- Devise unique XOF, sans conversion de l’historique.
- Avoirs émis mais non imputables sur une nouvelle vente.
- Aucun prestataire externe de paiement intégré.
- Objectifs de reprise, rétentions et seuils d’exploitation à définir par environnement.
