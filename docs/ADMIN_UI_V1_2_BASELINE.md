# Fondation mesurée du dashboard — v1.2.0

## État de référence

Les mesures sont relevées sur `develop` au commit `84b4bb8`, après la fusion de
la PR #62. Elles décrivent les ressources servies par `admin.html` sans prétendre
mesurer le réseau ou les performances d’un appareil de production.

| Ressource | Référence | Budget initial |
|---|---:|---:|
| `templates/admin.html` | 61 408 octets | 70 000 octets |
| `static/css/admin.css` | 26 056 octets | 35 000 octets |
| 20 modules `static/js/admin-*.js` | 163 983 octets | 190 000 octets |

Les budgets sont des garde-fous de croissance non compressée. Ils ne remplacent
pas les mesures navigateur avant/après prévues lors de l’incrément performance.

## Breakpoints de recette

| Contexte | Largeur Playwright | Attente principale |
|---|---:|---|
| Mobile | 390 px | Navigation hors-canvas, contenu sans débordement de page |
| Tablette | 768 px | Formulaires utilisables et tableaux explicitement défilables |
| Petit bureau | 1024 px | Hiérarchie lisible avec navigation adaptée |
| Bureau | 1440 px | Sidebar, contenu et métriques pleinement déployés |

Les seuils CSS existants de 780, 900 et 1100 px restent protégés pendant les
premiers incréments. Leur modification nécessitera une mesure et une adaptation
documentée des scénarios concernés.

## Indicateurs prioritaires proposés

La synthèse du dashboard utilisera uniquement des données déjà disponibles :

1. ventes nettes de la période, avec lien vers les statistiques ;
2. marge commerciale, avec état « indisponible » si un coût manque ;
3. ruptures et stocks faibles, avec lien vers les alertes ;
4. achats en brouillon nécessitant une action.

Le rôle root devra sélectionner une librairie avant l’affichage d’indicateurs
scopés. Un propriétaire ne verra que sa librairie. Chaque carte indiquera sa
période ou la mention « situation actuelle », son unité, son état de chargement,
son erreur éventuelle et sa destination.

## Contrat protégé

Le contrôle automatisé conserve les vingt sections principales, les dix-neuf
dialogues statiques et le dialogue dynamique des retours fournisseurs, les vingt
modules JavaScript d’administration, les contrôles globaux
de session et les attributs `data-dashboard-*`. Les sélecteurs métier plus fins
restent couverts par les tests frontend et les parcours Playwright existants.

Les états à couvrir lors de la synthèse sont : chargement, résultat, valeur nulle,
donnée indisponible, erreur HTTP et absence de sélection pour le rôle root.

## Mesures navigateur à relever avant l’optimisation

- nombre de requêtes et octets transférés lors du premier affichage authentifié ;
- événements `DOMContentLoaded` et `load` dans le même environnement de test ;
- nombre d’appels API au démarrage pour root et propriétaire ;
- absence de requête dupliquée lors d’un renouvellement de jeton ;
- stabilité des parcours aux quatre largeurs de référence.

Ces résultats seront enregistrés avec la machine, le navigateur, la configuration
du serveur et le commit testés afin que la comparaison reste reproductible.
