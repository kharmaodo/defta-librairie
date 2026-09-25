#!/usr/bin/env python3
"""Contrôle statique du socle OWASP sécurité/résilience v1.6."""

from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]
errors: list[str] = []


def read(relative: str) -> str:
    path = ROOT / relative
    if not path.is_file():
        errors.append(f"fichier absent : {relative}")
        return ""
    return path.read_text(encoding="utf-8")


backlog = read("BACKLOG.md")
owasp_backlog = read("docs/BACKLOG_OWASP_V1_6.md")
baseline = read("docs/OWASP_SECURITY_BASELINE_V1_6.md")
auth_load_doc = read("docs/AUTHENTICATION_LOAD_TEST_V1_6.md")
authorization_doc = read("docs/AUTHORIZATION_CONCURRENCY_TEST_V1_6.md")
resource_doc = read("docs/RESOURCE_LIMITS_V1_6.md")
integrity_doc = read("docs/BUSINESS_INTEGRITY_CONCURRENCY_V1_6.md")
auth_load = read("tests/load/auth-rate-limit.js")
rate_test = read("internal/middleware/rate_limit_test.go")
commercial_test = read("cmd/commercial_http_test.go")
api = read("internal/handlers/api.go")

for token in (
    "## Backlog v1.6.0 — sécurité OWASP et résilience sous charge",
    "| 1 | Baseline de sécurité | Réalisé",
    "| 2 | Authentification sous charge | Réalisé",
    "| 3 | Autorisation concurrente | Réalisé",
    "| 4 | Limites de ressources | Réalisé",
    "| 5 | Intégrité métier concurrente | Réalisé",
    "| 6 | Observabilité et gate de release |",
):
    if token not in backlog:
        errors.append(f"backlog v1.6 incomplet : {token}")

for document in (baseline, auth_load_doc, authorization_doc, resource_doc, integrity_doc):
    if "v1.6" not in document:
        errors.append("contrat OWASP v1.6 absent ou incomplet")

for token in (
    "AUTH_LOAD_ALLOW !== 'isolated'",
    "AUTH_LOAD_BASE_URL",
    "auth_rate_limited",
    "Retry-After",
):
    if token not in auth_load:
        errors.append(f"scénario de charge auth incomplet : {token}")

for token in (
    "TestRateLimiterEnforcesConcurrentLimitPerClient",
    "TestRateLimiterSeparatesClients",
):
    if token not in rate_test:
        errors.append(f"test de limite absent : {token}")

for token in (
    "TestCommercialHTTPRejectsConcurrentCrossLibraryAccess",
    "TestCommercialHTTPConfirmsSaleOnlyOnceUnderConcurrency",
):
    if token not in commercial_test:
        errors.append(f"test métier/confinement absent : {token}")

for token in ("maxAPISearchLength = 256", "normalizeAPISearch"):
    if token not in api:
        errors.append(f"borne de recherche catalogue absente : {token}")

if "| 6 | US-6 — Observabilité et gate de release |" not in owasp_backlog:
    errors.append("US-6 absente du contrat détaillé")

if errors:
    print("ÉCHEC : socle OWASP v1.6 incomplet.", file=sys.stderr)
    for error in errors:
        print(f"- {error}", file=sys.stderr)
    raise SystemExit(1)

print("OK : socle OWASP v1.6, tests concurrents, limites et contrats contrôlés.")
