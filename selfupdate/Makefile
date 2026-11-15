test:
	go test ./...

test/cov/report:
	go test ./... -covermode=set -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

test/cov/diff:
	go test ./... -covermode=set -coverprofile=coverage.out
	../scripts/diff_coverage.sh go coverage.out

tools/lint:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0

lint: tools/lint
	golangci-lint run

lint/fix: tools/lint
	golangci-lint run --fix
