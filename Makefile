tools: tools/lint

tools/lint: tools/lint/go tools/lint/sql

tools/lint/go:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.0.2
	go install github.com/segmentio/golines@v0.12.2
	go install github.com/daixiang0/gci@v0.13.5
	go install github.com/securego/gosec/v2/cmd/gosec@v2.21.4

tools/lint/sql:
ifeq ($(UNAME_S),Darwin)
	brew install sqlfluff
endif
ifeq ($(UNAME_S),Linux)
	pip install sqlfluff
endif

lint/sql: tools/lint/sql
	sqlfluff lint --dialect postgres .

lint: tools/lint
	golangci-lint run
	make lint/sql

lint/fix:
	golines . -m 88 -w
	golangci-lint run --fix
	gci write --skip-generated -s standard -s default .
	sqlfluff fix --dialect postgres .

build: 
	go build -o=./bin/publish ./cmd/publish

run:
	go run ./...

test:
	go test ./...

test/v:
	go test ./... -v
	
test/race:
	go test ./... -race -v

test/pprof:
	go test ./... -cpuprofile cpu.prof -memprofile mem.prof -bench ./cmd/publish

test/cov/report:
	go test ./... -coverpkg=./cmd/publish,./internal/...,./apps/... -covermode=set -coverprofile=coverage.out.tmp
	cat coverage.out.tmp | grep -v "_mock.go" > coverage.out

test/cov: test/cov/report
	go tool cover -html=coverage.out -o=coverage.html
	make test/cov/open

test/cov/open:
	open ./coverage.html
