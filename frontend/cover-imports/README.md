# Interface des imports de couvertures (US-1706)

Cette application React, TypeScript et Vite est une page de l'administration DEFTA-LIBRAIRIE. Elle utilise le JWT en mémoire de session, le renouvellement partagé et la redirection 401 de `/static/js/admin-http.js`. Le serveur Go la sert sur `/admin/cover-imports` ; le tableau de bord existant fournit le lien. Aucun deuxième écran de connexion n'est créé.

```sh
cd frontend/cover-imports
npm ci
npm run build
```

Le build écrit les fichiers servis par Go dans `static/cover-imports/` (fichiers versionnés pour les déploiements sans Node). Après une modification des sources, reconstruire et committer aussi les fichiers générés. Pour la recette du parcours :

```sh
go test -tags fts5 ./...
go vet -tags fts5 ./...
npm run test:browser -- --grep 'cover import UI'
```

Un propriétaire voit sa librairie ; le root sélectionne une librairie active. La liste interroge l'API toutes les trois secondes tant que la page est visible. Elle affiche les états individuels des images ; la source privée est téléchargée uniquement lorsqu'une revue est ouverte, puis son URL objet est révoquée. Les actions restent soumises aux contrôles serveur. L'envoi conserve sa clé d'idempotence lors d'un échec réseau et ne répète jamais automatiquement une modification.
