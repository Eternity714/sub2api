"""Check GRS.AI PRD acceptance IDs against first-party test references."""

import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
PRD = ROOT / "docs/superpowers/specs/2026-09-21-grsai-delivery-modes-prd.md"
DECLARATION = re.compile(r"^\d+\.\s+\*\*(AC-\d+)\*\*", re.MULTILINE)
REFERENCE = re.compile(r"\bAC-\d+(?:\.\d+)?\b")


def main() -> int:
    declared = set(DECLARATION.findall(PRD.read_text(encoding="utf-8")))
    referenced = set()
    for directory, pattern in ((ROOT / "backend/internal", "*.go"), (ROOT / "test", "*.py")):
        for path in directory.rglob(pattern):
            for line in path.read_text(encoding="utf-8").splitlines():
                if "@covers" in line:
                    referenced.update(REFERENCE.findall(line))

    if not declared or declared != referenced:
        print(f"GRS.AI AC traceability failed: declared={len(declared)}, referenced={len(referenced)}")
        print(f"orphaned={sorted(declared - referenced)}, ghost={sorted(referenced - declared)}")
        return 1
    print(f"GRS.AI AC traceability passed: {len(declared)} declared and referenced")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
