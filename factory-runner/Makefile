RELEASE ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

test:
	go test ./...

test/cov/report:
	go test ./... -covermode=set -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

tools/lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0

lint: tools/lint
	golangci-lint run

lint/fix: tools/lint
	golangci-lint run --fix

build:
	go build -ldflags="-w -s -X main.Release=$(RELEASE)" \
		-o=./bin/factory-runner-darwin-arm64 ./cmd/factory-runner
