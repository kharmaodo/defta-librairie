# Refonte du dashboard d’administration — v1.1.0

## Objectif et limites

La version `v1.1.0` modernise uniquement la présentation de `admin.html` : header,
navigation thématique, mise en page, responsive et cohérence visuelle. Le footer
conserve exactement son contenu. Les règles métier, API, autorisations et formats
de données restent inchangés.

La refonte est progressive afin que chaque incrément puisse être testé et fusionné
indépendamment. Les contenus restent présents dans le DOM et initialisés comme en
`v1.0.0` ; la navigation conduit vers les sections sans transformer le dashboard
en onglets masquant des fonctions.

## Architecture de l’information cible

| Thématique | Rubriques | Comportement |
|---|---|---|
| Vue d’ensemble | Accueil, statistiques, alertes, exports | Groupe dépliable ; liens vers les ancres de page |
| Catalogue | Livres, tags, stocks | Groupe dépliable |
| Commerce | Clients, ventes, caisses, encaissements | Groupe dépliable |
| Approvisionnement | Fournisseurs, bons d’achat, retours fournisseurs | Groupe dépliable |
| Service après-vente | Retours clients | Lien direct |
| Configuration | Paramétrage de la librairie | Lien direct |
| Administration | Propriétaires, sessions, journal d’audit | Groupe dépliable ; rubriques filtrées selon le rôle existant |

Sur écran large, la navigation prend la forme d’une barre latérale. Sur petit
écran, elle devient un panneau refermable. Les groupes utilisent des boutons
natifs exposant `aria-expanded` et `aria-controls`. La touche `Échap` ferme le
panneau mobile ou le sous-menu actif et restitue le focus au déclencheur.

## Contrat DOM protégé

Les identifiants et attributs déjà utilisés par les modules JavaScript et les
tests Playwright sont conservés. La nouvelle structure les enveloppe sans les
renommer. Cela inclut notamment :

- le contenu principal `#dashboard-main`, le rôle `#role-badge` et la déconnexion
  `#logout-button` ;
- les formulaires et tableaux des livres, stocks, clients, ventes, achats,
  paiements, retours clients et retours fournisseurs ;
- les panneaux `#library-settings-panel`, `#csv-exports-panel`,
  `#business-alerts-panel`, `#commercial-statistics-panel`, `#owners-section` et
  `#supplier-returns-panel` ;
- les identifiants des vingt dialogues et leurs relations d’étiquetage.

Les nouvelles sections reçoivent des identifiants stables dédiés à la navigation.
Les nouveaux contrôles utilisent des attributs `data-dashboard-*`, séparés des
sélecteurs métier existants.

## Journal des changements de sélecteurs

| Élément | Avant | Après | Impact tests |
|---|---|---|---|
| Sélecteurs métier existants | Identifiants actuels | Inchangés | Aucun remplacement prévu |
| Navigation thématique | Absente | `[data-dashboard-nav]` | Nouvelle couverture |
| Bouton du menu mobile | Absent | `[data-dashboard-menu-toggle]` | Nouvelle couverture |
| Groupes de navigation | Absents | `[data-dashboard-nav-group]` | Nouvelle couverture |
| Liens de rubrique | Absents | `[data-dashboard-nav-link]` | Nouvelle couverture |
| Ancres des panneaux sans identifiant | Absentes | `#suppliers-panel`, `#purchases-panel`, `#customers-panel`, `#cash-registers-panel`, `#payments-panel`, `#sales-panel`, `#customer-returns-panel`, `#inventory-panel`, `#tags-panel`, `#books-panel`, `#audit-panel`, `#sessions-panel` | Ajout sans remplacement des sélecteurs métier |

Ce tableau doit être mis à jour dans le même commit si un sélecteur existant doit
exceptionnellement changer. Une adaptation de test ne peut ni supprimer une
assertion ni réduire la couverture fonctionnelle.

## Organisation attendue de `admin.css`

La feuille reste non minifiée, indentée et découpée par commentaires dans cet
ordre : variables, reset et accessibilité, layout général, header, navigation,
sous-menus, contenu, panneaux et métriques, formulaires, tableaux, dialogues,
footer, responsive et impression. Les couleurs, espacements, rayons, ombres et
transitions partagent des variables CSS afin de limiter les valeurs isolées.

Les états `hover`, `focus-visible`, actif, désactivé, erreur et succès doivent
rester perceptibles. Les animations respectent `prefers-reduced-motion`.

## Critères d’acceptation Playwright

- La suite existante passe intégralement, sans `skip`, sans suppression
  d’assertion et sans sélecteur rendu volontairement moins précis.
- Chaque sous-menu s’ouvre et se ferme à la souris et au clavier ; son
  `aria-expanded` reflète l’état réel.
- Tabulation, `Entrée`, `Espace` et `Échap` sont couvertes pour les contrôles
  introduits.
- Un lien de navigation conduit à une section ciblée et y place un focus utile.
- Le menu mobile s’ouvre, se ferme et restitue le focus au bouton déclencheur.
- Header, contenu principal et footer gardent leurs repères accessibles ; le
  contenu textuel du footer reste inchangé.
- La suite complète est exécutée avec `npm run test:browser`, en plus des contrôles
  statiques et de `go test -tags fts5 ./...`.

## Séquence de livraison

1. Ajouter les ancres sémantiques et la navigation sans modifier les modules métier.
2. Déployer le système visuel commenté et vérifier les principaux breakpoints.
3. Ajouter le comportement minimal de navigation dans un module JavaScript isolé.
4. Compléter Playwright pour les interactions nouvelles et rejouer tous les parcours.
5. Documenter les éventuels changements de sélecteurs, vérifier la release et
   mettre à jour `BACKLOG.md` après chaque fusion.
