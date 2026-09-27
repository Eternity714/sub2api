#!/usr/bin/env python3
"""Check GRS.AI PRD AC IDs against actual test declarations."""

import json
import pathlib
import re
import sys


ROOT = pathlib.Path(__file__).resolve().parents[1]
PRD = ROOT / "docs/superpowers/specs/2026-09-24-grsai-durable-delivery-prd.md"
MAPPING = ROOT / "test/grsai_ac_traceability.json"


def main():
    expected = set(re.findall(r"^\|\s*(GRSAI-DM-AC-\d{2})\s*\|", PRD.read_text(encoding="utf-8"), re.MULTILINE))
    mapping = json.loads(MAPPING.read_text(encoding="utf-8"))
    errors = []
    for ac_id in sorted(expected - mapping.keys()):
        errors.append("unmapped " + ac_id)
    for ac_id in sorted(mapping.keys() - expected):
        errors.append("unknown " + ac_id)
    checked = 0
    for ac_id, references in mapping.items():
        if not references:
            errors.append("empty mapping " + ac_id)
        for reference in references:
            path_text, separator, name = reference.partition("::")
            path = ROOT / path_text
            if not separator or not path.is_file() or not re.fullmatch(r"[A-Za-z_][A-Za-z_0-9]*", name):
                errors.append("invalid reference " + reference)
                continue
            source = path.read_text(encoding="utf-8")
            if not re.search(r"\b(?:func|def)\s+" + re.escape(name) + r"\s*\(", source):
                errors.append("missing test " + reference)
            checked += 1
    print(json.dumps({"ac_ids": len(expected), "test_references": checked, "errors": errors}, ensure_ascii=True))
    return 1 if errors or len(expected) != 15 else 0


if __name__ == "__main__":
    sys.exit(main())
