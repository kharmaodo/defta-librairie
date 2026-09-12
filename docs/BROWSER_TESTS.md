# Tests navigateur — authentification et parcours métier

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

## Scénarios couverts

- Connexion invalide et message d’erreur accessible.
- Connexion root, écran des propriétaires, déconnexion et cookie révoqué.
- Création d’un propriétaire par API sur la base temporaire, mot de passe
  obligatoire dans le navigateur, nouvelle connexion et refus d’une API root.
- Deux appels protégés avec un jeton d’accès volontairement invalide : un seul
  renouvellement par cookie et récupération du rôle attendu.
- Cycle commercial root sur une librairie de test : création d’un livre sans
  stock, entrée de sept unités, brouillon de vente de deux unités, confirmation
  avec stock à cinq, puis annulation avec stock restauré à sept.
- Approvisionnement root : fournisseur, deux achats réceptionnés respectivement
  à `4 × 1 000` et `6 × 2 000`, stock final de dix, puis vente à `3 000`.
  Les statistiques doivent exposer un coût connu de `1 600`, une marge de
  `1 400` et aucun coût inconnu, ce qui vérifie le coût moyen pondé.
- Paiements root : caisse active, vente confirmée de `6 000`, puis règlements
  de `1 000` en espèces, `2 000` en mobile money et `3 000` par carte. Le solde
  passe de non payé à partiellement payé, puis payé avec un reste nul.
- Retours clients root : deux exemplaires d’une vente encaissée sont retournés
  séparément, l’un remboursé en espèces et l’autre réglé par avoir. Chaque
  finalisation restitue le stock et produit les audits attendus.
- Retour fournisseur root : deux exemplaires issus d’un achat réceptionné de
  cinq unités sont expédiés. Le stock passe de cinq à trois, avec motif,
  valorisation figée et audit d’expédition.
- Exports et impressions root : téléchargement des cinq CSV avec contrôle de
  leur nom et de données propres à la librairie temporaire, puis ouverture et
  impression des reçus d’une vente et d’un achat.

Le dernier scénario exerce le chemin 401 sans attendre une expiration réelle.
Les cookies, requêtes et serveur sont réels ; aucune réponse API n’est simulée.

## Portée et validation

Le cycle commercial vérifie le DOM et les appels réels jusqu’à la restauration
du stock. L’approvisionnement vérifie ses réceptions et le CMP par l’effet
observable sur la marge d’une vente réelle. Les exports utilisent de vrais
téléchargements et l’impression vérifie les reçus préparés ainsi que l’appel du
navigateur. La suite ne couvre pas encore une recette d’accessibilité exhaustive. Ne déclarer ce
nouvel incrément validé qu’après l’exécution réelle de Chromium.

```sh
npm run test:browser -- --list
node --test scripts/test-admin-*.cjs
python3 scripts/check-admin-accessibility.py
go test -tags fts5 ./...
```

Références officielles : [serveur géré par Playwright](https://playwright.dev/docs/test-webserver)
et [assertions avec attente](https://playwright.dev/docs/test-assertions).
