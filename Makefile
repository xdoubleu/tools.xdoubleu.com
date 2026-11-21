hooks/test:
	./scripts/test_hooks.sh
	./scripts/test_lint_skills.sh

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

# Fail when CI installs a tool the job image's manifest lacks or pins differently.
lint/ci-tools:
	./scripts/lint_ci_tools.sh

# Build the flake's dev shell: the toolchain CI and the job image share.
flake/build:
	nix develop .# --command true

# Fail when a script lost its executable bit.
lint/script-modes:
	./scripts/lint_script_modes.sh

# Exercise scripts/resolve_deploy_version.sh against a stubbed GitHub and Kamal.
lint/deploy-version:
	./scripts/test_resolve_deploy_version.sh

# The Mac sandbox probe needs macOS 26 and `container`; CI only parses it.
lint/sandbox-probe:
	bash -n scripts/factory_sandbox_probe.sh
	bash -n scripts/factory_finish.sh

# Factory-job finish step: gates for the changed modules, prints the Checks: line.
factory/finish:
	./scripts/factory_finish.sh $(BASE)

# Wait for a PR's CI, then its merge: make pr/wait PR=<n> [MODE=mergeable].
pr/wait:
	./scripts/pr_wait.sh $(PR) $(MODE)

# Fail on invalid SKILL.md frontmatter (harnesses silently drop the skill).
lint/skills:
	./scripts/lint_skills.sh

# Word budgets for agent instruction files; no long added comment blocks;
# backticked repo paths in the docs must exist.
lint/docs:
	./scripts/lint_docs.sh
	./scripts/lint_doc_paths.sh

# Exercise scripts/lint_doc_paths.sh against a synthetic repo.
lint/docs/test:
	./scripts/test_lint_doc_paths.sh
	./scripts/test_check_feature_gate.sh
