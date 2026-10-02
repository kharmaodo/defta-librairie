# Audit applicatif DEFTA-LIBRAIRIE — correctif proposé v1.7.2

Audit du code de `develop` à partir de `ea2f67653a6983c543216b8ab75b677b007a0a22`, le 2 octobre 2026. Les changements sont isolés sur `security/v1.7.2-audit`. Aucun changement de données, de migration historique, de `.env`, du feature flag OCR ou de la version publiée v1.7.1.

La sécurité repose sur l'autorisation serveur, le transport et les protections du navigateur. Les scripts admin restent publics : ils ne contiennent aucune permission ni aucun secret. Ce rapport utilise OWASP Top 10 2021 et les familles ASVS niveau 2 comme grille de lecture ; il ne constitue pas une certification ASVS complète ni un test d'intrusion de la production.

## 1. Vulnérabilités et correctifs

| ID | Fichier / route | Description confirmée | Gravité | Référence OWASP |
|---|---|---|---|---|
| SEC-01 | `internal/handlers/catalogue.go`, `/` | `globalCfg` complet était journalisé : champs JWT, MinIO et NATS inclus. Suppression du log. L'accès effectif à ces journaux par un tiers n'est pas établi. | Haute | A09 — Security Logging and Monitoring Failures ; ASVS V7 |
| SEC-02 | `go.mod`, image worker | Go 1.26.0 : govulncheck a trouvé 27 vulnérabilités atteignables de bibliothèque standard. Minimum et image relevés à 1.26.7 ; `x/crypto` à 0.56.0. L'atteignabilité ne prouve pas une exploitation. | Haute | A06 — Vulnerable and Outdated Components ; ASVS V14 |
| SEC-03 | `/static/`, `cmd/main.go` | FileServer permettait le listing et la publication accidentelle de fichiers. Garde des fichiers réguliers, extensions d'actifs autorisées et contrat OpenAPI explicitement public. | Moyenne | A05 — Security Misconfiguration ; ASVS V14 |
| SEC-04 | `/api/books`, `/` | Détails de SQL/erreur exposés et statuts internes/score de recherche rendus publics. Erreurs génériques et DTO public partagé API/HTML ; versions de build retirées des pages publiques et du shell admin. | Moyenne | A05 ; ASVS V7/V13 |
| SEC-05 | En-têtes et templates | `unsafe-inline` pour styles, polices distantes, styles inline. CSP stricte, CSS externe et polices locales avec licences. Aucun XSS exploitable n'a été démontré. | Moyenne | A03 — Injection / A05 ; ASVS V5/V14 |
| SEC-06 | `/api/auth/login`, `/refresh`, `/logout` en mode cookie | Pas de token CSRF dédié ; protection préexistante par header personnalisé et SameSite. Ajout double-submit signé, origine et Fetch Metadata sur les mutations. | Moyenne | A01 — Broken Access Control ; ASVS V4 |
| SEC-07 | Déploiement HTTP | Le serveur seul ne garantissait pas HTTPS en production. Configuration explicite PUBLIC_ORIGIN : cookies Secure, backend loopback, redirection canonique et HSTS ; exemple proxy TLS 1.2/1.3. Le transport du déploiement réel reste à vérifier. | Haute si exposé en HTTP | A02 — Cryptographic Failures ; ASVS V9 |
| SEC-08 | Authentification et conversions numériques | Verrouillage constant, entrées login non bornées par champ et conversions potentiellement réductrices. Limites de champs, verrouillage progressif plafonné et comparaisons/conversions bornées. Aucun contournement de rôle confirmé. | Moyenne | A07 — Identification and Authentication Failures ; ASVS V2 |

Les correctifs complets sont dans les fichiers du dépôt, avec une annexe contenant les middlewares prêts à reprendre. Aucun masquage global des diagnostics gosec : 43 exceptions locales sont documentées pour les opérations de fermeture/nettoyage, SQL construit à partir de constantes, chemins d'outils opérateur, conversions positives bornées et cookies de développement. Elles ne constituent pas 43 vulnérabilités corrigées et doivent rester revues lors de changements de ces sites.

Si la configuration contenant de vrais secrets a été écrite dans des journaux, restreindre leur accès et renouveler les secrets concernés (JWT, MinIO, NATS) suivant les procédures d'exploitation. Le correctif ne supprime pas vos journaux et ne modifie pas vos secrets.

## 2. Autorisation, routes et IDOR

L'[inventaire exhaustif des routes](SECURITY_ROUTES_V1_7_2.md) indique méthode, middleware et rôle. Les routes GET acceptent HEAD selon net/http. L'inventaire inclut les routes commerciales enregistrées par helper et les shells HTML. Le contrat OpenAPI contient 123 opérations API ; 116 sont soumises à session ; ces 116 opérations (métriques incluses) sont vérifiées sans authentification par le test navigateur dédié.

Chaîne existante conservée : `AuthenticateSession` vérifie signature JWT, session active en base et correspondance utilisateur/rôle/bibliothèque ; `RequirePasswordChanged` impose le changement initial ; `RequireRoles` impose ROOT ou OWNER selon la route. Les services appliquent `library_id` aux ressources et relations (livres, ventes, clients, paiements, retours, import/revue/source). Un JWT signé mais révoqué n'est pas accepté. Le rôle ou la bibliothèque du JS ne fait pas autorité.

`/admin` et `/admin/cover-imports` livrent intentionnellement un shell public sans données métier. Le JS redirige un visiteur non connecté, mais la frontière de sécurité est l'API. Aucun middleware d'authentification n'est ajouté aux scripts : les authentifier avec un cookie imposerait une nouvelle identité ambiante et compliquerait le chargement avant connexion. C'est une option de défense en profondeur, pas une correction d'accès.

`GET /api/books/{id}/cover` est public par conception et retourne uniquement une variante publiée READY ; pas la source privée MinIO ni une couverture en attente/rejetée. Les ID séquentiels ne sont donc pas, à eux seuls, un IDOR. L'ID public nécessaire à cette URL est conservé. Les routes privées de couverture/import imposent le périmètre bibliothèque. Les tests existants couvrent propriétaire A/propriétaire B, ressources parentes et accès ROOT ; aucun IDOR supplémentaire n'a été confirmé dans la revue. Cela ne remplace pas une campagne d'intrusion avec jeux de données de production.

## 3. XSS, SQL, fichiers et CSV

- Le rendu HTML utilise `html/template`. Aucun `template.HTML`, `template.JS` ou rendu HTML par `text/template` trouvé dans les chemins applicatifs examinés. Les données restent du texte échappé ; la CSP fournit une protection supplémentaire.
- Les sinks `innerHTML` restants dans les scripts admin contenaient des constantes. Ils ont été supprimés : `replaceChildren`, `createElement`, `textContent` pour paiements/retours ; formulaire de retour fournisseur déplacé dans le template HTML. Aucun `eval` ou `insertAdjacentHTML` trouvé dans les sources clients examinées. Les bundles React générés ne sont pas réécrits manuellement.
- Les requêtes SQL utilisent des valeurs liées. Les fragments dynamiques de filtres/tri sont des constantes serveur ; les noms de tables temporaires FTS proviennent d'un compteur interne. Aucun identifiant SQL ne vient directement d'une valeur utilisateur. La syntaxe de recherche FTS est conservée et liée en paramètre ; recherche bornée à 256 caractères, repli LIKE conservé et son tri qualifié `d.id` pour éviter une ambiguïté après jointures taxonomiques (révélée par le test XSS). Le log de recherche brut est retiré.
- Uploads existants conservés : MaxBytesReader avant parsing multipart, validation réelle JPEG/PNG (signature/décodage), taille et dimensions/pixels, limites de lot et quota, stockage privé. Imports : 100 fichiers et 500 MiB par lot, enveloppe multipart bornée. Aucun SVG/HTML accepté comme image.
- Les exports CSV appliquent déjà `safeCSVCell` : préfixe apostrophe lorsque la première valeur significative est `=`, `+`, `-`, `@` après espaces/contrôles/BOM. Les tests correspondants restent obligatoires. L'import CSV ne doit jamais évaluer les formules ; les chemins examinés ne les évaluent pas.

Le mapping SQL et HTML auteur/titre est correct. Un test avec valeurs distinctes protège le DTO ; si une fiche réelle contient le titre dans le champ auteur, vérifier la donnée ou l'import concerné avant une correction ciblée. Aucune donnée existante n'est réécrite sur cette seule observation.

## 4. Transport, CSP et sessions

En local, PUBLIC_ORIGIN reste vide et HTTP fonctionne (WSL et relais inclus). En production, définir `PUBLIC_ORIGIN=https://votre-domaine` : la configuration refuse une origine HTTP, avec chemin ou identifiants, impose Secure pour le refresh cookie et lie le backend à 127.0.0.1. Le reverse proxy doit être sur le même hôte et écraser `X-Forwarded-Proto`, comme dans [l'exemple nginx](../deploy/nginx-security.conf.example). Le header d'un client distant n'est pas reconnu comme preuve de HTTPS.

Le proxy termine TLS 1.2/1.3 et redirige HTTP. Le middleware redirige aussi vers l'origine canonique configurée, sans utiliser le Host reçu, et émet `Strict-Transport-Security: max-age=31536000; includeSubDomains` sur HTTPS. **includeSubDomains engage tous les sous-domaines : leur HTTPS doit être vérifié avant activation.** Aucun déploiement ni certificat réel n'a été changé/testé par cet audit. Le TLSConfig Go impose aussi MinVersion TLS1.2, mais ListenAndServe reste HTTP derrière le proxy ; ce champ ne chiffre pas à lui seul la connexion.

CSP : `default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data: blob:; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'`. Aucun script/style inline autorisé, aucune ressource tierce chargée à l'exécution : SRI tiers sans objet. Manrope, Noto Kufi Arabic et Noto Sans Arabic sont locaux ; sources, empreintes et licences dans [static/fonts/README.md](../static/fonts/README.md). `nosniff`, Referrer-Policy, Permissions-Policy et X-Frame-Options existants sont conservés.

Refresh cookie : HttpOnly, SameSite=Strict, expiration limitée, Secure en production. Session créée à chaque login ; refresh tournant, réutilisation et révocation vérifiées en base. Cookie CSRF distinct, lisible par JS par nécessité, aléatoire signé, host-only, SameSite=Strict et préfixe __Host en production ; il n'authentifie jamais.

Toutes les mutations passent la vérification origine/Fetch Metadata. Le mode cookie déclaré par `X-Defta-Session: cookie` exige aussi `X-Defta-CSRF`, égal au cookie signé. Sans ce mode, les endpoints auth exigent un refresh token explicite dans JSON ; les API métier exigent Authorization Bearer et n'utilisent pas les cookies comme identité. Ces API n'ont donc pas d'autorité ambiante susceptible d'être exploitée par CSRF. Aucun CORS permissif n'est introduit. Ne pas ajouter à l'avenir une authentification API par cookie sans rendre le token CSRF obligatoire sur ces routes.

La limitation préexistante de 10 requêtes/minute/adresse sur login/refresh est conservée. Verrouillage après cinq échecs, puis 15/30/60 minutes selon les échecs persistants ; succès remet le compteur à zéro. Échecs auditables, messages client identiques pour utilisateur inconnu/désactivé/verrouillé. Champs login limités : username 256 octets, password 4096 octets, UTF-8 valide. Le proxy loopback rend la limite IP conservatrice/globalisée : une politique d'IP cliente de confiance distribuée est une amélioration séparée, à concevoir explicitement plutôt qu'accepter des X-Forwarded-For falsifiables.

Secrets par environnement/.env local conservé, aucun secret ajouté aux actifs ; aucune lecture ou édition du `.env` d'origine. Timeouts existants 5s ReadHeaderTimeout, 15s ReadTimeout, 30s WriteTimeout, 60s IdleTimeout conservés. Les handlers JSON/multipart appliquent leurs limites propres : ne pas ajouter une limite globale trop petite qui casserait les lots.

## 5. Vérification reproductible

Depuis le dépôt, avec Go 1.26.7+ et les outils épinglés dans le workflow :

```sh
go test -tags fts5 ./...
go test -race -tags fts5 ./...
go vet -tags fts5 ./...
govulncheck -tags fts5 ./...
gosec -tags fts5 ./...
staticcheck -tags fts5 ./...
npm run test:frontend
sh scripts/check-delivery.sh
```

Résultats locaux : Go/Go race/vet réussis ; 125 tests frontend réussis ; contrôles OpenAPI (123 opérations), accessibilité (23 dialogues), restauration (6 tests), contrats OWASP et couvertures réussis. govulncheck : 0 vulnérabilité atteignable, 0 dans les packages importés ; 1 avis module-only dans `x/crypto/openpgp`, package non importé, non maintenu et sans correctif annoncé. Ce résultat n'est pas « tous les modules sont exempts d'avis ». gosec : 0 alerte restante avec exceptions locales documentées ; staticcheck : aucune alerte. Chromium n'a pas pu être installé localement (archive de téléchargement tronquée) : les 33 parcours complets sont une gate CI, pas un succès local prétendu.

### En-têtes et accès

```sh
BASE_URL=http://127.0.0.1:8080
curl -I "$BASE_URL/login"
curl -I "$BASE_URL/"
curl -I "$BASE_URL/static/js/admin-books.js"
curl -I "$BASE_URL/static/"             # 404
curl -I "$BASE_URL/static/js/"          # 404
curl -I "$BASE_URL/static/.env"         # 404
curl -I "$BASE_URL/static/openapi.json" # 200 : contrat volontairement public
curl -i "$BASE_URL/api/admin/owners"    # 401
curl -i "$BASE_URL/api/manage/books"    # 401
curl -i -X DELETE "$BASE_URL/api/manage/books/1" # 401
curl -i -X POST "$BASE_URL/api/auth/refresh" \
  -H 'X-Defta-Session: cookie' -H 'Content-Type: application/json' -d '{}' # 403 : CSRF absent
curl -i -X POST "$BASE_URL/api/auth/login" \
  -H 'Origin: https://attacker.example' -H 'Content-Type: application/json' -d '{}' # 403
curl -i --get "$BASE_URL/" --data-urlencode 'q=<script>alert(1)</script>'
```

Vérifier CSP stricte, absence d'URL Google Fonts et de versions visibles ; la recherche doit afficher/échappper le texte sans exécuter de script. Le test Playwright protège le refus anonyme de **toutes les 116 opérations protégées**, pas seulement les exemples ci-dessus. Les tests de services continuent de vérifier refus d'accès entre bibliothèques et session révoquée ; les parcours métier doivent tous passer avant fusion.

```sh
# Sur le déploiement réel seulement, après configuration du proxy/certificat :
curl -I http://votre-domaine/login          # 308 vers HTTPS canonique
curl -I https://votre-domaine/login         # HSTS + CSP ; CSRF Secure/SameSite
openssl s_client -connect votre-domaine:443 -tls1_2 </dev/null
# TLS 1.0/1.1 doivent être refusés ; le succès d'un curl local ne prouve pas TLS.
```

Manuel navigateur : login/logout/refresh, sessions, upload/retry/remplacement/import OCR, preview, paiements/retours/exports/impression, thèmes et mobile ; aucune violation CSP attendue pour ces parcours. Essayer un titre/auteur contenant `<img src=x onerror=alert(1)>` en environnement de test puis ouvrir catalogue/admin/export : texte, aucun événement exécuté. Tester une fausse image et un CSV commençant par `=HYPERLINK(...)` en environnement de test : rejet de l'image et cellule neutralisée.

## 6. Limites et suite hors périmètre

- Pas de scan réseau de production, preuve TLS réelle, audit des droits de lecture des journaux historiques, inventaire des comptes réels ou attestation ASVS exhaustive.
- Aucun effacement automatique des journaux, rotation de secrets, migration de données, changement de politique OCR ou publication/écrasement du tag v1.7.1.
- Après fusion et gates réussies : publier une nouvelle v1.7.2 reconstruite avec Go corrigé. Les binaires v1.7.1 existants ne changent pas et restent basés sur l'ancien runtime.
- Une source map absente et des scripts moins visibles ne sont pas des barrières d'autorisation. Le build image privé/worker et ses interfaces internes nécessitent aussi leurs politiques réseau en exploitation ; une campagne dédiée d'intrusion et un examen d'infrastructure restent possibles séparément.

Références primaires : [OWASP CSRF](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html), [CSP](https://cheatsheetseries.owasp.org/cheatsheets/Content_Security_Policy_Cheat_Sheet.html), [TLS](https://cheatsheetseries.owasp.org/cheatsheets/Transport_Layer_Security_Cheat_Sheet.html), [sessions](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html), [sécurité Go](https://go.dev/doc/security/).

## 7. Code complet et câblage

Les fichiers ci-dessous sont autonomes dans le package middleware existant. Garder l'authentification et les services métier existants ; le middleware d'en-têtes ne remplace pas les rôles.

### internal/middleware/http_security.go

```go
package middleware

import (
	"defta-librairie/internal/identity"
	"net/http"
)

func SecureHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data: blob:; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if requestID, err := identity.NewID(); err == nil {
			w.Header().Set("X-Request-ID", requestID)
		}
		if len(r.URL.Path) >= len("/api/auth/") && r.URL.Path[:len("/api/auth/")] == "/api/auth/" {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}
```

### internal/middleware/csrf.go

```go
package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
)

// CSRF protects browser cookie session endpoints. Other write APIs require an
// explicit bearer token and never authenticate using ambient cookies.
// A signed, host-only random double-submit cookie is reinforced with same-origin and
// Fetch Metadata checks. It grants no authentication or business permission.
func CSRF(secure bool, next http.Handler, signingKey []byte) http.Handler {
	valid := func(token string) bool {
		parts := strings.Split(token, ".")
		if len(parts) != 2 || len(parts[0]) != 43 {
			return false
		}
		mac := hmac.New(sha256.New, signingKey)
		mac.Write([]byte(parts[0]))
		expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
		return subtle.ConstantTimeCompare([]byte(expected), []byte(parts[1])) == 1
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := "defta_csrf"
		if secure {
			name = "__Host-defta_csrf"
		}
		safe := r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions
		if !safe {
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				csrfError(w)
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				expectedScheme := "http"
				if secure || r.TLS != nil {
					expectedScheme = "https"
				}
				parsed, err := url.Parse(origin)
				if err != nil || parsed.User != nil || parsed.Host != r.Host || parsed.Scheme != expectedScheme || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
					csrfError(w)
					return
				}
			}
			if strings.EqualFold(r.Header.Get("X-Defta-Session"), "cookie") {
				cookie, err := r.Cookie(name)
				token := r.Header.Get("X-Defta-CSRF")
				if err != nil || !valid(token) || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(token)) != 1 {
					csrfError(w)
					return
				}
			}
		}
		if safe && (r.URL.Path == "/login" || r.URL.Path == "/admin" || r.URL.Path == "/admin/cover-imports") {
			cookie, err := r.Cookie(name)
			if err != nil || !valid(cookie.Value) {
				random := make([]byte, 32)
				if _, err := rand.Read(random); err != nil {
					http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
					return
				}
				nonce := base64.RawURLEncoding.EncodeToString(random)
				mac := hmac.New(sha256.New, signingKey)
				mac.Write([]byte(nonce))
				token := nonce + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
				// #nosec G124 -- Production uses Secure and __Host prefix; CSRF cookie intentionally readable for double-submit header, not an authentication cookie.
				http.SetCookie(w, &http.Cookie{Name: name, Value: token, Path: "/", Secure: secure, SameSite: http.SameSiteStrictMode})
			}
		}
		next.ServeHTTP(w, r)
	})
}

func csrfError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"csrf_failed","message":"Request origin or CSRF token rejected"}`))
}
```

### internal/middleware/production_https.go

```go
package middleware

import (
	"net"
	"net/http"
)

// ProductionHTTPS accepts forwarded HTTPS only from the loopback reverse
// proxy. Production configuration also binds the backend to loopback.
func ProductionHTTPS(origin string, next http.Handler) http.Handler {
	if origin == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		ip := net.ParseIP(host)
		secure := r.TLS != nil || (err == nil && ip != nil && ip.IsLoopback() && r.Header.Get("X-Forwarded-Proto") == "https")
		if !secure {
			// #nosec G710 -- Canonical HTTPS origin validated at startup; request Host never controls destination.
			http.Redirect(w, r, origin+r.URL.RequestURI(), http.StatusPermanentRedirect)
			return
		}
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}
```

### internal/middleware/static_files.go

```go
package middleware

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// StaticFiles serves regular browser assets only; directories, dotfiles and
// source maps are never exposed, even when added accidentally to static/.
func StaticFiles(root fs.FS) http.Handler {
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if !fs.ValidPath(name) {
			http.NotFound(w, r)
			return
		}
		for _, component := range strings.Split(name, "/") {
			if strings.HasPrefix(component, ".") {
				http.NotFound(w, r)
				return
			}
		}
		switch strings.ToLower(path.Ext(name)) {
		case ".js", ".css", ".svg", ".png", ".jpg", ".jpeg", ".webp", ".ico", ".woff", ".woff2", ".ttf":
		default:
			if name != "openapi.json" {
				http.NotFound(w, r)
				return
			}
		}
		info, err := fs.Stat(root, name)
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}
```

### internal/middleware/authentication.go

```go
package middleware

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type AccessSessionValidator interface {
	IsActive(context.Context, string, string, models.UserRole, string, time.Time) (bool, error)
}

func Authenticate(tokens *auth.TokenManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeUnauthorized(w)
			return
		}
		claims, err := tokens.Parse(parts[1])
		if err != nil {
			writeUnauthorized(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.ContextWithClaims(r.Context(), claims)))
	})
}

func AuthenticateSession(tokens *auth.TokenManager, sessions AccessSessionValidator, next http.Handler) http.Handler {
	return Authenticate(tokens, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok || claims.SessionID == "" {
			writeUnauthorized(w)
			return
		}
		active, err := sessions.IsActive(r.Context(), claims.SessionID, claims.Subject,
			claims.Role, claims.LibraryID, time.Now().UTC())
		if err != nil {
			writeSessionUnavailable(w)
			return
		}
		if !active {
			writeUnauthorized(w)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("WWW-Authenticate", `Bearer realm="defta-librairie"`)
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized", "message": "Authentication required"})
}

func writeSessionUnavailable(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "session_validation_failed", "message": "Session validation unavailable"})
}
```

### internal/middleware/authorization.go

```go
package middleware

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"encoding/json"
	"net/http"
)

// RequireRoles autorise uniquement les rôles explicitement déclarés.
// Authenticate doit être exécuté avant ce middleware.
func RequireRoles(next http.Handler, allowed ...models.UserRole) http.Handler {
	roles := make(map[models.UserRole]struct{}, len(allowed))
	for _, role := range allowed {
		roles[role] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			writeUnauthorized(w)
			return
		}
		if _, ok = roles[claims.Role]; !ok {
			writeForbidden(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequirePasswordChanged(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			writeUnauthorized(w)
			return
		}
		if claims.PasswordChangeRequired {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "password_change_required", "message": "Password change required before accessing this resource",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireLibraryAccess applique l'isolation des données par librairie.
// SUPER_ADMIN_ROOT peut accéder à toutes les librairies. OWNER_LIBRARY ne
// peut accéder qu'à l'identifiant library_id porté par son JWT.
func RequireLibraryAccess(next http.Handler, libraryID func(*http.Request) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			writeUnauthorized(w)
			return
		}

		switch claims.Role {
		case models.RoleSuperAdminRoot:
			next.ServeHTTP(w, r)
		case models.RoleOwnerLibrary:
			if claims.LibraryID == "" || libraryID == nil || libraryID(r) != claims.LibraryID {
				writeForbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		default:
			writeForbidden(w)
		}
	})
}

func writeForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   "forbidden",
		"message": "Insufficient permissions",
	})
}
```

### internal/handlers/public_book.go

```go
package handlers

import (
	"database/sql"
	"defta-librairie/internal/models"
	"strconv"
)

// PublicBook deliberately excludes internal state, FTS scores, library and
// moderation metadata. ID is retained to locate the public READY derivative.
type PublicBook struct {
	ID        int                `json:"id"`
	Title     string             `json:"title"`
	Auteur    models.StringField `json:"auteur"`
	Editeur   models.StringField `json:"editeur"`
	Price     float64            `json:"price"`
	Volume    int                `json:"volume"`
	Tags      models.StringField `json:"tags"`
	Categorie models.StringField `json:"categorie"`
	CoverURL  models.StringField `json:"coverUrl"`
}

func publicBook(b models.Book) PublicBook {
	return PublicBook{ID: b.ID, Title: b.Title, Auteur: b.Auteur, Editeur: b.Editeur, Price: b.Price, Volume: b.Volume, Tags: b.Tags, Categorie: b.Categorie,
		CoverURL: models.StringField{NullString: sql.NullString{String: "/api/books/" + strconv.Itoa(b.ID) + "/cover?variant=large&format=jpeg", Valid: true}}}
}
```

### Câblage dans cmd/main.go

```go
// PUBLIC_ORIGIN validée par config.Load ; vide uniquement pour usage local HTTP.
addr := ":" + cfg.Port
if cfg.PublicOrigin != "" { addr = "127.0.0.1:" + cfg.Port }
server := &http.Server{
 Addr: addr,
 Handler: middleware.ProductionHTTPS(cfg.PublicOrigin,
  middleware.SecureHTTP(middleware.CSRF(cfg.AuthCookieSecure,
   observability.Wrap(mux), []byte(cfg.JWTSecret)))),
 ReadHeaderTimeout: 5 * time.Second,
 ReadTimeout: 15 * time.Second,
 WriteTimeout: 30 * time.Second,
 IdleTimeout: 60 * time.Second,
 TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12},
}
// Imports additionnels : crypto/tls, os ; imports middleware/net/http/time existants.
mux.Handle("/static/", http.StripPrefix("/static/", middleware.StaticFiles(os.DirFS("static"))))
// Garder le câblage actuel de chaque API :
protected := middleware.AuthenticateSession(tokens, sessionRepository,
 middleware.RequirePasswordChanged(middleware.RequireRoles(handler,
  models.RoleSuperAdminRoot, models.RoleOwnerLibrary)))
// Les services contrôlent ensuite la bibliothèque et la ressource.
_ = protected // illustration ; les vrais handlers sont déjà enregistrés dans main.go.
_ = server
```

Le câblage ci-dessus illustre les imports et la composition ; les variables `tokens`, `sessionRepository`, `observability`, `handler` et `cfg` sont les dépendances déjà instanciées. Le fichier réel `cmd/main.go` contient le câblage exécutable complet de toutes les routes.

### Correctifs JS, templates et fuites

Les fichiers complets prêts à utiliser sont `static/js/admin-http.js`, `admin-payments.js`, `admin-returns.js`, `admin-supplier-returns.js`, `templates/admin.html`, `base.html`, `catalogue.html`, `login.html` et les deux partials livres. Ils conservent les sélecteurs existants. Chaque réponse HTML passe par html/template ; les DOM clients utilisent du texte. Exemple minimal complet de remplacement d'une ligne dynamique :

```js
function appendTextCell(row, value) {
 const cell = document.createElement("td");
 cell.textContent = value == null ? "" : String(value);
 row.appendChild(cell);
 return cell;
}
function replaceTextRows(tableBody, values) {
 tableBody.replaceChildren();
 for (const value of values) {
  const row = document.createElement("tr");
  appendTextCell(row, value);
  tableBody.appendChild(row);
 }
}
```

Cet exemple n'est pas ajouté comme helper générique au produit. Les changements exacts appliqués dans les modules sont visibles dans le diff ; aucun échappement artisanal de HTML n'est nécessaire avec textContent. Le client réel lit le cookie CSRF signé et l'envoie uniquement dans authJSON, avec X-Defta-Session: cookie ; les requêtes métier gardent leur Bearer existant.

SEC-01 : suppression pure du log globalCfg. SEC-04 : DTO ci-dessus, API renvoie une erreur générique `{"error":"internal_error","message":"Catalogue temporarily unavailable"}` sans details, SSR `http.Error(w, "Erreur de rendu", 500)` sans contenu d'erreur. SEC-02 : go.mod/Go.sum et Dockerfile corrigés, contrôlés par les trois analyseurs. SEC-08 : login_service.go contient la validation UTF-8/longueur et le verrouillage progressif complet ; tests dans login_service_test.go. Les valeurs SQL, le neutraliseur CSV et la validation d'images corrects préexistants ne sont pas réécrits.
