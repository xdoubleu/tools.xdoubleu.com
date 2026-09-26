hooks/test:
	./scripts/test_hooks.sh

# Validate the provisioned Grafana dashboards.
lint/grafana:
	python3 scripts/validate_grafana_dashboards.py

# Boot the wrapper image and assert provisioning loads cleanly. Needs Docker.
grafana/verify:
	./scripts/verify_grafana_image.sh

# Pre-merge gate: infra-apply wouldn't fail on a malformed prometheus.yml.
lint/infra:
	./scripts/lint_infra.sh

# Fail on leftover merge-conflict markers in any tracked file.
lint/conflict-markers:
	./scripts/lint_conflict_markers.sh

# Fail if any workflow YAML doesn't parse.
lint/workflows:
	./scripts/lint_workflows.sh

# Exercise scripts/routine_watchdog.sh against synthetic transcripts.
lint/watchdog:
	./scripts/test_routine_watchdog.sh

# Exercise scripts/routine_preflight.sh against a mock MCP server.
lint/preflight:
	./scripts/test_routine_preflight.sh

# The agent-routine workflow's transcript metrics script.
routines/test:
	./scripts/test_routine_metrics.sh

# Fail on invalid SKILL.md frontmatter (harnesses silently drop the skill).
lint/skills:
	./scripts/lint_skills.sh

# Word budgets for agent instruction files; no long added comment blocks.
lint/docs:
	./scripts/lint_docs.sh
