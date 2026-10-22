#!/usr/bin/env bash
# Developer helper.
#   scripts/run.sh build      build ./build/watchd
#   scripts/run.sh install    go install with the version stamped in
#   scripts/run.sh test       run the test suite with the race detector
#   scripts/run.sh dev ARGS   run the CLI from source (go run . ARGS)
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS="-s -w -X github.com/omar0ali/watchd/internal/version.Version=${VERSION}"

case "${1:-}" in
build)
    go build -ldflags "$LDFLAGS" -o build/watchd .
    echo "built build/watchd (${VERSION})"
    ;;
install)
    go install -ldflags "$LDFLAGS" .
    echo "installed watchd (${VERSION}) to $(go env GOBIN || true)$(go env GOPATH)/bin"
    ;;
test)
    go vet ./...
    go test -race ./...
    ;;
dev)
    shift
    go run . "$@"
    ;;
*)
    echo "usage: scripts/run.sh build|install|test|dev [args...]" >&2
    exit 1
    ;;
esac
