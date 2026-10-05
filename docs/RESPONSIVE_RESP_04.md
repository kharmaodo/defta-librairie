# RESP-04 — tableaux et listes admin

## Choix de présentation

Les tableaux natifs sont conservés : titres de colonnes, ordre, valeurs et
boutons métier restent dans le DOM d'origine. Aucun appel API supplémentaire,
changement Go, rôle, règle métier, données, .env ni génération de cartes en
double. Cette approche permet de comparer les lignes et évite de modifier les
modules de ventes, paiements, stocks et retours.

| Groupes | Présentation retenue |
|---|---|
| Livres, inventaire, fournisseurs, clients, propriétaires, caisses | Tableau avec défilement interne et actions maintenues à droite sur mobile |
| Ventes, achats, paiements, retours clients/fournisseurs | Toutes les colonnes conservées ; actions accessibles, sans réduction des montants ou états |
| Catégories et éditeurs | Colonnes multilingues conservées et région déjà nommée respectée |
| Audit, sessions, alertes | Tableau comparatif avec région nommée, retour ligne des longues métadonnées et défilement clavier |
| Historiques, lignes et règlements dans les dialogues | Même socle de défilement ; aucune refonte du dialogue ni de ses formulaires |
| Tags | Liste flexible, noms longs pouvant revenir à la ligne et suppression tactile |
| Modération | Socle tableau seulement ; refonte de revue/imports dans RESP-06 |

## Comportement

Le module admin-tables.js identifie les régions table-wrap et lit les en-têtes
existants. Il ne recompose ni les lignes de données ni les actions. Les régions
sont nommées à partir du panneau/dialogue ; les noms existants sont préservés.
Une indication de défilement et un tabindex sont ajoutés quand le tableau
déborde ; les descriptions et tabindex préexistants sont restaurés sinon.

ResizeObserver et MutationObserver prennent en compte chargement, filtrage,
pagination, ouverture des dialogues et nouveaux tableaux. Les rafraîchissements
sont regroupés via requestAnimationFrame. Les observateurs des régions retirées
sont libérés. Les messages empty sont enveloppés sans modifier leur texte pour
rester lisibles lorsque la ligne s'étend sur toutes les colonnes.

À 900 px et moins, texte long et boutons peuvent revenir à la ligne. Les
cellules restent des cellules natives, y compris celles que les modules
existants marquent row-actions ou book-cover-title. Seules les colonnes dont le
dernier en-tête est Action/Actions sont sticky à droite ; les cellules colspan
et les colonnes de valeurs ne sont pas rendues sticky. Aucun bouton désactivé
n'est réactivé. Les styles utilisent les couleurs du thème ; l'indication et
le sticky sont désactivés pour l'impression.

Le budget JavaScript admin passe de 200 000 à 203 500 octets pour compter
explicitement le module (aucune exclusion du contrôle). Les budgets HTML et
CSS historiques restent inchangés. La feuille du composant est séparée.

## Vérification

```bash
npm ci
npx playwright install --with-deps chromium
npm run test:frontend
npx playwright test tests/browser/admin-tables-responsive.spec.cjs
npm run test:browser
go test -tags fts5 ./...
go vet -tags fts5 ./...
```

Quatre nouveaux scénarios utilisent un livre créé via les API existantes dans
la base jetable : 320×700, 390×844, 768×1024 et paysage 812×375. Ils vérifient
les six colonnes, le texte long arabe/latin, les trois actions visibles dans la
région, la hauteur tactile, le défilement par flèche, l'ouverture de Modifier,
l'état vide après filtrage et l'absence de débordement global. Les suites
métier existantes vérifient ventes, paiements, retours, exports et impressions.

Recette manuelle : parcourir les différents tableaux et tags au tactile et au
clavier ; filtrer/paginer ; ouvrir un historique ; vérifier les boutons actifs
et désactivés, les deux thèmes et les impressions. Safari/Firefox, zoom 200 %
et lecteur d'écran restent à vérifier sur leurs appareils. Lancer le checkout
qui contient ces assets avec la recette isolée, sans modifier le .env actif.
