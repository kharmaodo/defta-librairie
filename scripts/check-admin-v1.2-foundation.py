#!/usr/bin/env python3
"""Protect the measured foundation agreed for the v1.2 admin dashboard."""

from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
HTML_PATH = ROOT / "templates/admin.html"
CSS_PATH = ROOT / "static/css/admin.css"
JS_DIRECTORY = ROOT / "static/js"

HTML = HTML_PATH.read_text(encoding="utf-8")
CSS = CSS_PATH.read_text(encoding="utf-8")
ADMIN_SCRIPTS = sorted(JS_DIRECTORY.glob("admin-*.js"))
SUPPLIER_RETURNS_JAVASCRIPT = (
    JS_DIRECTORY / "admin-supplier-returns.js"
).read_text(encoding="utf-8")


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"ERREUR fondation dashboard v1.2 : {message}")


protected_sections = (
    "dashboard-overview", "dashboard-summary", "library-settings-panel", "csv-exports-panel",
    "business-alerts-panel", "commercial-statistics-panel", "suppliers-panel",
    "purchases-panel", "customers-panel", "dashboard-indicators",
    "cash-registers-panel", "payments-panel", "sales-panel",
    "customer-returns-panel", "inventory-panel", "owners-section",
    "tags-panel", "books-panel", "audit-panel", "sessions-panel",
    "supplier-returns-panel", "book-submissions-panel",
)
for section_id in protected_sections:
    require(f'id="{section_id}"' in HTML, f"section protégée absente : #{section_id}")

protected_controls = (
    "role-badge", "change-password-button", "logout-button",
    "dashboard-main", "dashboard-error", "book-total", "sale-total",
    "owner-total", "audit-total", "session-total",
)
for control_id in protected_controls:
    require(f'id="{control_id}"' in HTML, f"contrôle protégé absent : #{control_id}")

for attribute in (
    "data-dashboard-nav", "data-dashboard-menu-toggle",
    "data-dashboard-nav-group", "data-dashboard-nav-link",
    "data-dashboard-nav-backdrop",
):
    require(attribute in HTML, f"attribut de navigation absent : {attribute}")

require(HTML.count("<dialog ") == 23, "vingt-trois dialogues statiques sont attendus")
require(
    "<dialog " in SUPPLIER_RETURNS_JAVASCRIPT,
    "le dialogue dynamique de retour fournisseur est absent",
)
require(HTML.count('src="/static/js/admin-') >= 20, "au moins vingt modules admin sont attendus")
require('src="/static/js/admin-summary.js"' in HTML, "module de synthèse absent")

for breakpoint in (780, 900, 1100):
    require(
        f"@media(max-width:{breakpoint}px)" in CSS,
        f"breakpoint de référence absent : {breakpoint}px",
    )

sizes = {
    "HTML": HTML_PATH.stat().st_size,
    "CSS": CSS_PATH.stat().st_size,
    "JavaScript admin": sum(path.stat().st_size for path in ADMIN_SCRIPTS),
}
budgets = {"HTML": 70_000, "CSS": 35_000, "JavaScript admin": 190_000}
for asset, size in sizes.items():
    require(size <= budgets[asset], f"budget {asset} dépassé : {size} > {budgets[asset]} octets")

print(
    "OK: fondation v1.2, sections et dialogues protégés, modules admin, "
    "breakpoints 780/900/1100px et budgets statiques contrôlés."
)
print(
    f"Mesures: HTML={sizes['HTML']} o, CSS={sizes['CSS']} o, "
    f"JavaScript admin={sizes['JavaScript admin']} o."
)
