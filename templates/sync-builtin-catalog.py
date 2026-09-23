#!/usr/bin/env python3
"""Regenerates templates/templates-data.js from templates/templates.json.

pages/dashboard.html and pages/event.html load templates-data.js before
template-engine.js, so it seeds window.WEDDINGHUB_BUILTIN_TEMPLATES: the list the
studio renders from before /api/templates answers, and the only list when the API
is unreachable. It has to carry the same entries and archived flags as the JSON.
backend/api/catalog_builtin_test.go fails if the two ever diverge.

Run from anywhere: python3 templates/sync-builtin-catalog.py
"""
import json
from pathlib import Path

HERE = Path(__file__).resolve().parent
SRC = HERE / "templates.json"
OUT = HERE / "templates-data.js"

with SRC.open(encoding="utf-8") as fh:
    templates = json.load(fh)

blocks = []
for tpl in templates:
    body = json.dumps(tpl, indent=2, ensure_ascii=False)
    blocks.append("\n".join("  " + line if line else line for line in body.split("\n")))

with OUT.open("w", encoding="utf-8") as fh:
    fh.write("window.WEDDINGHUB_BUILTIN_TEMPLATES = [\n\n")
    fh.write(",\n\n".join(blocks))
    fh.write("\n];\n")

active = sum(1 for t in templates if not t.get("archived"))
print("wrote %s: %d entries (%d active, %d archived)" % (
    OUT.name, len(templates), active, len(templates) - active))
