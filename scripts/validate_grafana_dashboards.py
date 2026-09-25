#!/usr/bin/env python3
"""Validate the provisioned Grafana dashboards (nothing else checks them).

Each file must be valid JSON with a non-empty, unique `uid` and a `title`;
every panel must use a provisioned datasource uid; every Prometheus target
needs a non-empty `expr`. Run via `make lint/grafana`.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

DASHBOARD_DIR = Path(__file__).resolve().parent.parent / "infra" / "grafana" / "dashboards"
# Datasource uids provisioned in infra/grafana/provisioning/datasources/.
DATASOURCE_UIDS = frozenset({"prometheus", "github", "sentry"})


def _iter_panels(panels: list[dict]):
    for panel in panels:
        yield panel
        # Row panels nest their children under `panels`.
        yield from _iter_panels(panel.get("panels", []))


def validate_file(path: Path, seen_uids: dict[str, str]) -> list[str]:
    errors: list[str] = []
    try:
        dashboard = json.loads(path.read_text())
    except json.JSONDecodeError as exc:
        return [f"{path.name}: invalid JSON: {exc}"]

    uid = dashboard.get("uid", "")
    title = dashboard.get("title", "")
    if not uid:
        errors.append(f"{path.name}: missing `uid`")
    elif uid in seen_uids:
        errors.append(f"{path.name}: `uid` {uid!r} already used by {seen_uids[uid]}")
    else:
        seen_uids[uid] = path.name
    if not title:
        errors.append(f"{path.name}: missing `title`")

    for panel in _iter_panels(dashboard.get("panels", [])):
        if panel.get("type") == "row":
            continue
        label = f"{path.name}: panel {panel.get('id', '?')} ({panel.get('title', '')!r})"
        ds = panel.get("datasource")
        if isinstance(ds, dict) and ds.get("uid") not in (*DATASOURCE_UIDS, None):
            errors.append(
                f"{label}: datasource uid {ds.get('uid')!r} not in {sorted(DATASOURCE_UIDS)}"
            )
        for target in panel.get("targets", []):
            tds = target.get("datasource")
            tds_uid = tds.get("uid") if isinstance(tds, dict) else None
            if tds_uid is not None and tds_uid not in DATASOURCE_UIDS:
                errors.append(
                    f"{label}: target {target.get('refId', '?')} datasource uid "
                    f"{tds_uid!r} not in {sorted(DATASOURCE_UIDS)}"
                )
            # Only Prometheus targets use `expr`.
            panel_uid = ds.get("uid") if isinstance(ds, dict) else None
            effective_uid = tds_uid or panel_uid or "prometheus"
            if effective_uid == "prometheus" and not str(target.get("expr", "")).strip():
                errors.append(f"{label}: target {target.get('refId', '?')} has empty `expr`")
    return errors


def main() -> int:
    files = sorted(DASHBOARD_DIR.glob("*.json"))
    if not files:
        print(f"no dashboards found in {DASHBOARD_DIR}", file=sys.stderr)
        return 1

    seen_uids: dict[str, str] = {}
    errors: list[str] = []
    for path in files:
        errors.extend(validate_file(path, seen_uids))

    if errors:
        print("Grafana dashboard validation failed:", file=sys.stderr)
        for err in errors:
            print(f"  - {err}", file=sys.stderr)
        return 1

    print(f"ok: {len(files)} Grafana dashboard(s) valid")
    return 0


if __name__ == "__main__":
    sys.exit(main())
