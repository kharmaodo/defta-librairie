# Recette responsive du dashboard — v1.2.0

## Matrice de référence

| Largeur | Contexte | Navigation | Contenu |
|---|---|---|---|
| 390 px | Mobile | Panneau hors-canvas, fermeture par Échap | Une colonne, champs et actions pleine largeur |
| 768 px | Tablette portrait | Panneau hors-canvas | Une colonne et tableaux défilables |
| 1 024 px | Petit bureau | Panneau hors-canvas | Deux colonnes pour la synthèse |
| 1 440 px | Bureau | Sidebar visible | Quatre colonnes pour la synthèse |

## Garanties

- La page ne provoque pas de débordement horizontal global.
- Les tableaux débordants restent contenus dans `.table-wrap` et défilent
  horizontalement sans déplacer toute la page.
- Les formulaires, recherches et groupes d’actions passent en une colonne sur
  mobile ; leurs contrôles utilisent toute la largeur disponible.
- Les panneaux et conteneurs flexibles peuvent rétrécir grâce à `min-width: 0`.
- Le menu mobile conserve son état `aria-expanded`, se ferme avec `Échap` et
  restitue le focus à son bouton.
- La vue bureau conserve la sidebar et la grille de synthèse à quatre colonnes.

## Contrôle automatisé

`tests/browser/responsive.spec.cjs` exécute le même contrat aux quatre largeurs.
Il vérifie la largeur réelle du document, la limite droite de la synthèse, le
défilement des tableaux et le comportement de la navigation.

La suite complète reste obligatoire afin que ces adaptations ne masquent aucune
régression des parcours métier.
