#!/usr/bin/env bash
# Enforces docs/adr-0036-downstream-cross-schema-reads.md: an app's non-test Go
# and SQL may name another app's schema only along recipes <- mealplans <-
# shoppinglist, and may write to it only for the listed exceptions.
# Test files are exempt (they seed fixtures across schemas).
set -euo pipefail

# Paths below are relative to api/.
cd "$(dirname "$0")/.."

python3 - <<'PY'
import os
import re
import sys

# app -> schemas it may read or hold foreign keys onto.
ALLOWED = {
    "mealplans": {"recipes"},
    "shoppinglist": {"recipes", "mealplans"},
}
# (app, schema, table) writes into another app's schema.
ALLOWED_WRITES = {
    ("shoppinglist", "mealplans", "plan_meals"),  # SetItemExcluded flag
    # 00001_init.sql adopts the plan tables that began life in recipes.
    ("mealplans", "recipes", "plans"),
    ("mealplans", "recipes", "plan_access"),
    ("mealplans", "recipes", "plan_meals"),
}

apps = sorted(
    d for d in os.listdir("apps") if os.path.isdir(f"apps/{d}/migrations")
)
schema = "(" + "|".join(apps) + ")"
opt = r"(?:IF\s+(?:NOT\s+)?EXISTS\s+)?"
ref = re.compile(
    rf"\b(FROM|JOIN|INTO|UPDATE|REFERENCES|TABLE|TRUNCATE)\s+{opt}{schema}\.(\w+)",
    re.I,
)
write = re.compile(
    r"\b(?:INSERT\s+INTO|UPDATE|DELETE\s+FROM|TRUNCATE(?:\s+TABLE)?"
    rf"|(?:ALTER|CREATE|DROP)\s+TABLE)\s+{opt}{schema}\.\w+",
    re.I,
)

failed = False
for app in apps:
    for root, _, files in os.walk(f"apps/{app}"):
        if "/mocks" in root:
            continue
        for name in sorted(files):
            if not name.endswith((".go", ".sql")) or name.endswith("_test.go"):
                continue
            path = os.path.join(root, name)
            comment = r"--[^\n]*" if name.endswith(".sql") else r"//[^\n]*"
            text = re.sub(comment, lambda m: " " * len(m.group()), open(path).read())
            writes = {m.start(1) for m in write.finditer(text)}
            for m in ref.finditer(text):
                target, table = m.group(2).lower(), m.group(3)
                if target == app:
                    continue
                line = text.count("\n", 0, m.start()) + 1
                verb = "write" if m.start(2) in writes else "read"
                if target not in ALLOWED.get(app, set()):
                    problem = f"may not {verb}"
                elif verb == "write" and (app, target, table) not in ALLOWED_WRITES:
                    problem = "may not write"
                else:
                    continue
                failed = True
                print(f"{path}:{line}: {app} {problem} {target}.{table}")

if failed:
    print()
    print("Cross-schema SQL goes downstream only (recipes <- mealplans <- shoppinglist),")
    print("reads and foreign keys only; see docs/adr-0036-downstream-cross-schema-reads.md.")
    sys.exit(1)
PY
