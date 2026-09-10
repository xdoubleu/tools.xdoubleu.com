#!/usr/bin/env python3
"""Validate the provisioned Grafana dashboards (issue #1527).

Nothing else in the pipeline looks at infra/grafana/dashboards/*.json, and a
malformed dashboard only surfaces as a silent "Dashboard provisioning" error
in Grafana's logs after deploy. This checks the invariants that matter:

  * the file is valid JSON
  * it has a non-empty `uid` and `title`, and no two dashboards share a `uid`
  * every panel references the provisioned datasource by uid (`prometheus`)
  * every panel target carries a non-empty `expr`

Run via `make lint/grafana` (wired into `make lint`).
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

DASHBOARD_DIR = Path(__file__).resolve().parent.parent / "infra" / "grafana" / "dashboards"
DATASOURCE_UID = "prometheus"


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
        if isinstance(ds, dict) and ds.get("uid") not in (DATASOURCE_UID, None):
            errors.append(f"{label}: datasource uid {ds.get('uid')!r} != {DATASOURCE_UID!r}")
        for target in panel.get("targets", []):
            ds = target.get("datasource")
            if isinstance(ds, dict) and ds.get("uid") not in (DATASOURCE_UID, None):
                errors.append(
                    f"{label}: target {target.get('refId', '?')} datasource uid "
                    f"{ds.get('uid')!r} != {DATASOURCE_UID!r}"
                )
            if not str(target.get("expr", "")).strip():
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
