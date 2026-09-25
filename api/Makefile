UNAME_S := $(shell uname -s)

# Per-worktree cache: the shared default mixes worktrees' paths into output.
export GOLANGCI_LINT_CACHE := $(CURDIR)/.golangci-cache

# Recreate the local Postgres volume; use when a failure looks like test-order
# pollution rather than a real regression.
db/reset:
	docker-compose down -v
	docker-compose up -d

tools: tools/lint tools/proto

tools/lint: tools/lint/go tools/lint/sql

tools/lint/go:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2
	go install github.com/segmentio/golines@v0.13.0
	go install github.com/daixiang0/gci@v0.14.0
	go install github.com/securego/gosec/v2/cmd/gosec@v2.29.0

# Keep in step with Homebrew's sqlfluff (unpinnable on macOS): LT01's
# `TEXT[]` rule differs between 3.x and 4.x.
tools/lint/sql:
ifeq ($(UNAME_S),Darwin)
	brew install sqlfluff
endif
ifeq ($(UNAME_S),Linux)
	pip install sqlfluff==4.3.0
endif

lint/sql: tools/lint/sql
	sqlfluff lint --dialect postgres .

tools/proto:
	go install github.com/bufbuild/buf/cmd/buf@v1.73.0

# For environments that can't reach buf.build. Versions must match
# buf.gen.yaml's pins (checked by make lint/proto-local-versions).
tools/proto/local: tools/proto
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
	go install connectrpc.com/connect/cmd/protoc-gen-connect-go@v1.21.0

lint/proto: tools/proto
	cd ../proto && buf lint

lint/migrations:
	./scripts/check_migration_versions.sh

lint/kamal-secrets:
	./scripts/check_kamal_secrets.sh

lint/proto-local-versions:
	./scripts/check_proto_local_versions.sh

tools/arch:
	go install github.com/kisielk/godepgraph@v1.0.0

# Mermaid package-dependency diagram of this module's own packages (gen/
# excluded). Diagnostic only; output is not committed.
arch/diagram: tools/arch
	godepgraph -format mermaid -s -novendor \
		-onlyprefixes tools.xdoubleu.com \
		-ignoreprefixes tools.xdoubleu.com/gen \
		tools.xdoubleu.com/cmd/api > package-graph.mmd

lint: tools/lint
	golangci-lint run
	make lint/sql
	make lint/proto
	make lint/migrations
	make lint/kamal-secrets
	make lint/proto-local-versions

lint/pkg: tools/lint
	golangci-lint run ./$(PKG)/...

lint/fix/pkg: tools/lint
	golines ./$(PKG) -m 88 -w --ignore-generated
	golangci-lint run --fix ./$(PKG)/...
	gci write --skip-generated -s standard -s default -s "prefix(tools.xdoubleu.com)" ./$(PKG)

lint/fix: tools/lint
	golines . -m 88 -w --ignore-generated
	golangci-lint run --fix
	gci write --skip-generated -s standard -s default -s "prefix(tools.xdoubleu.com)" .
	sqlfluff fix --dialect postgres .
	make lint/proto

lint/go/ci:
	golangci-lint run

lint/sql/ci:
	sqlfluff lint --dialect postgres .

proto/generate: tools/proto
	buf generate

# Same output as proto/generate, using local plugins instead of buf.build.
proto/generate/local: tools/proto/local
	buf generate --template buf.gen.local.yaml

# Mirrors CI's proto-staleness check.
proto/check: proto/generate
	git diff --exit-code gen/ || (echo "api/gen/ is out of date — run 'make proto/generate' (api) and 'npm run generate' (web), then commit the results." && exit 1)

build:
	go build -o=./bin/api ./cmd/api

run:
	go run ./cmd/api

debug:
	dlv debug --headless --listen=:2345 --api-version=2 --accept-multiclient ./cmd/api

test:
	go test -p 1 ./...

test/v:
	go test -p 1 ./... -v

test/race:
	go test -p 1 ./... -race -v

# Converts PDFs in testdata/pdf.local/ (gitignored) to EPUB for manual review.
pdf/check:
	go test ./apps/books/internal/services/... -run TestPDFCheck -v -count=1

test/pprof:
	go test -p 1 ./... -cpuprofile cpu.prof -memprofile mem.prof -bench ./cmd/api

tools/mutation:
	go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0

# Runs gremlins on Go packages changed vs origin/main (one invocation per
# package); --diff further scopes mutants to changed lines. .gremlins.yaml
# excludes gen/ and *_mock.go.
test/mutation/diff: tools/mutation
	@pkgs=$$(python3 ../tools/diff_packages_go.py); \
	if [ -z "$$pkgs" ]; then \
		echo "No changed Go packages vs origin/main -- nothing to mutation-test."; \
	else \
		status=0; \
		for pkg in $$pkgs; do \
			echo "Diff-scoped mutation testing: $$pkg"; \
			gremlins unleash --diff origin/main "$$pkg" || status=1; \
		done; \
		exit $$status; \
	fi

# Full-repo baseline; slow, not for routine use.
test/mutation: tools/mutation
	gremlins unleash ./...

test/cov/report:
	go test -p 1 ./... -coverpkg=./cmd/api,./internal/...,./apps/... -covermode=set -coverprofile=coverage.out.tmp
	cat coverage.out.tmp | grep -v "_mock.go" | grep -v "/gen/" > coverage.out.filtered
	echo "mode: set" > coverage.out
	# -coverpkg makes other binaries emit count=0 duplicates that Codecov doesn't
	# OR-merge; keep the highest count per line.
	tail -n +2 coverage.out.filtered | sort -t' ' -k1,1 -k3,3rn | awk '!seen[$$1]++' >> coverage.out
	# Wrapped signature lines belong to no coverage block; extend blocks back to
	# `func` so Codecov doesn't count them as missed.
	python3 ../tools/extend_signature_coverage.py coverage.out

test/cov: test/cov/report
	go tool cover -html=coverage.out -o=coverage.html
	make test/cov/open

test/cov/diff: test/cov/report
	python3 ../tools/diff_coverage_go.py coverage.out

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
	go test ./apps/watchparty \
		-coverpkg=./apps/watchparty,./apps/watchparty/internal/... \
		-covermode=set -count=1 -coverprofile=cov_watchparty_pkg.out
	go test ./apps/watchparty/internal/... \
		-coverpkg=./apps/watchparty,./apps/watchparty/internal/... \
		-covermode=set -count=1 -coverprofile=cov_watchparty_internal.out
	go test ./cmd/api \
		-coverpkg=./cmd/api,./internal/... \
		-covermode=set -count=1 -coverprofile=cov_api_pkg.out
	go test ./internal/app \
		-coverpkg=./internal/app \
		-covermode=set -count=1 -coverprofile=cov_internal_app.out
	go test ./internal/contacts \
		-coverpkg=./internal/contacts \
		-covermode=set -count=1 -coverprofile=cov_internal_contacts.out
	go test ./internal/format \
		-coverpkg=./internal/format \
		-covermode=set -count=1 -coverprofile=cov_internal_format.out
	python3 ../tools/merge_coverage.py \
		cov_todos_pkg.out cov_todos_internal.out \
		cov_games_pkg.out cov_games_internal.out \
		cov_books_pkg.out cov_books_internal.out \
		cov_recipes_pkg.out \
		cov_mealplans_pkg.out \
		cov_shoppinglist_pkg.out cov_shoppinglist_svc.out \
		cov_watchparty_pkg.out cov_watchparty_internal.out \
		cov_api_pkg.out \
		cov_internal_app.out cov_internal_contacts.out \
		cov_internal_format.out
