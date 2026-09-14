# Feuille de route du dashboard — v1.2.0

## Objectif

La version `v1.2.0` rend le dashboard plus synthétique, plus rapide et plus
agréable sur mobile comme sur écran large. Elle prolonge le système visuel de
`v1.1.0` sans réécrire l’interface ni modifier silencieusement le métier.

## Principes de livraison

- Conserver les routes HTTP, formats de données, autorisations et règles métier.
- Conserver les identifiants DOM et attributs utilisés par les modules et tests.
- Livrer un seul incrément vérifiable par branche et mettre à jour `BACKLOG.md`
  après chaque fusion confirmée dans `origin/develop`.
- Mesurer l’état initial avant toute optimisation de performance.
- Maintenir l’usage au clavier, les libellés accessibles, le contraste et
  `prefers-reduced-motion`.
- Exécuter toute la suite Playwright sans test ignoré avant chaque release.

## Périmètre proposé

### Synthèse métier

La zone d’accueil présentera un nombre limité d’indicateurs utiles à l’action :
activité commerciale, niveau de stock et alertes prioritaires. Chaque indicateur
devra afficher sa période, son unité, son état de chargement et un accès à la
rubrique détaillée. Le choix final des indicateurs sera arrêté pendant le cadrage
à partir des données déjà exposées par l’API.

### Responsive et navigation

Les largeurs de référence couvriront au minimum mobile, tablette et bureau. Les
tableaux conserveront leurs informations essentielles et pourront employer un
défilement explicite ou une présentation compacte. Aucun contenu métier ne sera
supprimé uniquement pour faire tenir une vue étroite.

### Thème sombre et composants

Le thème sombre s’appuiera sur les variables CSS existantes et respectera la
préférence du système. Si un choix manuel est ajouté, il sera persistant,
accessible au clavier et couvert par Playwright. Boutons, badges, cartes,
formulaires, tableaux, alertes et dialogues partageront des états cohérents.

### Performance frontend

Une mesure de référence précédera les changements. Les optimisations viseront le
poids transféré, le temps de chargement initial et le travail JavaScript au
démarrage. L’initialisation différée ne devra ni masquer une erreur ni créer de
course entre les formulaires, filtres et rafraîchissements existants.

## Hors périmètre initial

- Nouvelle règle métier ou nouveau moyen de paiement.
- Refonte des API ou migration vers un framework frontend.
- Multidevise et conversion de l’historique.
- Consommation d’un avoir sur une nouvelle vente.
- Suppression d’une assertion ou assouplissement artificiel d’un test existant.

Ces sujets nécessitent un cadrage métier séparé avant d’entrer dans la version.

## Critères de recette

1. Les contrôles Go avec FTS5, statiques et Playwright passent intégralement.
2. Aucun test n’est marqué `skip` pour contourner une régression.
3. Les parcours sont couverts aux largeurs mobile, tablette et bureau retenues.
4. Les interactions nouvelles fonctionnent à la souris et au clavier.
5. Les thèmes conservent un contraste lisible et des états de focus visibles.
6. Les mesures avant/après accompagnent toute revendication de performance.
7. Les changements de sélecteurs sont consignés avec leur impact sur les tests.

## Séquence

1. Photographier l’état fonctionnel et les mesures de référence.
2. Valider les indicateurs et leur hiérarchie avant leur mise en forme.
3. Livrer les améliorations responsive séparément du thème.
4. Optimiser seulement les coûts observés et rejouer les parcours métier.
5. Stabiliser, documenter et produire les trois archives multiplateformes.
