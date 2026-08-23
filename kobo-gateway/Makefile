# Pinned below go.mod's stated minimum: darwinkit's AppKit bridge SIGABRTs
# under Go 1.25+ (https://github.com/progrium/darwinkit/issues/286), so this
# forces the exact toolchain regardless of what `go` is on PATH — a bare
# go.mod `go` line only sets a minimum and won't downgrade a newer ambient
# Go. Bump only once that issue is fixed upstream.
export GOTOOLCHAIN := go1.24.13

RELEASE ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
# Empty by default (dev builds, go test): initSentry (main.go) then skips
# sentry.Init entirely, so nothing is ever reported off a dev machine.
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
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.0

lint: tools/lint
	golangci-lint run

lint/fix: tools/lint
	golangci-lint run --fix

# The menu bar needs cgo + the real AppKit/Xcode SDK, so this only builds on
# macOS (arm64 native — no cross-compile). See
# ../.github/workflows/build-kobo-gateway.yml (macos-14 runner) for CI; on
# Linux only the no-UI stub (menubar_other.go) compiles.
build:
	go build -ldflags="-w -s -X main.Release=$(RELEASE) -X main.SentryDSN=$(SENTRY_DSN)" \
		-o=./bin/kobo-gateway-darwin-arm64 ./cmd/kobo-gateway

# Packages the built binary into KoboGateway.app + kobo-gateway.dmg for the
# download button, alongside the raw binary the self-updater fetches.
# Outputs to ./dist/kobo-gateway/. macOS only (hdiutil).
dist: build
	RELEASE=$(RELEASE) ./cmd/kobo-gateway/assets/package.sh
