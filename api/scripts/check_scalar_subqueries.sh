#!/usr/bin/env bash
# A scalar subquery `(SELECT col FROM schema.table WHERE ...)` used as a value
# fails with SQLSTATE 21000 once two rows match. It must aggregate, end in
# LIMIT 1, or filter on a unique key of its table (primary key or unique index
# from the migrations' Up sections). A table holding one row by design carries
# `-- single-row` in the subquery. See docs/convention-database-queries.md.
# Test files are exempt.
set -euo pipefail

# Paths below are relative to api/.
cd "$(dirname "$0")/.."

python3 - <<'PY'
import glob
import os
import re
import sys

AGG = re.compile(
    r"\b(count|sum|max|min|avg|array_agg|string_agg|bool_or|bool_and|json_agg"
    r"|jsonb_agg|jsonb_object_agg)\s*\(",
    re.I,
)
# What may precede the subquery's "(" when it is not a scalar value.
NOT_SCALAR = re.compile(
    r"(\bEXISTS|\bIN|\bNOT\s+IN|\bANY|\bALL|\bFROM|\bJOIN|\bUNION(?:\s+ALL)?"
    r"|\bARRAY|\bAS|\bLATERAL|\bUSING)\s*$",
    re.I,
)
IDENT = r'"?\w+"?'


def cols(text):
    return frozenset(c.strip().strip('"').lower() for c in text.split(","))


def up_section(sql):
    return re.split(r"--\s*\+goose\s+Down", sql, flags=re.I)[0]


def strip_comments(sql):
    return re.sub(r"--[^\n]*", lambda m: " " * len(m.group()), sql)


def unique_keys():
    """schema.table -> {constraint name: column set} after every Up section."""
    keys = {}
    files = sorted(
        glob.glob("apps/*/migrations/*.sql")
        + glob.glob("internal/platform/*/migrations/*.sql"),
        key=os.path.basename,
    )
    for path in files:
        sql = strip_comments(up_section(open(path).read()))
        for m in re.finditer(r"DROP\s+SCHEMA\s+(?:IF\s+EXISTS\s+)?(\w+)", sql, re.I):
            for table in [t for t in keys if t.startswith(m.group(1).lower() + ".")]:
                del keys[table]
        for m in re.finditer(r"DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?([\w.]+)", sql, re.I):
            keys.pop(m.group(1).lower(), None)
        for m in re.finditer(
            r"CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([\w.]+)\s*\((.*?)\n\);",
            sql, re.I | re.S,
        ):
            table, body = m.group(1).lower(), m.group(2)
            found = keys.setdefault(table, {})
            for line in re.split(r",\s*\n", body):
                line = line.strip()
                tl = re.match(r"(?:CONSTRAINT\s+\w+\s+)?(PRIMARY\s+KEY|UNIQUE)\s*\(([^)]*)\)", line, re.I)
                inline = re.match(rf"({IDENT})\s+.*\b(PRIMARY\s+KEY|UNIQUE)\b", line, re.I)
                if tl:
                    kind, key = tl.group(1).upper(), cols(tl.group(2))
                elif inline and not re.match(r"(CONSTRAINT|CHECK|FOREIGN)\b", line, re.I):
                    kind, key = inline.group(2).upper(), cols(inline.group(1))
                else:
                    continue
                found["pk" if kind.startswith("PRIMARY") else f"c{len(found)}"] = key
        for m in re.finditer(
            r"CREATE\s+UNIQUE\s+INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?([\w.]+)\s+ON\s+"
            r"([\w.]+)\s*\(([^)]*)\)(\s*WHERE)?",
            sql, re.I,
        ):
            # A partial index only constrains some rows.
            if not m.group(4):
                keys.setdefault(m.group(2).lower(), {})[m.group(1).lower()] = cols(m.group(3))
        for m in re.finditer(
            r"ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?([\w.]+)\s+(.*?);",
            sql, re.I | re.S,
        ):
            table, ops = m.group(1).lower(), m.group(2)
            found = keys.setdefault(table, {})
            if re.search(r"DROP\s+CONSTRAINT\s+(?:IF\s+EXISTS\s+)?\w*_pkey", ops, re.I):
                found.pop("pk", None)
            pk = re.search(r"ADD\s+(?:CONSTRAINT\s+\w+\s+)?PRIMARY\s+KEY\s*\(([^)]*)\)", ops, re.I)
            if pk:
                found["pk"] = cols(pk.group(1))
        for m in re.finditer(r"DROP\s+INDEX\s+(?:IF\s+EXISTS\s+)?([\w.]+)", sql, re.I):
            name = m.group(1).lower()
            for found in keys.values():
                found.pop(name, None)
    return keys


def close_paren(text, start):
    depth = 0
    for i in range(start, len(text)):
        depth += {"(": 1, ")": -1}.get(text[i], 0)
        if depth == 0:
            return i
    return len(text)


def check(path, keys):
    raw = open(path).read()
    comment = r"//[^\n]*"
    text = re.sub(comment, lambda m: " " * len(m.group()), raw)
    problems = []
    for m in re.finditer(r"\(\s*SELECT\b", text, re.I):
        if NOT_SCALAR.search(text[: m.start()]):
            continue
        sub = text[m.start() : close_paren(text, m.start()) + 1]
        select_list = re.split(r"\bFROM\b", sub, maxsplit=1, flags=re.I)[0]
        if AGG.search(select_list) or re.search(r"\bLIMIT\s+1\b", sub, re.I):
            continue
        if "-- single-row" in sub:
            continue
        table = re.search(rf"\bFROM\s+(\w+\.\w+)(?:\s+(?:AS\s+)?(?!WHERE\b|JOIN\b|ORDER\b|GROUP\b)(\w+))?", sub, re.I)
        if not table:
            continue  # selects from a CTE or a derived table
        name = table.group(1).lower()
        where = re.search(r"\bWHERE\b(.*)", sub, re.I | re.S)
        filtered = set()
        if where:
            for c in re.finditer(r"(?:\b\w+\.)?(\w+)\s*=\s*(?!=)", where.group(1)):
                filtered.add(c.group(1).lower())
        if any(k <= filtered for k in keys.get(name, {}).values()):
            continue
        line = text.count("\n", 0, m.start()) + 1
        problems.append(f"{path}:{line}: scalar subquery on {name} matches several rows")
    return problems


keys = unique_keys()
failed = False
for root, _, files in os.walk("."):
    if "/gen" in root or "/mocks" in root or "/node_modules" in root:
        continue
    for name in sorted(files):
        if name.endswith(".go") and not name.endswith("_test.go"):
            for problem in check(os.path.join(root, name)[2:], keys):
                failed = True
                print(problem)

if failed:
    print()
    print("Make the subquery unique by constraint, aggregate it, or end it in LIMIT 1;")
    print("see docs/convention-database-queries.md.")
    sys.exit(1)
PY
