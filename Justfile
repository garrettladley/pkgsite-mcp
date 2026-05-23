set quiet := true
set shell := ["bash", "-cu"]

export GOCACHE := justfile_directory() + "/.cache/go-build"
export GOLANGCI_LINT_CACHE := justfile_directory() + "/.cache/golangci-lint"
export GOFLAGS := "-buildvcs=false"
export GOROOT := ""

oapi_codegen_version := env("OAPI_CODEGEN_VERSION", "v2.7.0")

[private]
default:
    just --list

# install go dependencies
[group('dev')]
install:
    go mod download

# tidy go modules
[group('dev')]
tidy:
    go mod tidy

# generate pkgsite API client
[group('codegen')]
generate:
    go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@{{ oapi_codegen_version }} -config internal/pkgsiteapi/oapi-codegen.yaml internal/pkgsiteapi/openapi.json

# verify generated code is up to date
[group('ci')]
generate-verify: generate
    git diff --exit-code -- internal/pkgsiteapi/client.gen.go

# format code
[group('dev')]
fmt:
    golangci-lint fmt

# modernize Go code
[group('dev')]
go-fix:
    go fix ./...

# verify go fix does not produce changes
[group('ci')]
go-fix-verify:
    #!/usr/bin/env bash
    set -euo pipefail
    go fix ./...
    git diff --exit-code || (echo "go fix produced changes - run 'go fix ./...' locally and commit" && exit 1)

# run linter
[group('dev')]
lint:
    go fix ./...
    golangci-lint run --path-mode=abs --config=".golangci.yml" --timeout=5m

# run linter with auto-fix
[group('dev')]
lint-fix:
    go fix ./...
    golangci-lint run --path-mode=abs --config=".golangci.yml" --timeout=5m --fix

# build the MCP server
[group('build')]
build:
    go build -ldflags="-s -w" -o .cache/bin/pkgsite-mcp ./cmd/pkgsite-mcp

# run Go vet
[group('ci')]
vet:
    go vet ./...

# build the Docker image
[group('docker')]
docker-build:
    docker build -t pkgsite-mcp:local .

# deploy to Fly
[group('deploy')]
deploy:
    flyctl deploy --remote-only

# start local Redis
[group('docker')]
up:
    docker compose up

# start local Redis in the background
[group('docker')]
up-detached:
    docker compose up -d

# stop local Redis
[group('docker')]
down:
    docker compose down

# show Redis logs
[group('docker')]
logs:
    docker compose logs -f redis

# run the HTTP MCP server locally
[group('dev')]
serve-http: build
    KV_REDIS_URL="${KV_REDIS_URL:-redis://localhost:9736/0}" .cache/bin/pkgsite-mcp serve --transport http --addr :8080

# run tests
[group('dev')]
test:
    go test -v -race ./...

# run tests with coverage output for CI
[group('ci')]
test-ci:
    go test -v -json -race -coverpkg=./... -covermode=atomic -coverprofile=coverage.txt ./... -timeout 5m

# run local verification
[group('dev')]
check: generate fmt lint vet test build

alias b := build
alias t := test
