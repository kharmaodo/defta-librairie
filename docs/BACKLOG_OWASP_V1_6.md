# Backlog v1.6.0 — sécurité OWASP et résilience sous charge

## Objectif

Faire de la sécurité une capacité vérifiable de Defta Librairie : prouver, sur un
environnement isolé, que les contrôles d’authentification, d’autorisation, de
consommation de ressources et d’intégrité métier restent effectifs sous charge
et sous requêtes concurrentes.

Cette version ne prétend pas certifier une conformité OWASP complète. Elle
établit un socle de tests reproductibles, de seuils mesurés et de contrôles
correctifs pour les risques pertinents de l’application.

## Règles de sécurité des essais

- Exécuter les tests de charge uniquement contre un environnement dédié,
  avec base SQLite, MinIO et NATS de test.
- Ne jamais viser la production ni utiliser des comptes, données, secrets ou
  buckets de production.
- Fixer les seuils après une mesure de référence ; les valeurs initiales ne
  doivent pas être arbitraires.
- Conserver les journaux, métriques et résultats anonymisés comme preuves de
  recette.
- Un échec de test de sécurité ou d’intégrité métier bloque la release.

## Références

- [OWASP API Security Top 10 — 2023](https://owasp.org/API-Security/editions/2023/en/0x11-t10/)
- [API1: Broken Object Level Authorization](https://owasp.org/API-Security/editions/2023/en/0xa1-broken-object-level-authorization/)
- [API4: Unrestricted Resource Consumption](https://owasp.org/API-Security/editions/2023/en/0xa4-unrestricted-resource-consumption/)
- [OWASP Web Security Testing Guide](https://owasp.org/www-project-web-security-testing-guide/latest/)
- [OWASP Denial of Service Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Denial_of_Service_Cheat_Sheet.html)

## Incréments et user stories

| Ordre | US | Livrable vérifiable | Risques principaux | État |
|---|---|---|---|---|
| 1 | US-1 — Baseline de sécurité | Cartographie des routes, rôles, objets sensibles et flux métier ; matrice risque → contrôle → test | API1, API3, API5, API8, API9 | Réalisé — `docs/OWASP_SECURITY_BASELINE_V1_6.md` |
| 2 | US-2 — Authentification sous charge | Rafales de connexions erronées, anti-énumération, verrouillage/throttling, renouvellement et révocation de session testés | API2, consommation abusive | Réalisé — tests concurrents, scénario k6 isolé et procédure de recette |
| 3 | US-3 — Autorisation concurrente | Matrice root/propriétaire et librairie A/B exécutée en parallèle ; toute lecture ou mutation hors périmètre répond 403/404 sans fuite | API1, API3, API5 | Réalisé — 48 accès concurrents hors périmètre refusés sans paiement créé |
| 4 | US-4 — Limites de ressources | Limites et comportement mesurés pour upload de couverture, recherche FTS, pagination, exports et endpoints coûteux ; réponses d’erreur cohérentes | API4, API6 | À développer |
| 5 | US-5 — Intégrité métier concurrente | Ventes, paiements, retours, réceptions et couvertures soumis à concurrence ; absence de double mouvement, double paiement ou transition impossible | API6, logique métier | À développer |
| 6 | US-6 — Observabilité et gate de release | Rapports de charge, métriques, logs corrélés, seuils p95/p99/taux d’erreur et contrôle automatisé de release | API4, API8 | À développer |

## Preuve de l’incrément 1

La matrice de référence est définie dans `docs/OWASP_SECURITY_BASELINE_V1_6.md` : actifs, frontières, rôles, familles de routes, invariants, jeux de données et mesures à relever. L’US-2 reprend cette baseline pour les essais d’authentification sous charge.

## Critères d’acceptation communs

- Chaque scénario identifie le rôle, la librairie, les données créées, le débit,
  la durée et le résultat attendu.
- Les scénarios destructifs utilisent des données jetables et une base recréée
  pour chaque exécution.
- Les tests concurrentiels vérifient aussi directement les invariants SQLite :
  stock, montants, version de ressource, audit et absence de doublon.
- Les endpoints protégés ne divulguent ni contenu d’un objet hors périmètre,
  ni propriété sensible non autorisée.
- Les refus de limite sont explicites et exploitables : code HTTP adapté,
  message générique non sensible, événement de log corrélé.
- Le rapport distingue une indisponibilité de l’environnement de test d’un
  défaut applicatif.

## Découpage technique proposé

- Tests déterministes Go pour les invariants, l’autorisation et les courses
  ciblées.
- Scénarios de charge HTTP versionnés (outil à choisir lors de l’US-1) pour les
  profils authentifié, propriétaire et root.
- Jeux de données et services Docker dédiés à la recette de sécurité.
- Commande unique de validation à intégrer à la delivery après stabilisation,
  sans alourdir les tests unitaires ordinaires.

## Hors périmètre initial

- Test DDoS Internet, scan agressif d’infrastructure externe et tests sur
  production.
- Certification formelle ou audit de conformité par un tiers.
- Changement non justifié des règles métier existantes.
- Multidevise et consommation des avoirs : ces évolutions métier restent à
  planifier séparément.

## Mise à jour de suivi

Après chaque incrément validé et mergé dans `develop`, mettre à jour la ligne
correspondante avec la PR, le commit de référence, les mesures obtenues et les
seuils retenus.