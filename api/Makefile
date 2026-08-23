UNAME_S := $(shell uname -s)

# Worktree-local cache: golangci-lint's default cache dir is a single shared
# location, so concurrent worktrees (same Go module path) bleed each other's
# file paths into lint output. Scoping it under CURDIR isolates each worktree.
export GOLANGCI_LINT_CACHE := $(CURDIR)/.golangci-cache

# Recreates the local Postgres volume from scratch. Repeated local test runs
# against the same volume can leave state (rows, schema drift) that breaks a
# later run in ways unrelated to whatever's actually being changed — CI never
# sees this since every job gets a fresh container. Run this before a rerun if
# a failure looks like test-order pollution rather than a real regression.
db/reset:
	docker-compose down -v
	docker-compose up -d

tools: tools/lint tools/proto

tools/lint: tools/lint/go tools/lint/sql

tools/lint/go:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
	go install github.com/segmentio/golines@v0.13.0
	go install github.com/daixiang0/gci@v0.14.0
	go install github.com/securego/gosec/v2/cmd/gosec@v2.28.0

# Keep this pin in step with Homebrew's sqlfluff, which macOS gets below and
# cannot pin (PEP 668 rules out a bare `pip install` there). LT01's opinion on
# `TEXT[]` reversed between 3.x and 4.x, so a mismatch means `make lint/fix`
# on one platform produces SQL the other platform's lint rejects.
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
	go install github.com/bufbuild/buf/cmd/buf@v1.72.0

lint/proto: tools/proto
	cd ../proto && buf lint

lint: tools/lint
	golangci-lint run
	make lint/sql
	make lint/proto

lint/pkg: tools/lint
	golangci-lint run ./$(PKG)/...

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

# Mirrors what CI's proto-staleness check does: regenerate, then fail loudly
# if that changed anything under gen/ that wasn't already committed. Also
# doubles as local proof that lint/fix never touches gen/ — see gci's
# --skip-generated flag and .golangci.yml's formatters.exclusions.paths
# above; there is no ordering dependency between this and make lint/fix.
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

# Converts every PDF in apps/reading/internal/services/testdata/pdf.local/
# (gitignored, populated locally) to EPUB in a scratch dir alongside it, for
# manual inspection of real-world PDF→EPUB conversion quality. No-op if that
# directory doesn't exist or is empty.
pdf/check:
	go test ./apps/reading/internal/services/... -run TestPDFCheck -v -count=1

test/pprof:
	go test -p 1 ./... -cpuprofile cpu.prof -memprofile mem.prof -bench ./cmd/api

test/cov/report:
	go test -p 1 ./... -coverpkg=./cmd/api,./internal/...,./apps/... -covermode=set -coverprofile=coverage.out.tmp
	cat coverage.out.tmp | grep -v "_mock.go" | grep -v "/gen/" > coverage.out.filtered
	echo "mode: set" > coverage.out
	# -coverpkg spans every package, so most test binaries never execute
	# cmd/api/internal code and each writes a duplicate count=0 block for the
	# same lines the owning package's binary reports count=1 for. go tool cover
	# OR-merges duplicates locally, but Codecov's parser doesn't, so it
	# misreports genuinely covered lines as partial/missed. Keep the highest
	# count per line before uploading.
	tail -n +2 coverage.out.filtered | sort -t' ' -k1,1 -k3,3rn | awk '!seen[$$1]++' >> coverage.out

test/cov: test/cov/report
	go tool cover -html=coverage.out -o=coverage.html
	make test/cov/open

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
	python3 tools/merge_coverage.py \
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
