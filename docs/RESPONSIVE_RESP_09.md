# RESP-09 — validation finale responsive

Les RESP-01 à RESP-08 sont fusionnés et validés par le développeur. RESP-09 ajoute une matrice de contrôle continue et corrige les contraintes CSS révélées avec le texte agrandi à 320 px, sans changement backend, API, données, auth, `.env` ou dépendances. Aucun résultat automatisé ne vaut certification de tous les appareils.

## Matrice automatisée

La configuration habituelle garde la suite métier complète sur Chromium. `playwright.responsive.config.cjs` reprend le serveur jetable, un worker et zéro retry, et sélectionne les parcours responsive sur Chromium, Firefox et WebKit. Le workflow `Responsive validation` exécute les trois moteurs séparément ; un échec n'annule pas les autres moteurs.

| Périmètre | Contrôles |
|---|---|
| Catalogue public | 320/390/768/1024/1440 px, paysage 812×375, recherche réelle, cartes/tableau, pagination |
| Navigation admin | 320/390/768 px et paysage, menu, inertie, focus, ancres, historique |
| Tableaux/dialogues | mêmes petits écrans, défilement contenu, actions de 44 px, champs et validation conservés |
| Imports | rôles propriétaire/root, dépôt, revue, recherche/rejet de propositions, confirmation et quarantaine |
| États | réseau lent, échec, soumissions répétées, nouvel essai volontaire, lectures coalescées |
| Mode adapté | thème sombre, clavier, réduction des animations |
| Complément RESP-09 | GET/pagination sans JavaScript, texte racine de 16 à 32 px sur 320/1440 px, grand écran 1920 px |

L'augmentation du texte est un contrôle CSS, pas une simulation exacte du zoom navigateur à 200 %. WebKit Playwright n'est pas Safari installé sur un iPhone ; les sélecteurs de fichiers et claviers mobiles restent ceux de la recette réelle.

Les mesures Firefox arrondissent les dimensions subpixel à 0,01 px et la position de défilement au pixel, tout en gardant les seuils fonctionnels. L'upload vérifie les octets, le nom et le type du fichier présent dans le `FormData` réellement donné à `fetch` sur les trois moteurs, ainsi que la requête multipart et sa clé d'idempotence. Chromium et Firefox vérifient aussi les octets interceptés ; WebKit ne les expose pas dans son inspecteur réseau ([limite Playwright #6479](https://github.com/microsoft/playwright/issues/6479)). Le serveur d'import est simulé pour cette matrice ; le parcours métier existant conserve ses contrôles d'intégration.

## Commandes Linux/WSL

Depuis le checkout de recette ou un checkout de test à jour, conserver le `.env` existant : le serveur de tests utilise sa base temporaire et ne le charge pas. `npm ci` utilise le lockfile existant.

```bash
npm ci
npx playwright install --with-deps chromium firefox webkit
npm run test:browser
npm run test:responsive
# Ou un seul moteur :
npm run test:responsive -- --project=firefox
```

`--with-deps` peut nécessiter une élévation pour les paquets système WSL. Sans ces droits, utiliser les jobs CI ; ne pas modifier la machine ni contourner les restrictions. La CI conserve les résultats texte de chaque moteur sans stocker de captures d'images privées.

## Recette sur appareils réels — à renseigner

Tester sur la copie de recette avec la configuration locale/ngrok déjà validée, jamais sur la base originale. Relever le commit testé, appareil, OS, navigateur/version et date. Une ligne non renseignée reste non validée.

| Contrôle | Appareil/navigateur | Résultat/preuve |
|---|---|---|
| Téléphone Android, portrait/paysage : catalogue, menu admin, tableaux | À renseigner | À vérifier |
| iPhone Safari : mêmes parcours et retour Précédent/Suivant | À renseigner | À vérifier |
| Tablette : dialogues longs, clavier logiciel, dernières actions accessibles | À renseigner | À vérifier |
| Chrome/Edge Windows, Firefox, Safari macOS : zoom navigateur 200 % | À renseigner | À vérifier |
| Sélection native de plusieurs images, quotas et reprise de revue | À renseigner | À vérifier |
| Clavier seul et lecteur d'écran : focus, erreurs et chargements annoncés | À renseigner | À vérifier |
| Impression/export, ventes/paiements/retours sur recette | À renseigner | À vérifier |
| Réseau lent/coupé : saisie conservée, absence de nouvel essai automatique | À renseigner | À vérifier |

## Clôture

Après fusion : suite métier et trois moteurs verts au commit fusionné, puis résultats de recette réelle consignés. La partie automatisée est livrée avec ce PR ; la validation tous appareils reste ouverte jusqu'à ces preuves. La validation HTTPS en conditions VPS reste différée conformément à la décision antérieure et ne fait pas partie de ce jalon.

Références : [navigateurs Playwright](https://playwright.dev/docs/browsers), [projets Playwright](https://playwright.dev/docs/test-projects).
