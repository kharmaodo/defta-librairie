# Journal des versions

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
