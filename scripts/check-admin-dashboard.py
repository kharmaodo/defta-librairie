#!/usr/bin/env python3
"""Validate the stable structural contract of the v1.1 admin dashboard."""

from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
HTML = (ROOT / "templates/admin.html").read_text(encoding="utf-8")
CSS = (ROOT / "static/css/admin.css").read_text(encoding="utf-8")
JAVASCRIPT = (ROOT / "static/js/admin-navigation.js").read_text(encoding="utf-8")


def require(condition: bool, message: str) -> None:
    if not condition:
        raise SystemExit(f"ERREUR dashboard admin : {message}")


require(HTML.count("data-dashboard-nav-group") == 5, "cinq groupes thématiques attendus")
require(HTML.count("data-dashboard-nav-link") == 22, "vingt-deux liens de rubrique attendus")
require(
    '<footer class="admin-footer">Defta Librairie · {{.Version}} · {{.BuildDate}}</footer>' in HTML,
    "le contenu contractuel du footer a changé",
)

for target_id in (
    "dashboard-overview", "commercial-statistics-panel", "business-alerts-panel",
    "csv-exports-panel", "books-panel", "tags-panel", "categories-panel", "publishers-panel", "inventory-panel",
    "book-submissions-panel",
    "customers-panel", "sales-panel", "cash-registers-panel", "payments-panel",
    "suppliers-panel", "purchases-panel", "supplier-returns-panel",
    "customer-returns-panel", "library-settings-panel", "owners-section",
    "sessions-panel", "audit-panel",
):
    require(f'id="{target_id}"' in HTML, f"ancre absente : #{target_id}")

for section in (
    "Design tokens", "Reset, global elements and accessibility helpers",
    "Dashboard layout and header", "Thematic navigation and submenus",
    "Main content, panels and metrics", "Tables, row actions and pagination",
    "Dialogs and forms", "Responsive dashboard and mobile navigation",
    "Print modes", "Keyboard focus and reduced motion",
):
    require(section in CSS, f"section CSS absente : {section}")

require(len(CSS.splitlines()) >= 500, "admin.css semble minifié")
require(all(len(line) <= 200 for line in CSS.splitlines()), "admin.css contient une ligne minifiée")
require(
    "aria-current" in JAVASCRIPT and '"location"' in JAVASCRIPT,
    "l’état actif accessible est absent",
)

print("OK: dashboard admin, 5 groupes, 22 liens, ancres, footer et CSS commentée contrôlés.")
