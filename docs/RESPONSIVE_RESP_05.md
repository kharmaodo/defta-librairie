# RESP-05 — formulaires et dialogues admin

Les dialogues natifs gardent leurs champs, contraintes de validation, événements métier et gestion du focus. Aucun changement Go, API, session, rôles, données ou configuration.

Le fichier `static/css/admin-forms.css`, chargé après les styles existants, limite la hauteur à l'espace disponible (`dvh`, avec repli `vh`) et permet un défilement interne. Les titres longs se replient, les grilles utilisent des colonnes sans largeur minimale implicite. Sous 900 px, les contrôles atteignent 44 px, les textes des champs 16 px et les lignes de vente/achat/retour passent sur une colonne. Sous 600 px, les dialogues utilisent une marge réduite, les formulaires une colonne et les actions toute la largeur. Aucun pied de dialogue fixe ne recouvre les champs. Une interception de Tab/Shift+Tab aux limites du dialogue modal assure le retour au premier/dernier contrôle visible ; elle ne modifie ni Échap ni les soumissions. Ce code est ajouté au script de navigation déjà chargé, sans requête supplémentaire. Le budget JS est explicitement porté de 203 500 à 205 000 octets pour inclure ce correctif, sans exclusion. Toutes les règles sont limitées à l'écran pour préserver les reçus imprimés.

## Couverture

Le socle concerne les dialogues propriétaires, mots de passe, livres/historique/stock, fournisseurs, clients, achats, ventes, caisses, paiements, retours, référentiels, confirmation destructive et relance de soumission. Les formulaires intégrés aux dialogues héritent de ces règles. L'interface dédiée `/admin/admin-covers` sera traitée par RESP-06.

Quatre tests Playwright couvrent 320×700, 390×844, 768×1024 et 812×375 : ouverture réelle des formulaires livre/client/fournisseur/achat, absence de débordement horizontal, actions accessibles par défilement, validation numérique native, conservation de la saisie arabe, ajout/retrait de ligne d'achat, annulation, focus clavier et Échap. Les autres tests métier existants vérifient la persistance et les parcours complets.

## Recette manuelle

- Tester les formulaires longs, erreurs serveur, catégories multiples et actions en thème clair/sombre.
- Sur téléphone réel, ouvrir le clavier logiciel, atteindre le dernier champ puis Annuler/Enregistrer ; confirmer que le clavier ne masque pas durablement les actions.
- Vérifier Safari/Firefox, zoom 200 %, lecteur d'écran et rotation du téléphone.
- Vérifier les reçus imprimés et le changement obligatoire de mot de passe.

Les dimensions Playwright ne simulent pas le clavier logiciel ni tous les moteurs mobiles : ces vérifications restent manuelles. Prochaine étape : RESP-06, couvertures/imports.
