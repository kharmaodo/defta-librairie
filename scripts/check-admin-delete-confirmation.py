#!/usr/bin/env python3
"""Check the destructive confirmation contract for v1.3."""

from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
HTML = (ROOT / "templates/admin.html").read_text(encoding="utf-8")
COMPONENT = (ROOT / "static/js/admin-delete-confirmation.js").read_text(encoding="utf-8")

errors: list[str] = []

required_markup = (
    'id="delete-confirmation-dialog"',
    'id="delete-confirmation-form"',
    'data-delete-subject',
    'data-delete-expected',
    'name="confirmation"',
    'data-delete-error',
    'data-delete-cancel',
    'data-delete-confirm disabled',
    'aria-labelledby="delete-confirmation-title"',
    'aria-describedby="delete-confirmation-warning"',
)
for fragment in required_markup:
    if fragment not in HTML:
        errors.append(f"markup absent : {fragment}")

component_script = 'src="/static/js/admin-delete-confirmation.js"'
if component_script not in HTML:
    errors.append("module de confirmation absent de la page")
else:
    position = HTML.index(component_script)
    for module in ("admin-books.js", "admin-sales.js", "admin-tags.js"):
        if position > HTML.index(f'src="/static/js/{module}"'):
            errors.append(f"module de confirmation chargé après {module}")

required_logic = (
    "view.input.value !== active.expected",
    "view.form.reset()",
    "view.dialog.addEventListener('cancel'",
    "active.busy",
    "current?.trigger?.focus?.()",
    "await active.execute()",
)
for fragment in required_logic:
    if fragment not in COMPONENT:
        errors.append(f"garantie du composant absente : {fragment}")

modules = {
    "livre": ("static/js/admin-books.js", "expected: book.title", 'data-action === "delete-book"'),
    "tag": ("static/js/admin-tags.js", "expected: tag.name", "api/manage/tags"),
    "vente": ("static/js/admin-sales.js", "expected: sale.reference", 'data-action === "delete-sale"'),
}
for label, (path, expected, action) in modules.items():
    source = (ROOT / path).read_text(encoding="utf-8")
    if "DeftaDeleteConfirmation.run" not in source or expected not in source or action not in source:
        errors.append(f"suppression {label} non migrée vers le composant commun")

doc = (ROOT / "docs/ADMIN_DELETE_SAFETY_V1_3.md").read_text(encoding="utf-8")
for excluded in ("Désactiver", "Révoquer", "Annuler ou réceptionner"):
    if excluded not in doc:
        errors.append(f"exclusion métier non documentée : {excluded}")

if errors:
    raise SystemExit("ÉCHEC : contrat de suppression v1.3 incomplet.\n- " + "\n- ".join(errors))

print("OK: dialogue commun et suppressions livre, tag et brouillon de vente contrôlés.")
