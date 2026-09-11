# Tests navigateur — authentification

Prérequis Linux/WSL : Go correspondant à `go.mod`, compilateur C pour SQLite,
Node.js 20 ou plus et npm. La version de Playwright est fixée dans `package.json`
et les dépendances dans `package-lock.json`.

```sh
npm ci
npx playwright install --with-deps chromium
npm run test:browser
```

Le port 18080 doit être libre. Playwright refuse de réutiliser un serveur existant.
Le lanceur compile l’application avec FTS5, crée un répertoire temporaire et
une nouvelle base SQLite, applique les migrations via l’application, puis
initialise un root de test. Les variables DB/JWT sont imposées et l’application
s’exécute hors du dépôt, sans charger son `.env`. Les templates et fichiers
statiques sont liés depuis le dépôt. À l’arrêt normal, le serveur est terminé
et les fichiers temporaires sont supprimés. Un arrêt forcé peut laisser un
répertoire temporaire `defta-browser-*` dans le répertoire temporaire système.

Les identifiants présents dans les tests sont réservés à cette base jetable.
Aucun compte existant n’est utilisé. Les contextes navigateur sont distincts
pour chaque test ; un seul worker et aucun retry sont configurés.
Les traces, vidéos et captures sont désactivées pour ne pas enregistrer les
mots de passe et jetons. Les rapports locaux et dépendances sont ignorés par Git.

## Scénarios de cette première suite

- Connexion invalide et message d’erreur accessible.
- Connexion root, écran des propriétaires, déconnexion et cookie révoqué.
- Création d’un propriétaire par API sur la base temporaire, mot de passe
  obligatoire dans le navigateur, nouvelle connexion et refus d’une API root.
- Deux appels protégés avec un jeton d’accès volontairement invalide : un seul
  renouvellement par cookie et récupération du rôle attendu.

Le dernier scénario exerce le chemin 401 sans attendre une expiration réelle.
Les cookies, requêtes et serveur sont réels ; aucune réponse API n’est simulée.

## Portée et validation

Cette suite ne couvre pas encore les parcours commerciaux complets,
les téléchargements, l’impression ni une recette d’accessibilité exhaustive.
Ne déclarer cet incrément validé qu’après l’exécution réelle de Chromium.

```sh
npm run test:browser -- --list
node --test scripts/test-admin-*.cjs
python3 scripts/check-admin-accessibility.py
go test -tags fts5 ./...
```

Références officielles : [serveur géré par Playwright](https://playwright.dev/docs/test-webserver)
et [assertions avec attente](https://playwright.dev/docs/test-assertions).
