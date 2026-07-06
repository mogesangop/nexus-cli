#!/usr/bin/env bash
# Cross-build nexus-cli for Linux/macOS/Windows on amd64/arm64.
# CGO is off (pure Go), so no C cross-toolchain is needed.
# Produces per-platform archives (+ .sha256) in dist/ using npm platform/arch
# conventions so the npm postinstall shim can map them directly:
#   nexus-cli-<platform>-<arch>.tar.gz | .zip
# platform: linux | darwin | win32 ; arch: x64 | arm64
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

mkdir -p dist/.build

LDFLAGS="-s -w -X main.version=${VERSION}"

build_one() {
  local goos="$1" goarch="$2" suffix="$3" ext="$4"
  local out="dist/.build/nexus-cli${ext}"

  echo "==> build ${suffix} (${goos}/${goarch})"
  GOOS="${goos}" GOARCH="${goarch}" go build -trimpath -ldflags "${LDFLAGS}" \
    -o "${out}" ./cmd/nexus-cli

  local archive
  if [ "${goos}" = "windows" ]; then
    archive="dist/nexus-cli-${suffix}.zip"
    ( cd dist/.build && zip -q "../$(basename "${archive}")" "nexus-cli${ext}" )
  else
    archive="dist/nexus-cli-${suffix}.tar.gz"
    tar -C dist/.build -czf "${archive}" "nexus-cli${ext}"
  fi
  ( cd dist && sha256sum "$(basename "${archive}")" > "$(basename "${archive}").sha256" )
}

build_one linux   amd64 linux-x64    ""
build_one linux   arm64 linux-arm64  ""
build_one darwin  amd64 darwin-x64   ""
build_one darwin  arm64 darwin-arm64 ""
build_one windows amd64 win32-x64    ".exe"
build_one windows arm64 win32-arm64  ".exe"

rm -rf dist/.build

echo "==> built:"
ls -lh dist/
