# Scénario de charge — authentification v1.6

Ce scénario couvre l’US-2 du backlog OWASP. Il valide sous concurrence les
protections déjà présentes :

- verrouillage de compte après échecs répétés ;
- réponse générique pour utilisateur inconnu, compte désactivé ou verrouillé ;
- limite par adresse IP sur `POST /api/auth/login` et
  `POST /api/auth/refresh` ;
- réponse `429 Too Many Requests` avec `Retry-After`.

## Prérequis

- un environnement local ou de recette isolé ;
- une base SQLite de test et des données jetables ;
- [k6](https://grafana.com/docs/k6/latest/) installé ;
- aucune URL, donnée ou secret de production.

Le script refuse de démarrer sans le garde-fou
`AUTH_LOAD_ALLOW=isolated`.

## Exécution

```sh
AUTH_LOAD_ALLOW=isolated \
AUTH_LOAD_BASE_URL=http://127.0.0.1:8080 \
k6 run tests/load/auth-rate-limit.js
```

Paramètres facultatifs :

```sh
AUTH_LOAD_VUS=16 AUTH_LOAD_ITERATIONS=2
```

Le scénario utilise volontairement des identifiants inexistants. Les réponses
attendues sont `401` avant l’atteinte de la limite et `429` ensuite. Une
réponse réussie, une fuite de l’état du compte, une réponse inattendue ou
l’absence de `Retry-After` sur un refus de limite fait échouer le contrôle.

## Contrôles déterministes complémentaires

```sh
go test -race -tags fts5 ./internal/middleware ./internal/services ./internal/handlers ./cmd
```

Les tests Go couvrent le compteur atomique, la remise à zéro de fenêtre et
l’isolation des clients IP. Les tests de services vérifient le verrouillage et
la réponse non énumérante. Le scénario k6 complète ces contrôles avec des
requêtes HTTP concurrentes.

## Limites connues

Le limiteur est en mémoire et protège une instance. Un déploiement horizontal
devra utiliser un store partagé ou une limitation au proxy/API gateway, avec
une clé client et des métriques cohérentes. Cette décision est à traiter avant
tout déploiement multi-instance.

## Preuves à archiver

Conserver le résumé k6, les métriques applicatives, les logs corrélés et la
configuration non sensible de la campagne. Ne pas archiver de jeton, cookie,
mot de passe ni export de données de production.
