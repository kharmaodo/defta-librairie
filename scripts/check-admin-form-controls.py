#!/usr/bin/env python3
"""Check the shared form-control visual contract introduced for v1.3."""

from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CSS = (ROOT / "static/css/admin.css").read_text(encoding="utf-8")

errors: list[str] = []

if CSS.count("--input-border-color:") < 2:
    errors.append("--input-border-color must be defined in light and dark themes")
if "--input-border:" in CSS:
    errors.append("legacy --input-border variable remains")
if CSS.count("var(--input-border-color)") < 6:
    errors.append("not all form families use the shared input border")

required = {
    "hover": ":hover:not(:disabled)",
    "focus": ":focus-visible",
    "disabled": ":disabled",
    "hover token": "--input-border-hover-color",
    "disabled background": "--input-disabled-background",
    "focus ring": "--input-focus-ring",
    "login": ".login-card input",
    "dialog forms": ".form-grid input,.form-grid select,.form-grid textarea",
    "dashboard filters": ".filters input,.filters select",
    "summary filters": ".dashboard-summary-filters input,.dashboard-summary-filters select",
    "return lines": ".return-line input",
    "sale lines": ".sale-line input,.sale-line select",
    "inline search": ".inline-search input",
}
for label, fragment in required.items():
    if fragment not in CSS:
        errors.append(f"missing {label} guarantee: {fragment}")

if errors:
    raise SystemExit("ÉCHEC : contrat visuel des champs incomplet.\n- " + "\n- ".join(errors))

print("OK: bordures et états des champs couverts en thèmes clair et sombre.")
