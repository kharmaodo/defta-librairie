# Journal des versions

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
