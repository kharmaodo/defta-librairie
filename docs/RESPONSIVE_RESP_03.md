# RESP-03 — navigation admin adaptative

## Comportement

La navigation conserve ses cinq groupes, ses 22 liens et les ancres existantes.
Aucune modification Go, API, rôles, règles métier, données ou .env.

À 1100 px et moins, le menu utilise la hauteur du viewport (dvh avec repli vh)
indépendamment de l'en-tête. Un bouton de fermeture reste dans le panneau ; le
panneau défile en interne, y compris en paysage. Les liens ont une hauteur
minimale de 44 px. Le fond est couvert par le backdrop existant.

Pendant l'ouverture, le panneau est annoncé comme dialogue modal nommé. Les
éléments derrière sont inert ; Tab/Maj+Tab restent dans les contrôles visibles.
Échap, Fermer et le backdrop ferment le menu et rendent le focus au bouton
Menu. Les états inert antérieurs sont restaurés. Fermé sur mobile, le panneau
est lui-même inert et ses liens ne sont plus accessibles par tabulation.

La sélection d'une rubrique ferme le panneau avant de focaliser la section.
Le défilement respecte la préférence de réduction des animations. Le changement
de taille réinitialise l'ouverture et restaure l'accès au contenu. Sur desktop,
la barre latérale et la navigation clavier habituelles sont conservées.

Sur mobile/tablette l'en-tête n'est plus sticky : ses retours à la ligne ne
masquent ni les ancres ni les actions. À 480 px et moins les actions de compte
utilisent deux colonnes, avec rôle et déconnexion sur leur propre ligne. Les
boutons existants, leurs IDs et leurs événements restent inchangés.

La feuille `admin-navigation.css` contient les ajustements propres au composant,
chargés après admin.css. Aucun style inline ni assouplissement CSP n'est ajouté.

## Vérification

```bash
npm ci
npx playwright install --with-deps chromium
npm run test:frontend
npx playwright test tests/browser/admin-navigation-responsive.spec.cjs tests/browser/accessibility.spec.cjs tests/browser/responsive.spec.cjs tests/browser/experience-modes.spec.cjs
npm run test:browser
go test -tags fts5 ./...
go vet -tags fts5 ./...
```

Les quatre nouveaux scénarios couvrent 320×700, 390×844, 768×1024 et paysage
812×375 : actions de compte, focus modal cyclique, rubrique ciblée, fermeture,
Échap et transitions mobile→desktop→mobile. Les suites existantes vérifient
également navigation thématique, rôles, thèmes et parcours métier.

Recette manuelle : ouvrir le menu, dérouler les groupes, parcourir au tactile
et au clavier, atteindre une rubrique, changer l'orientation et vérifier les
boutons de compte. Vérifier également Safari/Firefox, zoom 200 % et annonce
au lecteur d'écran ; Chromium seul ne certifie pas ces appareils.

Lancer la recette depuis le checkout contenant ces assets (option --source du
lanceur si nécessaire), sans modifier ni charger le .env du dépôt actif.
Les tableaux, dialogues métier et imports OCR restent dans RESP-04 à RESP-06.
