# RESP-08 — retours de requête et interactions

Le catalogue public affiche un statut de chargement en arabe pendant sa navigation partielle. Le statut est annoncé poliment, ne capte pas le toucher, tient dans un écran de 320 px et ne décale pas la mise en page. Annulation d'une requête obsolète et repli SSR restent ceux de RESP-07.

L'admin affiche un statut commun pour les lectures JSON après 150 ms afin d'éviter le clignotement des requêtes rapides. Les lectures simultanées et coalescées sont comptées jusqu'au décodage. Les images binaires et flux de téléchargement ne déclenchent pas ce statut. Les erreurs restent dans leurs emplacements et messages métier existants.

Quand une modification JSON part d'un formulaire `entity-form` contenant le contrôle focalisé, le formulaire reçoit `aria-busy`, un statut local, et ses boutons de soumission sont désactivés. Une seconde soumission est interceptée pendant la requête. Les champs ne sont ni effacés ni désactivés ; fin, erreur ou annulation restaurent l'état initial des boutons et l'attribut busy. Aucun nouvel essai automatique de mutation n'est ajouté. Cette protection UI complète les garanties serveur ; elle ne remplace pas l'idempotence et ne couvre pas les mutations programmatiques sans formulaire focalisé, ni une opération métier entière après la fin de sa requête.

Le helper optionnel est intégré au script de navigation déjà chargé et au décodage JSON HTTP partagé. Authentification, rafraîchissement, autorisation, cache/coalescence et paramètres réseau restent inchangés. Le login et l'interface React d'import conservent leurs protections et retours locaux existants. Aucun changement Go, backend, données, `.env` ou dépendances.

Le budget JS admin inclut les ajouts : 205 000 → 207 000 octets, mesure 206 386 octets sans exclusion, toujours 25 scripts et une lecture initiale `auth/me`. Les styles communs sont dans `responsive.css`.

## Vérification

Trois tests navigateur ajoutés : requête publique lente et statut mobile ; création client en échec avec refus de double soumission, champs conservés puis nouvel essai explicite réussi ; statut de lectures simultanées/coalescées jusqu'à leur fin. Les données client du test sont créées par les API existantes dans la base jetable.

Recette manuelle : réseau lent/hors ligne, erreurs métier, formulaire long avec clavier logiciel, thèmes clair/sombre, lecteur d'écran, Safari/Firefox. Les états vides existants et les messages de quota restent conservés. Prochaine étape : RESP-09, matrice finale de vérification responsive et non-régression.
