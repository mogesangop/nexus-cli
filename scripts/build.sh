#!/usr/bin/env bash
# Cross-build nexus-cli for Linux amd64 and arm64.
# CGO is off (pure Go), so no C cross-toolchain is needed.
# Usage: scripts/build.sh [version]   (version defaults to git describe)
set -euo pipefail

VERSION="${1:-$(git describe --tags --always 2>/dev/null || echo dev)}"
export CGO_ENABLED=0
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

cd "$(git rev-parse --show-toplevel 2>/dev/null || echo .)"

echo "==> vet"
go vet ./...

echo "==> test"
go test ./...

mkdir -p dist

LDFLAGS="-s -w -X main.version=${VERSION}"

for ARCH in amd64 arm64; do
  OUT="dist/nexus-cli-linux-${ARCH}"
  echo "==> build ${OUT} (linux/${ARCH})"
  GOOS=linux GOARCH="${ARCH}" go build -trimpath -ldflags "${LDFLAGS}" \
    -o "${OUT}" ./cmd/nexus-cli
  (cd dist && sha256sum "nexus-cli-linux-${ARCH}" > "nexus-cli-linux-${ARCH}.sha256")
done

echo "==> built:"
ls -lh dist/
