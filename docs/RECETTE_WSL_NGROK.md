# Lancer la recette v1.7.2 sous Linux/WSL

Ce guide reprend la recette existante sur une copie SQLite et des services
MinIO/NATS séparés. Il ne configure pas un VPS ni ne valide le transport en
production. Le tunnel rend cette recette accessible sur Internet : utiliser
exclusivement les données et services de test. Ngrok termine le TLS ; le trajet
entre ngrok et Go est HTTP local. Ce service tiers transporte les requêtes.

## Préparation

Installer Go compatible avec go.mod, Python 3, les prérequis CGO/FTS5 et ngrok
pour Linux dans WSL. Garder les services de recette démarrés. Le modérateur local
partagé, sans état métier, doit répondre sur `http://127.0.0.1:8090/health/ready`.

Le dossier existant doit contenir :

- `source/` : checkout du candidat avec templates/static, **sans fichier .env** ;
- `defta-restored.db` : copie restaurée, jamais la base active ;
- `services.env` : identifiants privés et ports du projet Docker de recette.

Pour la recette déjà créée :

```bash
export DEFTA_RECETTE_DIR=/home/dmaodo/defta-recette-v1.7.2-Y5HzoX
chmod 600 "$DEFTA_RECETTE_DIR/services.env"
```

Le fichier privé existant doit définir `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`,
`NATS_USER`, `NATS_PASSWORD`, `MINIO_BUCKET_COVERS`, `MINIO_API_BIND` et
`NATS_CLIENT_BIND`. Les trois clés de console/monitoring du fichier déjà créé
sont aussi acceptées. Les deux ports API/client doivent être ceux des services
isolés (dans cette recette : `127.0.0.1:19000` et `127.0.0.1:14222`). Le script
ne crée ni ne remplace ce fichier et n'affiche pas ses valeurs. Il ne vérifie
pas que ces services sont isolés : contrôler le projet avec Docker Compose.

Le script peut être exécuté depuis un autre checkout : son emplacement ne
change pas la source lancée. Après fusion, depuis `~/defta-librairie` à jour,
utiliser les commandes suivantes. Aucun checkout/reset de la source de recette
n'est nécessaire pour accéder au script ; sélectionner une autre source avec
`--source /chemin/checkout-sans-env` si souhaité.

## HTTP local

Arrêter uniquement le précédent processus Go de recette avec Ctrl+C, puis :

```bash
python3 scripts/run-recette.py --recette-dir "$DEFTA_RECETTE_DIR" --check
python3 scripts/run-recette.py --recette-dir "$DEFTA_RECETTE_DIR"
```

Ouvrir `http://localhost:8080/login`. Le relais Windows existant peut rester
utilisé pour ce mode. PUBLIC_ORIGIN est vide, cookies non Secure. Les variables
applicatives du shell sont ignorées ; aucun .env n'est chargé depuis la source.
La clé JWT est aléatoire à chaque lancement : les sessions précédentes doivent
être renouvelées par une nouvelle connexion. La date correspond au build local.

## HTTPS via ngrok

Dans **un deuxième terminal du même WSL que Go**, lancer :

```bash
ngrok http http://127.0.0.1:8080
```

Garder le Host public : ne pas utiliser `--host-header=rewrite`. Relever l'URL
HTTPS réellement affichée. Puis arrêter le serveur Go de recette et le relancer
avec cette origine exacte, sans slash final ni chemin :

```bash
python3 scripts/run-recette.py \
  --recette-dir "$DEFTA_RECETTE_DIR" \
  --public-origin https://legged-corner-spearhead.ngrok-free.dev --check
python3 scripts/run-recette.py \
  --recette-dir "$DEFTA_RECETTE_DIR" \
  --public-origin https://legged-corner-spearhead.ngrok-free.dev
```

L'adresse ci-dessus est un exemple de la session actuelle : la remplacer si
ngrok en affiche une autre. Go écoute alors seulement sur 127.0.0.1 et reconnaît
X-Forwarded-Proto HTTPS uniquement depuis le loopback. Ne pas passer par le
relais Windows ni viser l'IP WSL pour la connexion ngrok→Go dans ce mode.

Ouvrir l'URL HTTPS `/login` en navigation privée ; se connecter puis tester
refresh, navigation admin et logout. Les cookies de session sont Secure,
HttpOnly et SameSite ; le cookie CSRF reste lisible par le JS, avec Secure.
Le contrôle d'origine/CSRF reste actif. Les accès HTTP locaux sont redirigés
vers l'origine HTTPS configurée ; ne pas essayer de se connecter sur HTTP.

```bash
curl --fail --silent --show-error \
  https://legged-corner-spearhead.ngrok-free.dev/api/health/ready
curl -I https://legged-corner-spearhead.ngrok-free.dev/login
curl -i https://legged-corner-spearhead.ngrok-free.dev/api/admin/owners
```

Attendus : ready, CSP stricte/HSTS et cookie CSRF Secure sur login, 401 pour
l'API admin anonyme. Si ngrok affiche une page d'avertissement, la valider dans
le navigateur avant le parcours. Ces contrôles de tunnel ne remplacent pas la
validation HTTPS sur le futur VPS.

En cas de 403 login/refresh, inspecter le corps JSON : `csrf_failed` indique
un refus d'origine ou de jeton. Vérifier URL configurée, Host conservé et
nouvelle navigation privée. Une boucle 308 indique souvent un agent hors
loopback ou un X-Forwarded-Proto absent/incorrect. Ne pas désactiver CSRF,
ni transmettre cookies, JWT, mots de passe ou capture contenant ces secrets.

Pour revenir au HTTP local : arrêter ngrok et Go, relancer sans
`--public-origin`, puis ouvrir une nouvelle session navigateur HTTP.
