# darwinkit's AppKit bridge SIGABRTs under Go 1.25+
# (https://github.com/progrium/darwinkit/issues/286); go.mod can't downgrade,
# so force the toolchain. Bump once fixed upstream (ADR 0015).
export GOTOOLCHAIN := go1.24.13

RELEASE ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
# Empty for dev builds: initSentry then skips sentry.Init.
SENTRY_DSN ?=

test:
	go test ./...

test/cov/report:
	go test ./... -covermode=set -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

test/cov: test/cov/report
	go tool cover -html=coverage.out -o=coverage.html
	open ./coverage.html

tools/lint:
	# auto for the tool build only: golangci-lint may need a newer Go than the
	# pin. Builds/tests still use the pinned toolchain.
	GOTOOLCHAIN=auto go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2

lint: tools/lint
	golangci-lint run

lint/fix: tools/lint
	golangci-lint run --fix

# macOS only (cgo + AppKit, arm64 native); Linux compiles only the no-UI stub.
build:
	go build -ldflags="-w -s -X main.Release=$(RELEASE) -X main.SentryDSN=$(SENTRY_DSN)" \
		-o=./bin/kobo-gateway-darwin-arm64 ./cmd/kobo-gateway

# Packages KoboGateway.app + .dmg plus the raw binary into ./dist/kobo-gateway/.
dist: build
	RELEASE=$(RELEASE) ./cmd/kobo-gateway/assets/package.sh
