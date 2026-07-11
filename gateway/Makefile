# Pinned below go.mod's stated minimum: darwinkit's AppKit bridge SIGABRTs
# under Go 1.25+ (https://github.com/progrium/darwinkit/issues/286), so this
# forces the exact toolchain regardless of what `go` is on PATH — a bare
# go.mod `go` line only sets a minimum and won't downgrade a newer ambient
# Go. Bump only once that issue is fixed upstream.
export GOTOOLCHAIN := go1.24.13

RELEASE ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

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

# The menu bar needs cgo + the real AppKit/Xcode SDK, so this only builds on
# macOS (arm64 native — no cross-compile). See ../.github/workflows/build-gateway.yml
# (macos-14 runner) for CI; on Linux only the no-UI stub (menubar_other.go) compiles.
build:
	go build -ldflags="-w -s -X main.Release=$(RELEASE)" \
		-o=./bin/kobo-gateway-darwin-arm64 ./cmd/kobo-gateway

# Packages the built binary into KoboGateway.app + kobo-gateway.dmg for the
# download button, alongside the raw binary the self-updater fetches.
# Outputs to ./dist/gateway/. macOS only (hdiutil).
dist: build
	RELEASE=$(RELEASE) ./cmd/kobo-gateway/assets/package.sh
