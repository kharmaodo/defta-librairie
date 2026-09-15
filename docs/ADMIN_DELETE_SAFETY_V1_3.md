# Inventaire des suppressions — v1.3.0

## Portée

Cet inventaire fixe le contrat de l’incrément 1 de `v1.3.0`. Il distingue la
méthode HTTP employée de la conséquence métier réelle : un `DELETE` peut
désactiver ou révoquer sans effacer les données, tandis qu’une route `POST`
peut déclencher une transition irréversible.

La confirmation renforcée par saisie protège les actions qui font disparaître
une ressource de l’usage normal sans restauration disponible dans l’interface.
Elle ne remplace pas les autorisations, le contrôle de périmètre par librairie,
le contrôle de version ni l’audit côté serveur.

## Suppressions à protéger par saisie exacte

| Ressource | Déclencheur actuel | Route | Effet observé | Texte exact attendu |
|---|---|---|---|---|
| Livre | `[data-action="delete-book"]` | `DELETE /api/manage/books/{id}` | Suppression logique par `deleted_at`, sans restauration exposée | Titre complet du livre |
| Tag | `#tags-list button[data-id]` | `DELETE /api/manage/tags/{id}` | Suppression physique du tag | Nom complet du tag |
| Brouillon de vente | `[data-action="delete-sale"]` | `DELETE /api/manage/sales/{id}` | Suppression physique de la vente DRAFT et de ses lignes | Référence de la vente |
| Brouillon d’achat | Aucun déclencheur dans l’interface actuelle | `DELETE /api/manage/purchases/{id}?version={version}` | Suppression physique de l’achat DRAFT et de ses lignes | Référence de l’achat si l’action est exposée |

Le livre reste dans cette liste bien que sa persistance soit logique : pour
l’utilisateur, l’action est actuellement sans mécanisme de restauration. Le
qualificatif « irréversible » décrit donc le parcours disponible, pas seulement
la présence physique de la ligne SQLite.

La route de suppression d’un brouillon d’achat ne doit pas être raccordée
implicitement pendant `v1.3.0`. Si elle est exposée ultérieurement, elle devra
réutiliser le même dialogue avec la référence exacte.

## Actions exclues de la confirmation renforcée

| Domaine | Action | Route ou mécanisme | Classification | Confirmation conservée |
|---|---|---|---|---|
| Propriétaire | Désactiver | `DELETE /api/admin/owners/{id}` | Désactivation réversible | Confirmation simple |
| Fournisseur | Désactiver | `DELETE /api/manage/suppliers/{id}` | Désactivation réversible | Confirmation simple |
| Client | Désactiver | `DELETE /api/manage/customers/{id}` | Désactivation réversible | Confirmation simple |
| Caisse | Désactiver | `DELETE /api/manage/cash-registers/{id}` | Désactivation réversible | Confirmation simple |
| Session | Révoquer | `DELETE /api/auth/sessions/{id}` | Révocation de sécurité | Confirmation simple |
| Vente confirmée | Annuler | transition dédiée | Transition métier auditée, avec restitution du stock | Confirmation simple |
| Achat | Annuler ou réceptionner | transition dédiée | Transition métier auditée | Confirmation simple |
| Retour client | Finaliser ou annuler | transition dédiée | Transition métier auditée | Confirmation simple |
| Retour fournisseur | Expédier ou annuler | transition dédiée | Transition métier auditée | Confirmation simple |
| Paiement | Annuler | transition dédiée | Transition financière auditée | Confirmation simple |
| Compte | Déverrouiller ou réactiver | transition dédiée | Action de restauration | Confirmation simple |

Le mot `DELETE` dans une route ne suffit donc pas à déclencher le nouveau
dialogue. Inversement, toute future suppression définitive doit être ajoutée à
la première matrice avant son exposition dans l’interface.

## Contrat du dialogue commun

Le composant prévu par l’incrément 3 respectera les garanties suivantes :

- un seul dialogue partagé par les modules admin ;
- titre et avertissement irréversible associés au dialogue ;
- nom ou référence attendue affichée comme texte, jamais seulement comme
  placeholder ;
- égalité JavaScript stricte entre la saisie et la valeur attendue ;
- comparaison sensible à la casse, sans `trim()`, normalisation ou correction ;
- confirmation désactivée tant que la valeur diffère ;
- remise à zéro de la saisie, de l’état désactivé et des erreurs à chaque
  fermeture, y compris par Échap ;
- focus initial dans la saisie et retour au bouton déclencheur à la fermeture ;
- blocage de la double soumission pendant la requête ;
- erreur HTTP annoncée dans le dialogue sans perdre la saisie utile ;
- fermeture et rafraîchissement uniquement après succès du serveur.

## Contrat de sélecteurs

Les sélecteurs métier existants restent stables :

| Usage | Sélecteur protégé |
|---|---|
| Suppression d’un livre | `[data-action="delete-book"]` |
| Suppression d’un tag | `#tags-list button[data-id]` |
| Suppression d’un brouillon de vente | `[data-action="delete-sale"]` |
| Tableau des livres | `#books-body` |
| Liste des tags | `#tags-list` |
| Tableau des ventes | `#sales-body` |

Le nouveau dialogue utilisera des sélecteurs dédiés et sémantiques, qui seront
figés au moment de son implémentation. Les tests ne doivent pas dépendre de
l’ordre des boutons ou de classes purement visuelles.

## Matrice de tests attendue

### Tests unitaires JavaScript

- valeur exacte : confirmation active ;
- casse différente : confirmation inactive ;
- espace initial ou final : confirmation inactive ;
- chaîne vide ou partielle : confirmation inactive ;
- fermeture puis réouverture : saisie vide et confirmation inactive ;
- double clic ou soumission concurrente : une seule requête ;
- erreur HTTP : dialogue ouvert et erreur annoncée ;
- succès : dialogue fermé et données rechargées.

### Tests Playwright

- parcours souris sur livre, tag et brouillon de vente ;
- saisie incorrecte puis exacte ;
- ouverture, Échap, réouverture et remise à zéro ;
- navigation au clavier, ordre de focus et restitution du focus ;
- texte irréversible accessible ;
- absence du dialogue renforcé pour une désactivation et une annulation ;
- non-régression des parcours commerciaux existants.

## Hors périmètre de l’incrément 1

Cet incrément ne modifie aucun dialogue, bouton, endpoint ou comportement
métier. Il ne rend pas visible la suppression d’un brouillon d’achat. Le
correctif des bordures de formulaire appartient à l’incrément 2 et
l’implémentation du dialogue commun à l’incrément 3.
