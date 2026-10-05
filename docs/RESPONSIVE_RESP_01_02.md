# RESP-01 et RESP-02 — socle et catalogue public

## Périmètre

Templates et CSS uniquement pour le comportement applicatif. Aucun changement
Go, API, authentification, données, migrations ou règles métier. Le catalogue
reste rendu serveur ; l'accueil sans recherche reste vide de livres. La
navigation partielle façon SPA sera traitée dans RESP-07.

## Diagnostic du code

| Écran | État observé dans le code | Suite |
|---|---|---|
| Accueil `/` | Viewport déjà présent ; grille à seuils 700/420 px ; hauteur de couvertures fixe ; titres sans espace non coupés ; recherche et pagination peu flexibles | RESP-02 : grille bornée, couvertures proportionnées, retour ligne des textes, recherche sur deux lignes sur petit mobile, pagination adaptable |
| Catalogue en tableau | Défilement horizontal interne déjà présent, colonnes sans retour ligne ; zone non focalisable au clavier | Texte enveloppé, région nommée et focalisable, défilement limité au tableau |
| `/login` | Viewport et feuille admin déjà présents | Socle commun de réduction des tailles intrinsèques ; parcours existants conservés |
| `/admin` | Menu adaptatif et tableaux défilants existants ; tests 390 à 1440 px | Socle commun et contrôle ajouté à 320 px ; refonte navigation/tableaux/dialogues dans RESP-03 à RESP-05 |
| Modération des couvertures dans admin | Sections et formulaires admin existants | Inventaire seulement ; adaptation métier dans RESP-06 |
| `/admin/cover-imports` | Interface React/Vite existante, accessible par cette route ; ce n'est pas `/admin-covers` | Inventaire seulement ; refonte dédiée RESP-06 |

Les causes sont issues de l'inspection du code. Aucun accès au navigateur WSL
utilisateur ni mesure du tunnel ngrok n'a été effectué ici. Ngrok transporte
la page ; les règles de mise en page sont celles des assets servis.

## Correctifs

- Socle CSS partagé : conteneurs structurels et contrôles peuvent rétrécir,
  images/médias bornés à leur conteneur. Aucun overflow global masqué.
- Cartes : largeur minimale bornée à l'espace disponible, contenu flexible et
  couvertures avec ratio, `object-fit: contain` conservé sans recadrage.
- Titres arabes/latins longs : retour ligne, ellipse et deux lignes conservés.
- Recherche tactile : bouton sur une ligne dédiée à 480 px et moins ; actions
  de suppression/changement de vue/pagination d'au moins 44 px de haut.
- En-tête, résultats et pagination peuvent revenir à la ligne ; focus visible.
- Tableau : retour ligne des métadonnées et région focalisable nommée.

## Vérification

```bash
npm ci
npx playwright install --with-deps chromium
npm run test:frontend
npx playwright test tests/browser/catalogue-responsive.spec.cjs tests/browser/responsive.spec.cjs
npm run test:browser
go test -tags fts5 ./...
go vet -tags fts5 ./...
```

Le nouveau test crée 31 livres via les API existantes sur la base jetable du
serveur de test. Il vérifie accueil, recherche, titres arabes très longs,
couvertures de secours, cartes/tableau, conservation du mode et pagination
réelle à 320/390/768/1024/1440 px, plus paysage 812×375. Le document et les
éléments principaux doivent rester dans le viewport. Les tests admin
existants ajoutent 320 px ; les suites de thèmes/accessibilité restent actives.

Recette manuelle sur les données de test : vérifier zoom 200 % (reflow),
clavier, portrait/paysage et appareils Safari/Firefox en plus de Chromium.
Contrôler les titres, auteurs, prix et images réels. Ces appareils et le zoom
ne sont pas certifiés par la seule matrice Playwright Chromium.

Après récupération de la branche, relancer Go depuis le checkout qui contient
les nouvelles feuilles/templates. Le lanceur de recette utilise par défaut
`RECETTE_DIR/source` : mettre cette source à jour sans écraser de travail, ou
utiliser `--source` avec un checkout isolé sans `.env`. Une mise à jour du
checkout principal seul ne change pas les assets de la source de recette.
