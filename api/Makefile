UNAME_S := $(shell uname -s)

tools: tools/lint tools/proto

tools/lint: tools/lint/go tools/lint/sql

tools/lint/go:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
	go install github.com/segmentio/golines@v0.13.0
	go install github.com/daixiang0/gci@v0.14.0
	go install github.com/securego/gosec/v2/cmd/gosec@v2.28.0

tools/lint/sql:
ifeq ($(UNAME_S),Darwin)
	brew install sqlfluff
endif
ifeq ($(UNAME_S),Linux)
	pip install sqlfluff==3.3.0
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

lint/pkg: tools/lint
	golangci-lint run ./$(PKG)/...

lint/fix: tools/lint
	golines . -m 88 -w
	golangci-lint run --fix
	gci write --skip-generated -s standard -s default -s "prefix(tools.xdoubleu.com)" .
	sqlfluff fix --dialect postgres .

lint/go/ci:
	golangci-lint run

lint/sql/ci:
	sqlfluff lint --dialect postgres .

proto/generate:
	buf generate

RELEASE ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

build:
	go build -ldflags="-X main.Release=$(RELEASE)" -o=./bin/api ./cmd/api

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

test/pprof:
	go test -p 1 ./... -cpuprofile cpu.prof -memprofile mem.prof -bench ./cmd/api

test/cov/report:
	go test -p 1 ./... -coverpkg=./cmd/api,./internal/...,./apps/... -covermode=set -coverprofile=coverage.out.tmp
	cat coverage.out.tmp | grep -v "_mock.go" | grep -v "/gen/" > coverage.out

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
	go test ./apps/icsproxy \
		-coverpkg=./apps/icsproxy,./apps/icsproxy/internal/... \
		-covermode=set -count=1 -coverprofile=cov_icsproxy_pkg.out
	go test ./apps/icsproxy/internal/services \
		-coverpkg=./apps/icsproxy/internal/services \
		-covermode=set -count=1 -coverprofile=cov_icsproxy_svc.out
	go test ./apps/icsproxy/internal/dtos \
		-coverpkg=./apps/icsproxy/internal/dtos \
		-covermode=set -count=1 -coverprofile=cov_icsproxy_dtos.out
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
		cov_icsproxy_pkg.out cov_icsproxy_svc.out cov_icsproxy_dtos.out \
		cov_recipes_pkg.out \
		cov_mealplans_pkg.out \
		cov_shoppinglist_pkg.out cov_shoppinglist_svc.out \
		cov_watchparty_pkg.out cov_watchparty_internal.out \
		cov_api_pkg.out \
		cov_internal_app.out cov_internal_contacts.out \
		cov_internal_format.out
