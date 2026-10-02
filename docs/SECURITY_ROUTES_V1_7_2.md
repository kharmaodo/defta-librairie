# Inventaire des routes — audit v1.7.2

Source : `cmd/main.go` sur `ea2f676`, puis câblage du correctif. Les GET acceptent aussi HEAD via net/http. Les méthodes non enregistrées sont refusées. `/admin` et `/admin/cover-imports` sont des shells publics sans données métier : les API exigent un JWT actif. Les scripts statiques restent publics intentionnellement. CSRF global : origine/Fetch Metadata sur les mutations ; token double-submit signé pour le mode cookie ; les autres API ne lisent aucun cookie comme identité.

| Méthode | Route | Middleware / rôle |
|---|---|---|
| GET | `/api/health/live` | Public |
| GET | `/api/health/ready` | Public |
| POST | `/api/auth/login` | Public ; validation credentials/refresh ; cookie + CSRF ou token explicite |
| POST | `/api/auth/refresh` | Public ; validation credentials/refresh ; cookie + CSRF ou token explicite |
| POST | `/api/auth/logout` | Public ; validation credentials/refresh ; cookie + CSRF ou token explicite |
| GET | `/api/auth/me` | Session JWT active ; identité courante |
| POST | `/api/auth/change-password` | Session JWT active ; identité courante |
| GET | `/api/auth/sessions` | Session JWT active ; identité courante |
| POST | `/api/auth/sessions/revoke-others` | Session JWT active ; identité courante |
| DELETE | `/api/auth/sessions/{id}` | Session JWT active ; identité courante |
| GET | `/api/audit-logs` | Session JWT + mot de passe changé + ROOT ou OWNER ; périmètre bibliothèque du service |
| GET | `/api/admin/metrics` | Session JWT + mot de passe changé + rôle ROOT |
| GET | `/api/admin/owners` | Session JWT + mot de passe changé + rôle ROOT |
| POST | `/api/admin/owners` | Session JWT + mot de passe changé + rôle ROOT |
| GET | `/api/admin/owners/{id}` | Session JWT + mot de passe changé + rôle ROOT |
| PATCH | `/api/admin/owners/{id}` | Session JWT + mot de passe changé + rôle ROOT |
| DELETE | `/api/admin/owners/{id}` | Session JWT + mot de passe changé + rôle ROOT |
| POST | `/api/admin/owners/{id}/unlock` | Session JWT + mot de passe changé + rôle ROOT |
| POST | `/api/admin/owners/{id}/reactivate` | Session JWT + mot de passe changé + rôle ROOT |
| POST | `/api/admin/owners/{id}/reset-password` | Session JWT + mot de passe changé + rôle ROOT |
| GET | `/api/manage/alerts` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/library-settings` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| PUT | `/api/manage/library-settings` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/exports/{kind}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/books` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/books` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/book-submissions` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/book-submissions` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/book-submissions/{id}/decision` | Session JWT + mot de passe changé + rôle ROOT |
| POST | `/api/manage/book-submissions/{id}/retry` | Session JWT + mot de passe changé + rôle ROOT |
| GET | `/api/manage/cover-imports` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/cover-imports` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/cover-imports/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/cover-imports/{id}/candidate-search` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/cover-imports/{id}/candidate-dismiss` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/cover-imports/{id}/source` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/cover-imports/{id}/decision` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/cover-imports/{id}/quarantine-decision` | Session JWT + mot de passe changé + rôle ROOT |
| POST | `/api/admin/cover-imports/{id}/legal-hold` | Session JWT + mot de passe changé + rôle ROOT |
| DELETE | `/api/admin/cover-imports/{id}/legal-hold` | Session JWT + mot de passe changé + rôle ROOT |
| GET | `/api/manage/books/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/books/{id}/history` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| PUT | `/api/manage/books/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| DELETE | `/api/manage/books/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/books/{id}/cover/status` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/books/{id}/cover/retry` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/books/{id}/cover` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/books/{id}/cover` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/inventory` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/books/{id}/inventory` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/books/{id}/inventory/entries` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/books/{id}/inventory/exits` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| PUT | `/api/manage/books/{id}/inventory` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| PATCH | `/api/manage/books/{id}/inventory/threshold` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/books/{id}/inventory/movements` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/suppliers` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/suppliers` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/suppliers/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| PUT | `/api/manage/suppliers/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| DELETE | `/api/manage/suppliers/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/suppliers/{id}/reactivate` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/customers` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/customers` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/customers/{id}/sales` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/customers/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| PUT | `/api/manage/customers/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| DELETE | `/api/manage/customers/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/customers/{id}/reactivate` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/cash-registers` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/cash-registers` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/cash-registers/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| PUT | `/api/manage/cash-registers/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| DELETE | `/api/manage/cash-registers/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/cash-registers/{id}/reactivate` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/purchases` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/purchases` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/purchases/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| PUT | `/api/manage/purchases/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| DELETE | `/api/manage/purchases/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/purchases/{id}/receive` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/purchases/{id}/cancel` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/supplier-returns` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/supplier-returns` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/supplier-returns/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| PUT | `/api/manage/supplier-returns/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/supplier-returns/{id}/cancel` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/supplier-returns/{id}/ship` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/categories` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/categories` | Session JWT + mot de passe changé + rôle ROOT |
| POST | `/api/manage/categories/{id}/disable` | Session JWT + mot de passe changé + rôle ROOT |
| PUT | `/api/manage/categories/{id}` | Session JWT + mot de passe changé + rôle ROOT |
| GET | `/api/manage/publishers` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/publishers` | Session JWT + mot de passe changé + rôle ROOT |
| POST | `/api/manage/publishers/{id}/disable` | Session JWT + mot de passe changé + rôle ROOT |
| PUT | `/api/manage/publishers/{id}` | Session JWT + mot de passe changé + rôle ROOT |
| GET | `/api/manage/tags` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/tags` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| PATCH | `/api/manage/tags/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| DELETE | `/api/manage/tags/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET/HEAD | `/static/` | Public |
| GET | `/{$}` | Public |
| GET | `/login` | Public |
| GET | `/admin` | Public |
| GET | `/admin/cover-imports` | Public |
| GET | `/api/books` | Public |
| GET | `/api/books/{id}/cover` | Public |
| GET | `/api/manage/statistics` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/sales` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/sales` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/sales/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| PUT | `/api/manage/sales/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| DELETE | `/api/manage/sales/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/sales/{id}/confirm` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/sales/{id}/cancel` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/sales/{id}/payments` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/sales/{id}/payments` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/sales/{id}/payment-balance` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/payments/{id}/void` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/customer-returns` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/customer-returns` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/customer-returns/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| PUT | `/api/manage/customer-returns/{id}` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/customer-returns/{id}/complete` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/customer-returns/{id}/cancel` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/customer-returns/{id}/settlements` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/customer-returns/{id}/settlements` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| GET | `/api/manage/customer-returns/{id}/settlement-balance` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |
| POST | `/api/manage/return-settlements/{id}/void` | Session JWT + mot de passe changé + ROOT ou OWNER (library_id) |

Statique : fichiers réguliers JS/CSS/images/polices autorisés ; OpenAPI public explicitement autorisé ; répertoires, dotfiles, maps et autres fichiers refusés. Couvertures publiques : dérivés actifs READY de livres non supprimés, sans source privée. Les endpoints de revue/source exigent le périmètre bibliothèque et le rôle existants.
