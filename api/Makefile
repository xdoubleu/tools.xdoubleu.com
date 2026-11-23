# Per-worktree cache: the shared default mixes worktrees' paths into output.
# ?= lets a sandbox redirect it out of a read-only checkout.
export GOLANGCI_LINT_CACHE ?= $(CURDIR)/.golangci-cache

# A job sandbox has the flake's tools at this profile but not on PATH and no C
# compiler: append it (a `nix develop` shell still wins) and build pure Go there.
export PATH := $(PATH):/nix/var/nix/profiles/devshell/bin
ifeq ($(shell command -v cc 2>/dev/null),)
export CGO_ENABLED ?= 0
endif

# Apply every migration (cmd/api's TestMain does it; idempotent). Test targets
# depend on it: other packages fail with 42P01 on an unmigrated database.
db/migrate:
	TZ=UTC go test -count=1 ./cmd/api -run TestApplyMigrations_LockTimeout

# Recreate the local Postgres volume; use when a failure looks like test-order
# pollution rather than a real regression.
db/reset:
	docker-compose down -v
	docker-compose up -d

# Tools come from the flake's dev shell (`nix develop`); these only check PATH.
tools/lint/go:
	@../scripts/require_tools.sh golangci-lint gci gosec golines

tools/lint/golines:
	@../scripts/require_tools.sh golines

tools/lint/sql:
	@../scripts/require_tools.sh sqlfluff

tools/proto:
	@../scripts/require_tools.sh buf

# For environments that can't reach buf.build; the plugins must match
# buf.gen.yaml's pins (make lint/proto-local-versions).
tools/proto/local: tools/proto
	@../scripts/require_tools.sh protoc-gen-go protoc-gen-connect-go

tools/mutation:
	@../scripts/require_tools.sh gremlins

tools/vuln:
	@../scripts/require_tools.sh govulncheck

tools/arch:
	@../scripts/require_tools.sh godepgraph

tools/lint: tools/lint/go tools/lint/sql

tools: tools/lint tools/proto

# --tab-len 1 matches lll, which counts a tab as one column.
GOLINES = golines -m 88 --tab-len 1 --ignore-generated

lint/sql: tools/lint/sql
	sqlfluff lint --dialect postgres .

lint/proto: tools/proto
	cd ../proto && buf lint

# New migration with a UTC timestamp version, so parallel branches never collide.
# app=<name> (apps/<name>) | platform/<name> (internal/platform/<name>) | global
migration/new:
	@test -n "$(app)" -a -n "$(name)" || { echo "usage: make migration/new app=<app> name=<name>"; exit 1; }
	@case "$(app)" in \
		global) dir=cmd/api/migrations ;; \
		platform/*) dir=internal/$(app)/migrations ;; \
		*) dir=apps/$(app)/migrations ;; \
	esac; \
	test -d "$$dir" || { echo "no migrations dir $$dir"; exit 1; }; \
	f="$$dir/$$(date -u +%Y%m%d%H%M%S)_$(name).sql"; \
	printf -- '-- +goose Up\n\n-- +goose Down\n' > "$$f"; echo "$$f"

lint/migrations:
	./scripts/test_check_migration_versions.sh
	./scripts/check_migration_versions.sh
	sqlfluff lint --dialect postgres apps/*/migrations internal/platform/*/migrations

lint/kamal-secrets:
	./scripts/check_kamal_secrets.sh

lint/hexagonal:
	./scripts/test_check_hexagonal.sh
	./scripts/check_hexagonal.sh

lint/cross-schema:
	./scripts/test_check_cross_schema.sh
	./scripts/check_cross_schema.sh

lint/scalar-subqueries:
	./scripts/test_check_scalar_subqueries.sh
	./scripts/check_scalar_subqueries.sh

lint/proto-local-versions:
	./scripts/check_proto_local_versions.sh

# Mermaid package-dependency diagram of this module's own packages (gen/
# excluded). Diagnostic only; output is not committed.
arch/diagram: tools/arch
	godepgraph -format mermaid -s -novendor \
		-onlyprefixes tools.xdoubleu.com \
		-ignoreprefixes tools.xdoubleu.com/gen \
		tools.xdoubleu.com/cmd/api > package-graph.mmd

lint: tools/lint
	golangci-lint run
	make lint/golines
	make lint/sql
	make lint/proto
	make lint/migrations
	make lint/kamal-secrets
	make lint/hexagonal
	make lint/cross-schema
	make lint/scalar-subqueries
	make lint/proto-local-versions

lint/pkg: tools/lint
	golangci-lint run ./$(PKG)/...

lint/fix/pkg: tools/lint
	$(GOLINES) -w ./$(PKG)
	golangci-lint run --fix ./$(PKG)/...
	gci write --skip-generated -s standard -s default -s "prefix(tools.xdoubleu.com)" ./$(PKG)

lint/fix: tools/lint
	$(GOLINES) -w .
	golangci-lint run --fix
	gci write --skip-generated -s standard -s default -s "prefix(tools.xdoubleu.com)" .
	sqlfluff fix --dialect postgres .
	make lint/proto

# Fails when lint/fix would rewrite anything: lll accepts //nolint:lll, golines doesn't.
lint/golines: tools/lint/golines
	@diff=$$($(GOLINES) --dry-run .); \
	if [ -n "$$diff" ]; then echo "$$diff"; echo "golines drift: run make lint/fix"; exit 1; fi

# Not part of lint: a new disclosure would fail unrelated work, so CI runs it on
# a schedule (api-vuln.yml).
lint/vuln: tools/vuln
	govulncheck ./...

lint/go/ci:
	golangci-lint run

lint/sql/ci:
	sqlfluff lint --dialect postgres .

proto/generate: tools/proto
	buf generate

# Same output as proto/generate, using local plugins instead of buf.build.
proto/generate/local: tools/proto/local
	buf generate --template buf.gen.local.yaml

# Mirrors CI's proto-staleness check. Compares gen/ before and after
# regenerating, so freshly generated but uncommitted stubs pass.
proto/check: tools/proto
	@before=$$(find gen -type f | sort | xargs sha256sum); \
	buf generate; \
	after=$$(find gen -type f | sort | xargs sha256sum); \
	[ "$$before" = "$$after" ] || { echo "api/gen/ is out of date — run 'make proto/generate' (api) and 'npm run generate' (web), then commit the results." >&2; exit 1; }

# proto/check with local plugins, for sandboxes that can't reach buf.build.
proto/check/local: tools/proto/local
	@before=$$(find gen -type f | sort | xargs sha256sum); \
	buf generate --template buf.gen.local.yaml; \
	after=$$(find gen -type f | sort | xargs sha256sum); \
	[ "$$before" = "$$after" ] || { echo "api/gen/ is out of date — run 'make proto/generate' (api) and 'npm run generate' (web), then commit the results." >&2; exit 1; }

build:
	go build -o=./bin/api ./cmd/api

run:
	go run ./cmd/api

debug:
	dlv debug --headless --listen=:2345 --api-version=2 --accept-multiclient ./cmd/api

# TZ=UTC: tests assert UTC timestamps (movies watch dates) while pgx scans
# TIMESTAMPTZ into time.Local; CI hosts are UTC, so pin it locally too.
test: db/migrate
	TZ=UTC go test -count=1 -p 1 ./...

test/v: db/migrate
	TZ=UTC go test -p 1 ./... -v

# Every read of an app on production-shaped data (one PR on several tasks, ...):
# fails on any error. The seed lives in internal/platform/factory/messyseed.
test/reads: db/migrate
	TZ=UTC go test -p 1 -count=1 -run TestMessyReads ./...

test/race: db/migrate
	TZ=UTC go test -p 1 ./... -race -v

# Converts PDFs in testdata/pdf.local/ (gitignored) to EPUB for manual review.
pdf/check:
	go test ./apps/books/internal/services/... -run TestPDFCheck -v -count=1

test/pprof:
	TZ=UTC go test -p 1 ./... -cpuprofile cpu.prof -memprofile mem.prof -bench ./cmd/api

# Runs gremlins on Go packages changed vs origin/main (one invocation per
# package); --diff further scopes mutants to changed lines. gremlins matches
# `git diff` paths to package-relative mutant paths, so it runs from the
# package dir with diff.relative (and diff.context=0: it undercounts
# multi-block hunks); check_mutation_report_go.py fails if a changed line's
# mutant is still SKIPPED and lists NOT COVERED ones. Serial because test packages share Postgres
# schemas. .gremlins.yaml excludes gen/ and *_mock.go.
test/mutation/diff: tools/mutation
	@pkgs=$$(python3 ../tools/diff_packages_go.py) || exit 1; \
	if [ -z "$$pkgs" ]; then \
		echo "No changed Go packages vs origin/main -- nothing to mutation-test."; \
	else \
		status=0; \
		report=$$(mktemp); \
		for pkg in $$pkgs; do \
			echo "Diff-scoped mutation testing: $$pkg"; \
			(cd "$$pkg" && \
				GOFLAGS="$$GOFLAGS -p=1 -count=1" GIT_CONFIG_COUNT=2 \
				GIT_CONFIG_KEY_0=diff.relative GIT_CONFIG_VALUE_0=true \
				GIT_CONFIG_KEY_1=diff.context GIT_CONFIG_VALUE_1=0 \
				gremlins unleash --diff origin/main --workers 1 \
					--output "$$report" "$$PWD") && \
			python3 ../tools/check_mutation_report_go.py "$$pkg" "$$report" || status=1; \
		done; \
		rm -f "$$report"; \
		exit $$status; \
	fi

# Full-repo baseline; slow, not for routine use.
test/mutation: tools/mutation
	GOFLAGS="$$GOFLAGS -p=1 -count=1" gremlins unleash --workers 1 ./...

# Overridable so a sandbox can point coverage artifacts outside the checkout.
COVERAGE ?= coverage.out

# -count=1: a cached package result would leave stale blocks in the profile.
test/cov/report: db/migrate
	TZ=UTC go test -p 1 -count=1 ./... -coverpkg=./cmd/api,./internal/...,./apps/... -covermode=set -coverprofile=$(COVERAGE).tmp
	$(MAKE) test/cov/merge

test/cov/merge:
	cat $(COVERAGE).tmp | grep -v "_mock.go" | grep -v "/gen/" > $(COVERAGE).filtered
	echo "mode: set" > $(COVERAGE)
	# -coverpkg makes other binaries emit count=0 duplicates of a block; keep the
	# highest count per block.
	tail -n +2 $(COVERAGE).filtered | sort -t' ' -k1,1 -k3,3rn | awk '!seen[$$1]++' >> $(COVERAGE)

test/cov: test/cov/report
	go tool cover -html=$(COVERAGE) -o=coverage.html
	make test/cov/open

# Test output goes to $(COVERAGE).log (only failures are echoed) so the
# changed-lines table is the last thing printed. COV_REUSE=1 re-reads the
# existing profile without re-running the suite.
test/cov/diff:
ifndef COV_REUSE
	@$(MAKE) --no-print-directory db/migrate > /dev/null
	@TZ=UTC go test -p 1 -count=1 ./... -coverpkg=./cmd/api,./internal/...,./apps/... -covermode=set -coverprofile=$(COVERAGE).tmp > $(COVERAGE).log 2>&1 \
		|| { grep -v -e '^ok ' -e 'no test files' -e '^coverage:' $(COVERAGE).log; echo "tests failed (full log: $(COVERAGE).log)"; exit 1; }
	@$(MAKE) --no-print-directory test/cov/merge > /dev/null
endif
	@../scripts/diff_coverage.sh go $(COVERAGE)

test/cov/open:
	open ./coverage.html

test/cov/per-pkg:
	go test ./apps/todos \
		-coverpkg=./apps/todos,./apps/todos/internal/... \
		-covermode=set -count=1 -coverprofile=cov_todos_pkg.out
	go test ./apps/todos/internal/... \
		-coverpkg=./apps/todos,./apps/todos/internal/... \
		-covermode=set -count=1 -coverprofile=cov_todos_internal.out
	go test ./apps/games \
		-coverpkg=./apps/games,./apps/games/internal/... \
		-covermode=set -count=1 -coverprofile=cov_games_pkg.out
	go test ./apps/games/internal/... \
		-coverpkg=./apps/games,./apps/games/internal/... \
		-covermode=set -count=1 -coverprofile=cov_games_internal.out
	go test ./apps/books \
		-coverpkg=./apps/books,./apps/books/internal/... \
		-covermode=set -count=1 -coverprofile=cov_books_pkg.out
	go test ./apps/books/internal/... \
		-coverpkg=./apps/books,./apps/books/internal/... \
		-covermode=set -count=1 -coverprofile=cov_books_internal.out
	go test ./apps/recipes \
		-coverpkg=./apps/recipes,./apps/recipes/internal/... \
		-covermode=set -count=1 -coverprofile=cov_recipes_pkg.out
	go test ./apps/mealplans \
		-coverpkg=./apps/mealplans,./apps/mealplans/internal/... \
		-covermode=set -count=1 -coverprofile=cov_mealplans_pkg.out
	go test ./apps/shoppinglist \
		-coverpkg=./apps/shoppinglist,./apps/shoppinglist/internal/... \
		-covermode=set -count=1 -coverprofile=cov_shoppinglist_pkg.out
	go test ./apps/shoppinglist/internal/services \
		-coverpkg=./apps/shoppinglist/internal/services \
		-covermode=set -count=1 -coverprofile=cov_shoppinglist_svc.out
	go test ./cmd/api \
		-coverpkg=./cmd/api,./internal/... \
		-covermode=set -count=1 -coverprofile=cov_api_pkg.out
	go test ./internal/app \
		-coverpkg=./internal/app \
		-covermode=set -count=1 -coverprofile=cov_internal_app.out
	go test ./internal/contacts \
		-coverpkg=./internal/contacts \
		-covermode=set -count=1 -coverprofile=cov_internal_contacts.out
	../scripts/diff_coverage.sh merge \
		cov_todos_pkg.out cov_todos_internal.out \
		cov_games_pkg.out cov_games_internal.out \
		cov_books_pkg.out cov_books_internal.out \
		cov_recipes_pkg.out \
		cov_mealplans_pkg.out \
		cov_shoppinglist_pkg.out cov_shoppinglist_svc.out \
		cov_api_pkg.out \
		cov_internal_app.out cov_internal_contacts.out
