#!/usr/bin/env python3
"""Validate the consolidated Playwright coverage contract for dashboard v1.2."""

from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parent.parent
BROWSER = ROOT / "tests" / "browser"

REQUIRED_SPECS = {
    "accessibility.spec.cjs": ("keyboard", "dialog", "thematic navigation"),
    "admin-dashboard.spec.cjs": ("thematic structure", "stable section targets"),
    "auth.spec.cjs": ("invalid login", "logout", "parallel protected requests"),
    "commercial-lifecycle.spec.cjs": ("confirmed/cancelled sale",),
    "customer-return-lifecycle.spec.cjs": ("refund and credit note",),
    "experience-modes.spec.cjs": ("mobile dark dashboard", "reduced motion"),
    "exports-printing.spec.cjs": ("CSV exports", "receipts print"),
    "payment-lifecycle.spec.cjs": ("cash, mobile money and card",),
    "performance.spec.cjs": ("coalesces concurrent profile reads",),
    "procurement-lifecycle.spec.cjs": ("weighted average cost",),
    "responsive.spec.cjs": ("const widths = [390, 768, 1024, 1440];",),
    "supplier-return-lifecycle.spec.cjs": ("supplier return",),
    "theme.spec.cjs": ("theme is accessible and persists",),
}

errors = []
for name, markers in REQUIRED_SPECS.items():
    path = BROWSER / name
    if not path.is_file():
        errors.append(f"scénario absent : {path.relative_to(ROOT)}")
        continue
    source = path.read_text(encoding="utf-8")
    for marker in markers:
        if marker not in source:
            errors.append(f"garantie absente dans {name} : {marker}")

skip_pattern = re.compile(r"\b(?:test|describe)\.(?:skip|fixme)\s*\(")
for path in sorted(BROWSER.glob("*.spec.cjs")):
    source = path.read_text(encoding="utf-8")
    if skip_pattern.search(source):
        errors.append(f"test ignoré ou neutralisé : {path.relative_to(ROOT)}")

config = (ROOT / "playwright.config.cjs").read_text(encoding="utf-8")
for contract in ("workers: 1", "retries: 0", "browserName: 'chromium'"):
    if contract not in config:
        errors.append(f"contrat Playwright absent : {contract}")

if errors:
    print("ÉCHEC : couverture navigateur v1.2 incomplète.", file=sys.stderr)
    for error in errors:
        print(f"- {error}", file=sys.stderr)
    raise SystemExit(1)

count = sum(
    len(re.findall(r"(?m)^\s*test\s*\(", path.read_text(encoding="utf-8")))
    for path in BROWSER.glob("*.spec.cjs")
)
print(
    "OK : matrice navigateur v1.2 contrôlée, "
    f"{len(REQUIRED_SPECS)} fichiers et {count} déclarations de scénarios sans skip."
)
