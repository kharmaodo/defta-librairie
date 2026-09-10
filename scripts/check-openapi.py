#!/usr/bin/env python3
"""Dependency-free route/reference/example checks; not a full OAS validator."""
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
doc = json.loads((ROOT / "static/openapi.json").read_text())
assert doc["openapi"] == "3.0.3"
paths = doc["paths"]
expected = {(m.lower(), p) for m, p in re.findall(
    r'mux\.Handle(?:Func)?\("(GET|POST|PUT|PATCH|DELETE) (/api/[^" ]+)"',
    (ROOT / "cmd/main.go").read_text())}
actual = {(m, p) for p, methods in paths.items() for m in methods}
assert expected == actual, ("Route drift", expected - actual, actual - expected)

def resolve(ref):
    assert ref.startswith("#/"), ref
    node = doc
    for part in ref[2:].split("/"):
        node = node[part]
    return node

def walk(node):
    if isinstance(node, dict):
        if "$ref" in node:
            resolve(node["$ref"])
        for value in node.values():
            walk(value)
    elif isinstance(node, list):
        for value in node:
            walk(value)

walk(doc)

def validate(value, schema):
    if "$ref" in schema:
        return validate(value, resolve(schema["$ref"]))
    if value is None and schema.get("nullable"):
        return
    for child in schema.get("allOf", []):
        validate(value, child)
    if "enum" in schema:
        assert value in schema["enum"], (value, schema)
    kind = schema.get("type")
    if kind == "object":
        assert isinstance(value, dict), value
        assert set(schema.get("required", [])) <= value.keys(), (value, schema)
        for key, item in value.items():
            if key in schema.get("properties", {}):
                validate(item, schema["properties"][key])
    elif kind == "array":
        assert isinstance(value, list)
        assert len(value) >= schema.get("minItems", 0)
        assert len(value) <= schema.get("maxItems", float("inf"))
        for item in value:
            validate(item, schema["items"])
    elif kind == "string":
        assert isinstance(value, str), value
        assert len(value) >= schema.get("minLength", 0)
        assert len(value) <= schema.get("maxLength", float("inf"))
    elif kind in ("integer", "number"):
        assert not isinstance(value, bool) and isinstance(value, int if kind == "integer" else (int, float)), value
        if "minimum" in schema:
            assert value > schema["minimum"] if schema.get("exclusiveMinimum") else value >= schema["minimum"]
        if "maximum" in schema:
            assert value <= schema["maximum"]
    elif kind == "boolean":
        assert isinstance(value, bool)

ids = set()
for path, methods in paths.items():
    for method, op in methods.items():
        assert op["operationId"] not in ids
        ids.add(op["operationId"])
        params = op.get("parameters", [])
        assert len({(p["name"], p["in"]) for p in params}) == len(params)
        for name in re.findall(r"\{(\w+)\}", path):
            assert any(p["name"] == name and p["in"] == "path" and p.get("required") for p in params)
        assert op["responses"] and op["x-codeSamples"]
        for media in op.get("requestBody", {}).get("content", {}).values():
            validate(media["example"], media["schema"])
print(f"OK: {len(actual)} operations; references, parameters and request examples checked.")
