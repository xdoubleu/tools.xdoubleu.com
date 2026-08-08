RELEASE ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

run:
	go run ./cmd/gateway

build:
	go build -ldflags="-w -s -X main.Release=$(RELEASE)" -o=./bin/gateway ./cmd/gateway

test:
	go test ./...

test/cov/report:
	go test ./... -covermode=set -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

test/cov: test/cov/report
	go tool cover -html=coverage.out -o=coverage.html
	open ./coverage.html

lint:
	go vet ./...
	gofmt -l .

lint/fix:
	gofmt -w .
