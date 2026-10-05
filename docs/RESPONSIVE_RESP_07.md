# RESP-07 — navigation fluide et état de lecture

Le catalogue public utilise les GET HTML existants pour actualiser uniquement `main` lors d'une recherche, d'une pagination ou d'un lien vers `/`. Il conserve les liens/formulaires SSR fonctionnels sans JavaScript. Aucun endpoint ni format backend n'est ajouté. Une réponse de même origine, HTML, succès, sur `/`, avec les deux sections attendues et sans script dans `main`, est nécessaire avant remplacement. Les contenus proviennent exclusivement des templates serveur échappés existants ; `DOMParser` n'est pas un assainisseur pour du HTML tiers.

Le dernier déplacement annule le précédent ; une erreur réseau, HTTP, redirection ou réponse inattendue revient à la navigation document habituelle. L'historique garde recherche et page dans l'URL ; Précédent/Suivant recharge les résultats par le même GET et restaure la position. La vue cartes/tableau reste une préférence facultative. `aria-busy`, focus sur le titre des résultats et aperçu de secours sont réinitialisés après chaque mise à jour. Ctrl/Maj/clic du milieu, téléchargements et liens externes gardent leur comportement natif.

Dans l'admin, les changements de section utilisent des entrées d'historique plutôt que remplacer la précédente. Précédent/Suivant réactive la section et son focus. Les filtres et lignes déjà chargés restent dans le même DOM ; aucun appel API supplémentaire, résultat métier mis en cache ou stockage persistant de filtre admin. Le catalogue public n'est pas appliqué à l'admin, au login ou aux imports React. La restauration interpages de filtres admin après rechargement complet reste hors périmètre : aucun état privé n'est sérialisé.

## Vérification

Trois nouveaux tests navigateur couvrent les résultats réels sur deux pages, les mises à jour sans rechargement, l'URL, la vue, le retour à la position, l'annulation d'une réponse lente, le repli serveur sur panne et l'historique admin conservant sa recherche. Les autres scénarios responsive, métier, authentification, XSS et performance restent inchangés.

Recette manuelle : Précédent/Suivant et recherche rapide sur Safari/Firefox, réseau lent/coupé, JavaScript désactivé, stockage désactivé, liens ouverts dans un nouvel onglet, zoom/lecteur d'écran. La navigation complète reste la solution de repli. Backend, API, Go, données, rôles et `.env` ne changent pas. Prochaine étape : RESP-08, états et interactions cohérents.
