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
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0

lint: tools/lint
	golangci-lint run

lint/fix: tools/lint
	golangci-lint run --fix

# macOS only (cgo + AppKit, arm64 native); Linux compiles only the no-UI stub.
# CGO_ENABLED=1 fails a cgo-less build rather than shipping that stub.
build:
	CGO_ENABLED=1 go build -ldflags="-w -s -X main.Release=$(RELEASE) -X main.SentryDSN=$(SENTRY_DSN) -X main.UpdatePublicKey=$(UPDATE_PUBLIC_KEY)" \
		-o=./bin/kobo-gateway-darwin-arm64 ./cmd/kobo-gateway

# Packages KoboGateway.app + .dmg plus the raw binary into ./dist/kobo-gateway/.
dist: build
	RELEASE=$(RELEASE) ./cmd/kobo-gateway/assets/package.sh
