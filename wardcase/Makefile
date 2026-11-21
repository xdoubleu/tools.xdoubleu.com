test:
	go test ./...

test/cov/report:
	go test ./... -covermode=set -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

test/cov/diff:
	go test ./... -covermode=set -coverprofile=coverage.out
	../scripts/diff_coverage.sh go coverage.out

tools/lint:
	@../scripts/require_tools.sh golangci-lint

lint: tools/lint
	golangci-lint run

lint/fix: tools/lint
	golangci-lint run --fix

# VERSION is the release whose base image (wardcase/Containerfile) this binary runs.
VERSION ?= dev

build:
	go build -ldflags "-X main.Version=$(VERSION)" -o bin/wardcase ./cmd/wardcase
