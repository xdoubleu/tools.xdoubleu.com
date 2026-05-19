UNAME_S := $(shell uname -s)

tools: tools/lint tools/proto

tools/lint: tools/lint/go tools/lint/sql

tools/lint/go:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.3
	go install github.com/segmentio/golines@v0.13.0
	go install github.com/daixiang0/gci@v0.13.7
	go install github.com/securego/gosec/v2/cmd/gosec@v2.22.11

tools/lint/sql:
ifeq ($(UNAME_S),Darwin)
	brew install sqlfluff
endif
ifeq ($(UNAME_S),Linux)
	pip install sqlfluff
endif

lint/sql: tools/lint/sql
	sqlfluff lint --dialect postgres .

tools/proto:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.5
	go install connectrpc.com/connect/cmd/protoc-gen-connect-go@v1.19.2
	go install github.com/bufbuild/buf/cmd/buf@v1.69.0

lint/proto: tools/proto
	cd ../proto && buf lint

lint: tools/lint
	golangci-lint run
	make lint/sql
	make lint/proto

lint/fix: tools/lint
	golines . -m 88 -w
	golangci-lint run --fix
	gci write --skip-generated -s standard -s default -s "prefix(tools.xdoubleu.com)" .
	sqlfluff fix --dialect postgres .

proto/generate: tools/proto
	buf generate

build:
	go build -o=./bin/api ./cmd/api

run:
	go run ./cmd/api

debug:
	dlv debug --headless --listen=:2345 --api-version=2 --accept-multiclient ./cmd/api

test:
	go test ./...

test/v:
	go test ./... -v

test/race:
	go test ./... -race -v

test/pprof:
	go test ./... -cpuprofile cpu.prof -memprofile mem.prof -bench ./cmd/api

test/cov/report:
	go test ./... -coverpkg=./cmd/api,./internal/...,./apps/... -covermode=set -coverprofile=coverage.out.tmp
	cat coverage.out.tmp | grep -v "_mock.go" | grep -v "/gen/" > coverage.out

test/cov: test/cov/report
	go tool cover -html=coverage.out -o=coverage.html
	make test/cov/open

test/cov/open:
	open ./coverage.html

test/cov/per-pkg:
	go test ./apps/todos \
		-coverpkg=./apps/todos,./apps/todos/internal/... \
		-covermode=set -coverprofile=cov_todos_pkg.out
	go test ./apps/backlog \
		-coverpkg=./apps/backlog,./apps/backlog/internal/... \
		-covermode=set -coverprofile=cov_backlog_pkg.out
	go test ./cmd/api \
		-coverpkg=./cmd/api,./internal/... \
		-covermode=set -coverprofile=cov_api_pkg.out
	python3 tools/merge_coverage.py \
		cov_todos_pkg.out cov_backlog_pkg.out cov_api_pkg.out
