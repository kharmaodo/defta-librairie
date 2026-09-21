# Delivery final — Defta Librairie

Ce document clôt le périmètre des douze priorités décrit dans `BACKLOG.md`.
Il distingue la validation du logiciel des décisions propres à chaque
environnement de production.

## Contrôle automatisé avant release

Prérequis : Go correspondant à `go.mod`, compilateur C et SQLite FTS5,
Node.js 20 ou plus, npm, Python 3 et Chromium installé par Playwright.

```sh
npx playwright install --with-deps chromium
./scripts/check-delivery.sh
```

La commande vérifie successivement le patch, le contrat OpenAPI,
l’accessibilité structurelle, le contrat du dashboard, la restauration SQLite,
les tests Go normaux et avec détecteur de courses, les tests frontend et la suite Chromium complète.
Elle s’arrête dès le premier échec et ne touche pas à la base applicative.

## Matrice de couverture livrée

| Domaine | Preuve principale |
|---|---|
| Authentification et autorisations | Tests Go et `tests/browser/auth.spec.cjs` |
| Catalogue, stock et vente | Tests services/HTTP et `commercial-lifecycle.spec.cjs` |
| Approvisionnement et CMP | Tests services et `procurement-lifecycle.spec.cjs` |
| Clients | Tests services/handlers et historique paginé |
| Paiements et caisse | Tests services et `payment-lifecycle.spec.cjs` |
| Retours clients | Tests services et `customer-return-lifecycle.spec.cjs` |
| Retours fournisseurs | Tests services et `supplier-return-lifecycle.spec.cjs` |
| Statistiques et alertes | Tests repositories/services/handlers et modules frontend |
| Exports et impressions | Tests Go et `exports-printing.spec.cjs` |
| Accessibilité | Contrôle structurel et `accessibility.spec.cjs` |
| Dashboard v1.1 | `check-admin-dashboard.py` et `admin-dashboard.spec.cjs` |
| API et exploitation | Contrat OpenAPI, sondes, métriques et test de restauration |
| Couvertures v1.4 | Validation des sources, outbox, worker JetStream, cycle de vie, lecture privée et `book-covers.spec.cjs` |

## Checklist de déploiement

- Créer une sauvegarde SQLite cohérente et la conserver hors de la machine.
- Tester cette sauvegarde avec la procédure de `docs/SQLITE_RESTORE.md`.
- Fournir un `JWT_SECRET` d’au moins 32 octets via le gestionnaire de secrets.
- Vérifier les chemins persistants, droits du processus et espace disque.
- Configurer TLS, `AUTH_COOKIE_SECURE=true` et le proxy inverse en production.
- Initialiser le compte root sans conserver son mot de passe dans les journaux.
- Vérifier `/api/health/live`, `/api/health/ready` et la collecte des métriques.
- Effectuer un smoke test de connexion, vente, stock, export et impression.
- Conserver l’artefact précédent et documenter le responsable du retour arrière.
- Si les couvertures sont activées : vérifier bucket privé, volume SQLite partagé avec le worker, persistance JetStream, sauvegarde MinIO et reprise de l’outbox.
- Contrôler l’upload, les états asynchrones, la lecture autorisée, le remplacement et le nettoyage après rétention.

## Décision de mise en production

Une release est candidate lorsque `check-delivery.sh` est entièrement vert sur
le commit à taguer et qu’une restauration récente a été réellement testée.
La construction, le tag annoté et le retour arrière sont détaillés dans
`docs/RELEASE.md`.
Les objectifs de reprise, la fréquence/rétention des sauvegardes, la rétention
des logs et les seuils d’alerting doivent être décidés par l’exploitant.

Les extensions métier déjà identifiées — consommation des avoirs, affichage du
solde net après retour et multidevise — ne font pas partie de cette release.
Elles doivent ouvrir un nouveau backlog plutôt que modifier silencieusement le
périmètre livré.
