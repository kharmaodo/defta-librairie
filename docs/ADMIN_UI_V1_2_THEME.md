# Système visuel et thème — v1.2.0

## Comportement

Le dashboard utilise le thème du système lorsqu’aucun choix manuel n’a été
enregistré. Le bouton du header bascule ensuite explicitement entre les thèmes
clair et sombre. Ce choix est conservé sous la clé `defta.adminTheme`.

Le thème est appliqué par `admin-theme.js` avant le chargement de la feuille de
style afin de limiter le flash d’une palette incorrecte. L’attribut
`data-theme` de l’élément `html` constitue l’unique état visuel partagé.

## Accessibilité

- Le bouton natif expose son état avec `aria-pressed`.
- Son nom accessible annonce l’action disponible, et non seulement l’état actuel.
- Les couleurs de texte, surfaces, bordures, champs, succès et erreurs utilisent
  des variables dédiées dans les deux thèmes.
- Les contours `focus-visible` existants restent inchangés.
- Les transitions continuent de respecter `prefers-reduced-motion`.

## Contrat de persistance

| Situation | Résultat |
|---|---|
| Aucune préférence locale | Thème clair ou sombre selon le système |
| Préférence `dark` | Thème sombre et bouton pressé |
| Préférence `light` | Thème clair et bouton non pressé |
| Clic sur le bouton | Bascule, persistance immédiate et libellé synchronisé |
| Rechargement | Restauration du choix manuel |

Les tests `test-admin-theme.cjs` et `theme.spec.cjs` protègent respectivement
la structure du contrat et son comportement réel dans Chromium.
