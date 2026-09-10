#!/usr/bin/env python3
"""Check static admin dialog names, close labels, error roles and table headers.
This is a structural check, not a browser or accessibility conformance audit.
"""
from html.parser import HTMLParser
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1]


class Check(HTMLParser):
    def __init__(self):
        super().__init__()
        self.ids = set()
        self.refs = []
        self.dialogs = 0
        self.errors = []
        self.button = None

    def handle_starttag(self, tag, attributes):
        a = dict(attributes)
        if "id" in a:
            if a["id"] in self.ids:
                self.errors.append("Duplicate ID: " + a["id"])
            self.ids.add(a["id"])
        if tag == "dialog":
            self.dialogs += 1
            if a.get("aria-labelledby"):
                self.refs.extend(a["aria-labelledby"].split())
            elif not a.get("aria-label"):
                self.errors.append("Dialog without accessible name")
        if "alert" in a.get("class", "").split() and a.get("role") != "alert":
            self.errors.append("Error message without alert role: " + a.get("id", "anonymous"))
        if tag == "th" and a.get("scope") not in ("col", "row", "colgroup", "rowgroup"):
            self.errors.append("Table header without scope")
        if tag == "button":
            self.button = [a, ""]

    def handle_data(self, text):
        if self.button is not None:
            self.button[1] += text

    def handle_endtag(self, tag):
        if tag == "button" and self.button is not None:
            attributes, text = self.button
            if text.strip() == "×" and not attributes.get("aria-label"):
                self.errors.append("Icon-only close button without name")
            self.button = None


check = Check()
check.feed((ROOT / "templates/admin.html").read_text())
script = (ROOT / "static/js/admin-supplier-returns.js").read_text()
markup = re.search(r"root\.innerHTML\s*=\s*`([\s\S]*?)`;", script)
assert markup, "Supplier return markup not found; adapt this checker"
check.feed(markup[1])
check.close()
check.errors.extend("Unresolved dialog title: " + ref for ref in check.refs if ref not in check.ids)
assert check.dialogs > 0, "No dialog checked"
assert not check.errors, "\n".join(check.errors)
print(f"OK: {check.dialogs} named dialogs; close buttons, alerts and table headers checked.")
