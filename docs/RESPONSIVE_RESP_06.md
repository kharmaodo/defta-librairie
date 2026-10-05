# RESP-06 — couvertures et imports responsive

L'interface React `/admin/cover-imports` garde le dépôt JPEG/PNG, les quotas, l'idempotence, les API, le suivi toutes les trois secondes, la portée des rôles et les décisions humaines existants. Aucun changement backend, données, migration ou `.env`.

Les en-têtes et compteurs se replient ; les propositions gardent leur miniature et disposent d'une colonne de texte sans largeur minimale implicite. Les titres/auteurs longs se replient, les fieldsets ne débordent plus et les boutons atteignent 44 px. Sous 900 px, l'image et les propositions passent sur une colonne. Sous 600 px, le dépôt et les panneaux occupent moins d'espace intérieur et les actions prennent la largeur disponible. L'aperçu conserve son ratio avec une hauteur bornée, la confirmation reste dans le viewport dynamique et défile si nécessaire. Tab/Maj+Tab restent dans sa confirmation native ; Échap et Annuler conservent le comportement existant, y compris la protection pendant une décision en cours.

Les fichiers générés de `static/cover-imports` sont reconstruits avec `npm ci --prefix frontend/cover-imports` puis `npm run build --prefix frontend/cover-imports`, sans changement de dépendances ni de requêtes supplémentaires.

## Tests et recette

Cinq scénarios Playwright ajoutés : 320×700, 390×844, 768×1024 et 812×375 couvrent les rôles propriétaire/root, le dépôt, le suivi, la recherche manuelle, le rejet d'une proposition, les titres longs arabe/latin, la confirmation de remplacement, son annulation et Échap, la boucle de focus et la validation finale. Un scénario root mobile couvre la revue de quarantaine et l'autorisation OCR. Ils interceptent les API pour isoler la mise en page ; les tests Go et les parcours existants conservent la couverture serveur.

À vérifier sur appareils réels : sélection de plusieurs images, clavier logiciel lors de la recherche, rotation, zoom 200 %, lecteur d'écran, Safari et Firefox. Les viewports Chromium ne certifient pas ces comportements. La zone de dépôt garde le sélecteur natif accessible au toucher ; aucune obligation de glisser-déposer.

Prochaine étape : RESP-07, navigation fluide et conservation de l'état avec les API existantes.
