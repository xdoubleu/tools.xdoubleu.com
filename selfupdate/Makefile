# A job sandbox has the flake's tools at this profile but not on PATH: append it
# (a `nix develop` shell still wins).
export PATH := $(PATH):/nix/var/nix/profiles/devshell/bin

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
