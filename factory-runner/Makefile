RELEASE ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

test:
	go test ./...

tools/mutation:
	go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0

# Mirrors api/'s target: gremlins on Go packages changed vs origin/main, mutants
# scoped to changed lines; fails if a changed line's mutant is SKIPPED.
test/mutation/diff: tools/mutation
	@pkgs=$$(python3 ../tools/diff_packages_go.py); \
	if [ -z "$$pkgs" ]; then \
		echo "No changed Go packages vs origin/main -- nothing to mutation-test."; \
	else \
		status=0; \
		report=$$(mktemp); \
		for pkg in $$pkgs; do \
			echo "Diff-scoped mutation testing: $$pkg"; \
			(cd "$$pkg" && \
				GOFLAGS="$$GOFLAGS -p=1 -count=1" GIT_CONFIG_COUNT=2 \
				GIT_CONFIG_KEY_0=diff.relative GIT_CONFIG_VALUE_0=true \
				GIT_CONFIG_KEY_1=diff.context GIT_CONFIG_VALUE_1=0 \
				gremlins unleash --diff origin/main --workers 1 \
					--output "$$report" "$$PWD") && \
			python3 ../tools/check_mutation_report_go.py "$$pkg" "$$report" || status=1; \
		done; \
		rm -f "$$report"; \
		exit $$status; \
	fi

test/cov/report:
	go test ./... -covermode=set -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

# The changed-line gate CI runs (>= 80% vs origin/main); `make test` does not.
test/cov/diff:
	go test ./... -covermode=set -coverprofile=coverage.out
	../scripts/diff_coverage.sh go coverage.out

tools/lint:
	../api/scripts/ensure_go_tool.sh golangci-lint github.com/golangci/golangci-lint/v2 github.com/golangci/golangci-lint/v2/cmd/golangci-lint 2.15.0

lint: tools/lint
	golangci-lint run

# CI lints on macOS; syscall field types differ per OS, so nolint directives can be unused there.
lint/darwin: tools/lint
	GOOS=darwin CGO_ENABLED=0 golangci-lint run

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
