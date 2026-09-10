# Exemples curl — Defta Librairie

Le contrat de référence est `static/openapi.json` (OpenAPI 3.0.3).
Consultation locale : `/static/api-docs.html`. Import possible dans les outils
compatibles OpenAPI. Référence : https://spec.openapis.org/oas/v3.0.3.html.

## Préparer les valeurs

```bash
BASE_URL=http://localhost:8080
# TOKEN : accessToken obtenu par login ; root pour /api/admin/owners.
# RESOURCE_ID : identifiant de la ressource du chemin, entier pour un livre.
# VERSION : valeur courante renvoyée par GET.
```

Chaque exemple est indépendant. Remplacer les valeurs majuscules dans les JSON
par des valeurs réelles ; les heredocs cités évitent toute expansion shell.
Ne pas exécuter ce document comme un scénario : certaines commandes modifient
ou suppriment des ressources. Les versions sont celles du dernier GET.
Pour un propriétaire, libraryId doit désigner sa librairie ; root doit choisir
une librairie sur les créations et sur statistiques/alertes/paramétrage.
Ajouter `libraryId` dans la query de ces trois derniers endpoints avec root.
Pour créer un retour avec root, fournir aussi libraryId correspondant à la
vente ou à l’achat rattaché. XOF est la devise unique.

## Session navigateur et rotation

Le mode par défaut renvoie refreshToken en JSON. Avec
`-H 'X-Defta-Session: cookie' -c cookies.txt` sur login, le serveur le place dans
un cookie HttpOnly. Pour refresh/logout, utiliser ce même header et
`-b cookies.txt -c cookies.txt` sans corps JSON. Chaque refresh remplace le jeton.
Après un changement de mot de passe, se reconnecter. Ne pas publier les jetons.

## Dates et formats

Listes ventes/achats/retours et audit : bornes incluses sur createdAt.
Statistiques et historique client : fin exclue ; historique sur confirmedAt,
ou createdAt pour les brouillons. Exports : fin exclue, updatedAt pour stocks,
createdAt pour les autres ressources. Consulter le détail de chaque opération.
La pagination habituelle normalise les valeurs invalides, sauf alertes et
historique client qui les rejettent. CSV : UTF-8 BOM, point-virgule, maximum
10 000 lignes, 422 au-delà. Un coût inconnu reste vide ; les cellules pouvant
être des formules sont préfixées d’une apostrophe.

## Catalogue des commandes

### POST /api/auth/login

 Mode JSON par défaut ; X-Defta-Session: cookie sélectionne le cookie defta_refresh. Refresh fait tourner le jeton ; sa réutilisation invalide la famille de sessions. Logout attend un refresh token, pas un access token.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/auth/login" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "username": "VOTRE_UTILISATEUR",
  "password": "VOTRE_MOT_DE_PASSE"
}
JSON
```

### POST /api/auth/refresh

 Mode JSON par défaut ; X-Defta-Session: cookie sélectionne le cookie defta_refresh. Refresh fait tourner le jeton ; sa réutilisation invalide la famille de sessions. Logout attend un refresh token, pas un access token.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/auth/refresh" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "refreshToken": "VOTRE_REFRESH_TOKEN"
}
JSON
```

### POST /api/auth/logout

 Mode JSON par défaut ; X-Defta-Session: cookie sélectionne le cookie defta_refresh. Refresh fait tourner le jeton ; sa réutilisation invalide la famille de sessions. Logout attend un refresh token, pas un access token.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/auth/logout" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "refreshToken": "VOTRE_REFRESH_TOKEN"
}
JSON
```

### GET /api/auth/me

Session JWT active requise ; accessible avant changement de mot de passe.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/auth/me" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/auth/change-password

Session JWT active requise ; accessible avant changement de mot de passe.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/auth/change-password" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "currentPassword": "MOT_DE_PASSE_ACTUEL",
  "newPassword": "NOUVEAU_MOT_DE_PASSE_FORT"
}
JSON
```

### GET /api/auth/sessions

Session JWT active requise ; accessible avant changement de mot de passe. Root peut filtrer les comptes ; propriétaire limité à ses sessions.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/auth/sessions" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/auth/sessions/revoke-others

Session JWT active requise ; accessible avant changement de mot de passe.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/auth/sessions/revoke-others" \
  -H "Authorization: Bearer ${TOKEN}"
```

### DELETE /api/auth/sessions/{id}

Session JWT active requise ; accessible avant changement de mot de passe.

```bash
curl --fail-with-body -sS -X DELETE "${BASE_URL}/api/auth/sessions/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### GET /api/audit-logs

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Propriétaire : ses propres événements ; filtre actor réservé à root.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/audit-logs" \
  -H "Authorization: Bearer ${TOKEN}"
```

### GET /api/admin/owners

SUPER_ADMIN_ROOT uniquement ; mot de passe temporaire à changer.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/admin/owners" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/admin/owners

SUPER_ADMIN_ROOT uniquement ; mot de passe temporaire à changer.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/admin/owners" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "username": "nouveau-proprietaire",
  "email": "contact@example.com",
  "password": "MOT_DE_PASSE_INITIAL_FORT",
  "library": {
    "name": "Librairie exemple",
    "description": "Librairie à Dakar"
  }
}
JSON
```

### GET /api/admin/owners/{id}

SUPER_ADMIN_ROOT uniquement ; mot de passe temporaire à changer.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/admin/owners/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### PATCH /api/admin/owners/{id}

SUPER_ADMIN_ROOT uniquement ; mot de passe temporaire à changer.

```bash
curl --fail-with-body -sS -X PATCH "${BASE_URL}/api/admin/owners/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "library": {
    "name": "Librairie renommée"
  }
}
JSON
```

### DELETE /api/admin/owners/{id}

SUPER_ADMIN_ROOT uniquement ; mot de passe temporaire à changer.

```bash
curl --fail-with-body -sS -X DELETE "${BASE_URL}/api/admin/owners/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/admin/owners/{id}/unlock

SUPER_ADMIN_ROOT uniquement ; mot de passe temporaire à changer.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/admin/owners/${RESOURCE_ID}/unlock" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/admin/owners/{id}/reactivate

SUPER_ADMIN_ROOT uniquement ; mot de passe temporaire à changer.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/admin/owners/${RESOURCE_ID}/reactivate" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/admin/owners/{id}/reset-password

SUPER_ADMIN_ROOT uniquement ; mot de passe temporaire à changer.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/admin/owners/${RESOURCE_ID}/reset-password" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "password": "NOUVEAU_MOT_DE_PASSE_FORT"
}
JSON
```

### GET /api/manage/alerts

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Root doit fournir libraryId. Paramètres inconnus ou répétés rejetés.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/alerts" \
  -H "Authorization: Bearer ${TOKEN}"
```

### GET /api/manage/library-settings

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Root doit fournir libraryId. Paramètres inconnus ou répétés rejetés. XOF fixe. GET retourne les champs complets ; PUT remplace les champs éditables et renvoie 204. Logo PNG/JPEG ≤128 Kio, ≤1024×1024 ; corps limité à 200 000 octets. Coordonnées actuelles utilisées à l’impression.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/library-settings" \
  -H "Authorization: Bearer ${TOKEN}"
```

### PUT /api/manage/library-settings

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Root doit fournir libraryId. Paramètres inconnus ou répétés rejetés. XOF fixe. GET retourne les champs complets ; PUT remplace les champs éditables et renvoie 204. Logo PNG/JPEG ≤128 Kio, ≤1024×1024 ; corps limité à 200 000 octets. Coordonnées actuelles utilisées à l’impression.

```bash
curl --fail-with-body -sS -X PUT "${BASE_URL}/api/manage/library-settings" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "currency": "XOF",
  "address": "Dakar",
  "phone": "",
  "email": "contact@example.com",
  "logoData": "",
  "defaultLowStockThreshold": 5,
  "printFooter": "Merci de votre visite",
  "version": 1
}
JSON
```

### GET /api/manage/exports/{kind}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Root : libraryId obligatoire sauf audit où il est interdit. q : sous-chaîne titre/référence/nom/identifiant de ressource. action et resourceType réservés à audit ; status interdit pour audit. Dates de modification stock, création métier, événement audit. Paramètres inconnus ou répétés rejetés.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/exports/stocks" \
  -H "Authorization: Bearer ${TOKEN}" \
  --output stocks.csv
```

### GET /api/manage/books

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/books" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/books

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Création root : libraryId obligatoire dans le JSON ; propriétaire : sa librairie est déduite si omis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/books" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "title": "كتاب exemple",
  "price": 1500,
  "volume": 1,
  "status": "AVAILABLE",
  "libraryId": "LIBRARY_ID"
}
JSON
```

### GET /api/manage/books/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/books/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### PUT /api/manage/books/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X PUT "${BASE_URL}/api/manage/books/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "title": "كتاب exemple",
  "price": 1500,
  "volume": 1,
  "status": "AVAILABLE",
  "libraryId": "LIBRARY_ID",
  "version": 1
}
JSON
```

### DELETE /api/manage/books/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X DELETE "${BASE_URL}/api/manage/books/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### GET /api/manage/books/{id}/history

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/books/${RESOURCE_ID}/history" \
  -H "Authorization: Bearer ${TOKEN}"
```

### GET /api/manage/inventory

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/inventory" \
  -H "Authorization: Bearer ${TOKEN}"
```

### GET /api/manage/books/{id}/inventory

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/books/${RESOURCE_ID}/inventory" \
  -H "Authorization: Bearer ${TOKEN}"
```

### PUT /api/manage/books/{id}/inventory

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. quantity représente la quantité absolue après ajustement, et non un delta.

```bash
curl --fail-with-body -sS -X PUT "${BASE_URL}/api/manage/books/${RESOURCE_ID}/inventory" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "quantity": 2,
  "reason": "Comptage physique",
  "version": 1
}
JSON
```

### POST /api/manage/books/{id}/inventory/entries

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/books/${RESOURCE_ID}/inventory/entries" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "quantity": 2,
  "reason": "Comptage physique",
  "version": 1
}
JSON
```

### POST /api/manage/books/{id}/inventory/exits

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/books/${RESOURCE_ID}/inventory/exits" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "quantity": 2,
  "reason": "Comptage physique",
  "version": 1
}
JSON
```

### PATCH /api/manage/books/{id}/inventory/threshold

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X PATCH "${BASE_URL}/api/manage/books/${RESOURCE_ID}/inventory/threshold" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "lowStockThreshold": 5,
  "version": 1
}
JSON
```

### GET /api/manage/books/{id}/inventory/movements

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/books/${RESOURCE_ID}/inventory/movements" \
  -H "Authorization: Bearer ${TOKEN}"
```

### GET /api/manage/suppliers

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/suppliers" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/suppliers

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Création root : libraryId obligatoire dans le JSON ; propriétaire : sa librairie est déduite si omis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/suppliers" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "name": "Fournisseur exemple",
  "libraryId": "LIBRARY_ID"
}
JSON
```

### GET /api/manage/suppliers/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/suppliers/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### PUT /api/manage/suppliers/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X PUT "${BASE_URL}/api/manage/suppliers/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "name": "Fournisseur exemple",
  "libraryId": "LIBRARY_ID",
  "version": 1
}
JSON
```

### DELETE /api/manage/suppliers/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X DELETE "${BASE_URL}/api/manage/suppliers/${RESOURCE_ID}?version=${VERSION}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/suppliers/{id}/reactivate

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/suppliers/${RESOURCE_ID}/reactivate?version=${VERSION}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### GET /api/manage/customers

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/customers" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/customers

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Création root : libraryId obligatoire dans le JSON ; propriétaire : sa librairie est déduite si omis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/customers" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "name": "Client exemple",
  "libraryId": "LIBRARY_ID"
}
JSON
```

### GET /api/manage/customers/{id}/sales

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Client désactivé consultable. Sans status : CONFIRMED et CANCELLED ; DRAFT uniquement sur filtre explicite. Date de confirmation, création pour brouillon. Montants bruts avant retours. Filtres inconnus/répétés rejetés.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/customers/${RESOURCE_ID}/sales" \
  -H "Authorization: Bearer ${TOKEN}"
```

### GET /api/manage/customers/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/customers/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### PUT /api/manage/customers/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X PUT "${BASE_URL}/api/manage/customers/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "name": "Client exemple",
  "libraryId": "LIBRARY_ID",
  "version": 1
}
JSON
```

### DELETE /api/manage/customers/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X DELETE "${BASE_URL}/api/manage/customers/${RESOURCE_ID}?version=${VERSION}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/customers/{id}/reactivate

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/customers/${RESOURCE_ID}/reactivate?version=${VERSION}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### GET /api/manage/cash-registers

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/cash-registers" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/cash-registers

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Création root : libraryId obligatoire dans le JSON ; propriétaire : sa librairie est déduite si omis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/cash-registers" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "name": "Caisse principale",
  "libraryId": "LIBRARY_ID"
}
JSON
```

### GET /api/manage/cash-registers/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/cash-registers/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### PUT /api/manage/cash-registers/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X PUT "${BASE_URL}/api/manage/cash-registers/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "name": "Caisse principale",
  "libraryId": "LIBRARY_ID",
  "version": 1
}
JSON
```

### DELETE /api/manage/cash-registers/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X DELETE "${BASE_URL}/api/manage/cash-registers/${RESOURCE_ID}?version=${VERSION}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/cash-registers/{id}/reactivate

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/cash-registers/${RESOURCE_ID}/reactivate?version=${VERSION}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### GET /api/manage/purchases

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/purchases" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/purchases

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Création root : libraryId obligatoire dans le JSON ; propriétaire : sa librairie est déduite si omis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/purchases" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "libraryId": "LIBRARY_ID",
  "supplierId": "SUPPLIER_ID",
  "lines": [
    {
      "bookId": 1,
      "quantity": 2,
      "unitCost": 1000
    }
  ]
}
JSON
```

### GET /api/manage/purchases/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/purchases/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### PUT /api/manage/purchases/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X PUT "${BASE_URL}/api/manage/purchases/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "libraryId": "LIBRARY_ID",
  "supplierId": "SUPPLIER_ID",
  "lines": [
    {
      "bookId": 1,
      "quantity": 2,
      "unitCost": 1000
    }
  ],
  "version": 1
}
JSON
```

### DELETE /api/manage/purchases/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X DELETE "${BASE_URL}/api/manage/purchases/${RESOURCE_ID}?version=${VERSION}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/purchases/{id}/receive

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Transition transactionnelle avec mouvements de stock et audit ; utiliser la version courante.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/purchases/${RESOURCE_ID}/receive" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "version": 1
}
JSON
```

### POST /api/manage/purchases/{id}/cancel

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/purchases/${RESOURCE_ID}/cancel" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "version": 1
}
JSON
```

### GET /api/manage/supplier-returns

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/supplier-returns" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/supplier-returns

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Création root : libraryId obligatoire dans le JSON ; propriétaire : sa librairie est déduite si omis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/supplier-returns" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "libraryId": "LIBRARY_ID",
  "purchaseId": "PURCHASE_ID",
  "reason": "Article défectueux",
  "lines": [
    {
      "purchaseLineId": "PURCHASE_LINE_ID",
      "quantity": 1
    }
  ]
}
JSON
```

### GET /api/manage/supplier-returns/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/supplier-returns/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### PUT /api/manage/supplier-returns/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X PUT "${BASE_URL}/api/manage/supplier-returns/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "libraryId": "LIBRARY_ID",
  "purchaseId": "PURCHASE_ID",
  "reason": "Article défectueux",
  "lines": [
    {
      "purchaseLineId": "PURCHASE_LINE_ID",
      "quantity": 1
    }
  ],
  "version": 1
}
JSON
```

### POST /api/manage/supplier-returns/{id}/cancel

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/supplier-returns/${RESOURCE_ID}/cancel" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "version": 1
}
JSON
```

### POST /api/manage/supplier-returns/{id}/ship

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Transition transactionnelle avec mouvements de stock et audit ; utiliser la version courante.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/supplier-returns/${RESOURCE_ID}/ship" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "version": 1
}
JSON
```

### GET /api/manage/tags

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/tags" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/tags

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Création root : libraryId obligatoire dans le JSON ; propriétaire : sa librairie est déduite si omis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/tags" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "name": "fiqh",
  "libraryId": "LIBRARY_ID"
}
JSON
```

### PATCH /api/manage/tags/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X PATCH "${BASE_URL}/api/manage/tags/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "name": "fiqh"
}
JSON
```

### DELETE /api/manage/tags/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X DELETE "${BASE_URL}/api/manage/tags/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### GET /api/books

Catalogue public et recherche FTS5 par q, avec pagination. Aucun JWT requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/books"
```

### GET /api/manage/statistics

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Période [from,to) obligatoire, root doit fournir libraryId. Marge et coûts inconnus peuvent être null. Dates des événements : confirmation vente, annulation vente, finalisation retour client, réception achat, expédition retour fournisseur.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/statistics?from=2026-09-01T00:00:00Z&to=2026-10-01T00:00:00Z" \
  -H "Authorization: Bearer ${TOKEN}"
```

### GET /api/manage/sales

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/sales" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/sales

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Création root : libraryId obligatoire dans le JSON ; propriétaire : sa librairie est déduite si omis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/sales" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "libraryId": "LIBRARY_ID",
  "customerId": "CUSTOMER_ID",
  "lines": [
    {
      "bookId": 1,
      "quantity": 1
    }
  ]
}
JSON
```

### GET /api/manage/sales/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/sales/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### PUT /api/manage/sales/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X PUT "${BASE_URL}/api/manage/sales/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "libraryId": "LIBRARY_ID",
  "customerId": "CUSTOMER_ID",
  "lines": [
    {
      "bookId": 1,
      "quantity": 1
    }
  ],
  "version": 1
}
JSON
```

### DELETE /api/manage/sales/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X DELETE "${BASE_URL}/api/manage/sales/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/sales/{id}/confirm

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Transition transactionnelle avec mouvements de stock et audit ; utiliser la version courante.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/sales/${RESOURCE_ID}/confirm" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "version": 1
}
JSON
```

### POST /api/manage/sales/{id}/cancel

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Refus si paiements enregistrés ou retours finalisés ; ne pas assimiler annulation de paiement et remboursement.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/sales/${RESOURCE_ID}/cancel" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "version": 1
}
JSON
```

### GET /api/manage/sales/{id}/payments

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/sales/${RESOURCE_ID}/payments" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/sales/{id}/payments

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/sales/${RESOURCE_ID}/payments" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "cashRegisterId": "CASH_REGISTER_ID",
  "method": "CASH",
  "amount": 500
}
JSON
```

### GET /api/manage/sales/{id}/payment-balance

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Montants bruts ; distinguer reste commercial et plafond remboursable. refundableAmount peut être null pour un avoir.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/sales/${RESOURCE_ID}/payment-balance" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/payments/{id}/void

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Annulation comptable avec version et motif ; ne constitue pas un remboursement physique.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/payments/${RESOURCE_ID}/void" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "version": 1,
  "reason": "Erreur de saisie"
}
JSON
```

### GET /api/manage/customer-returns

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/customer-returns" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/customer-returns

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Création root : libraryId obligatoire dans le JSON ; propriétaire : sa librairie est déduite si omis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/customer-returns" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "libraryId": "LIBRARY_ID",
  "saleId": "SALE_ID",
  "reason": "Retour client",
  "resolution": "REFUND",
  "lines": [
    {
      "saleLineId": "SALE_LINE_ID",
      "quantity": 1
    }
  ]
}
JSON
```

### GET /api/manage/customer-returns/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/customer-returns/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}"
```

### PUT /api/manage/customer-returns/{id}

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X PUT "${BASE_URL}/api/manage/customer-returns/${RESOURCE_ID}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "libraryId": "LIBRARY_ID",
  "saleId": "SALE_ID",
  "reason": "Retour client",
  "resolution": "REFUND",
  "lines": [
    {
      "saleLineId": "SALE_LINE_ID",
      "quantity": 1
    }
  ],
  "version": 1
}
JSON
```

### POST /api/manage/customer-returns/{id}/complete

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Transition transactionnelle avec mouvements de stock et audit ; utiliser la version courante.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/customer-returns/${RESOURCE_ID}/complete" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "version": 1
}
JSON
```

### POST /api/manage/customer-returns/{id}/cancel

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/customer-returns/${RESOURCE_ID}/cancel" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "version": 1
}
JSON
```

### GET /api/manage/customer-returns/{id}/settlements

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/customer-returns/${RESOURCE_ID}/settlements" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/customer-returns/{id}/settlements

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/customer-returns/${RESOURCE_ID}/settlements" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "method": "CASH",
  "amount": 500
}
JSON
```

### GET /api/manage/customer-returns/{id}/settlement-balance

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Montants bruts ; distinguer reste commercial et plafond remboursable. refundableAmount peut être null pour un avoir.

```bash
curl --fail-with-body -sS -X GET "${BASE_URL}/api/manage/customer-returns/${RESOURCE_ID}/settlement-balance" \
  -H "Authorization: Bearer ${TOKEN}"
```

### POST /api/manage/return-settlements/{id}/void

OWNER_LIBRARY dans son périmètre ou SUPER_ADMIN_ROOT ; mot de passe changé requis. Annulation comptable avec version et motif ; ne constitue pas un remboursement physique.

```bash
curl --fail-with-body -sS -X POST "${BASE_URL}/api/manage/return-settlements/${RESOURCE_ID}/void" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data-binary @- <<'JSON'
{
  "version": 1,
  "reason": "Erreur de saisie"
}
JSON
```
