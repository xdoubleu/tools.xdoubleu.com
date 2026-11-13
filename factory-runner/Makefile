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

# CGO_ENABLED=1 fails a cgo-less build rather than shipping the no-menu-bar stub.
build:
	CGO_ENABLED=1 go build -ldflags="-w -s -X main.Release=$(RELEASE) -X main.UpdatePublicKey=$(UPDATE_PUBLIC_KEY)" \
		-o=./bin/factory-runner-darwin-arm64 ./cmd/factory-runner

# Packages FactoryRunner.app + the drag-to-Applications .dmg plus the raw binary
# into ./dist/.  (macOS only: sips/iconutil/hdiutil.)
dist: build
	RELEASE=$(RELEASE) ./cmd/factory-runner/assets/package.sh

image:
	container build -t factory-job:dev -f Containerfile ..
