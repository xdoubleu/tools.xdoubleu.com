run:
	go run ./cmd/gateway

build:
	go build -ldflags="-w -s" -o=./bin/gateway ./cmd/gateway

test:
	go test ./...

test/cov/report:
	go test ./... -covermode=set -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

test/cov: test/cov/report
	go tool cover -html=coverage.out -o=coverage.html
	open ./coverage.html

tools/lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2

lint: tools/lint
	golangci-lint run

lint/fix: tools/lint
	golangci-lint run --fix
