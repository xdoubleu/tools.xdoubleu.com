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

# New ADR with the next free number (vs origin/main) and its AGENTS.md index line.
adr/new:
	./scripts/adr_new.sh "$(name)" "$(label)"

# Fail on invalid SKILL.md frontmatter (harnesses silently drop the skill).
lint/skills:
	./scripts/lint_skills.sh

# Line budgets for agent instruction files, skills and ADRs, and backticked
# repo paths that must exist: the xdoubleu/skills doc-budgets plugin at a
# pinned commit, configured by .claude/doc-budgets.json. Set DOC_BUDGETS_DIR
# to a plugin checkout to run offline; without it, a cached or Nix-store
# checkout is used before cloning.
DOC_BUDGETS_VERSION ?= 537d2caae13e00fced04cc290b5bf4b2f19264b7
DOC_BUDGETS_DIR ?= $(or $(XDG_CACHE_HOME),$(HOME)/.cache)/doc-budgets/$(DOC_BUDGETS_VERSION)
DOC_BUDGETS_STORE := $(patsubst %/scripts/lint_docs.sh,%,$(firstword $(wildcard /nix/store/*/plugins/doc-budgets/scripts/lint_docs.sh)))
lint/docs:
	@dir="$(DOC_BUDGETS_DIR)"; \
	if [ ! -x "$$dir/scripts/lint_docs.sh" ] && [ "$(origin DOC_BUDGETS_DIR)" = file ] && [ -n "$(DOC_BUDGETS_STORE)" ]; then \
		dir="$(DOC_BUDGETS_STORE)"; \
	fi; \
	test -x "$$dir/scripts/lint_docs.sh" || { \
		rm -rf "$$dir.tmp" && \
		git clone -q https://github.com/xdoubleu/skills "$$dir.tmp" && \
		git -C "$$dir.tmp" -c advice.detachedHead=false checkout -q $(DOC_BUDGETS_VERSION) && \
		mkdir -p "$$(dirname "$$dir")" && \
		mv "$$dir.tmp/plugins/doc-budgets" "$$dir" && \
		rm -rf "$$dir.tmp"; } && \
	"$$dir/scripts/lint_docs.sh" && \
	"$$dir/scripts/lint_doc_paths.sh"

# Exercise scripts/check_feature_gate.sh against a synthetic repo.
lint/docs/test:
	./scripts/test_check_feature_gate.sh
	./scripts/test_mutation_diff.sh
