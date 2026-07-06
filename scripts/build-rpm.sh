#!/usr/bin/env bash
# Build GPG-signed RPMs for nexus-cli and a yum repo index from prebuilt
# linux amd64/arm64 binaries.
#
# Env (required for signing):
#   GPG_PRIVATE_KEY  - ASCII-armored private key
#   GPG_PASSPHRASE   - key passphrase (empty if none)
#   GPG_KEY_ID       - signing key id
#
# Requires: fpm, rpm, rpmsign, createrepo, gpg
# Usage: scripts/build-rpm.sh <version> <amd64-binary> <arm64-binary>
set -euo pipefail

VERSION="${1:?version required}"
AMD64_BIN="${2:?amd64 binary required}"
ARM64_BIN="${3:?arm64 binary required}"
VERSION="${VERSION#v}"

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo .)"
DIST="${ROOT}/dist"
STAGE="${DIST}/.rpm-stage"
REPO_DIR="${DIST}/yum-repo"

rm -rf "${STAGE}" "${REPO_DIR}"
mkdir -p "${STAGE}/x86_64" "${STAGE}/aarch64" "${REPO_DIR}"

cp "${AMD64_BIN}" "${STAGE}/x86_64/nexus-cli"
cp "${ARM64_BIN}" "${STAGE}/aarch64/nexus-cli"
chmod +x "${STAGE}/x86_64/nexus-cli" "${STAGE}/aarch64/nexus-cli"

URL="https://github.com/mogesangop/nexus-cli"
DESC="CLI for governing Nexus Repository 3.76 guest/anonymous access"

build_rpm() {
  local arch="$1"
  fpm -s dir -t rpm \
    -n nexus-cli -v "${VERSION}" --iteration 1 --rpm-dist el9 \
    --url "${URL}" --description "${DESC}" --license MIT \
    --vendor mogesangop -m "mogesangop" \
    -a "${arch}" -p "${DIST}/" \
    -C "${STAGE}/${arch}" \
    nexus-cli=/usr/bin/nexus-cli
}

echo "==> build x86_64 rpm"
build_rpm x86_64
echo "==> build aarch64 rpm"
build_rpm aarch64

if [ -z "${GPG_PRIVATE_KEY:-}" ] || [ -z "${GPG_KEY_ID:-}" ]; then
  echo "error: GPG_PRIVATE_KEY and GPG_KEY_ID must be set to sign rpms" >&2
  exit 1
fi

echo "==> import GPG key"
echo "${GPG_PRIVATE_KEY}" | gpg --batch --import
gpg --batch --armor --export "${GPG_KEY_ID}" > "${DIST}/RPM-GPG-KEY-nexus-cli"

echo "==> configure gpg for non-interactive signing"
mkdir -p "${HOME}/.gnupg" && chmod 700 "${HOME}/.gnupg"
printf 'use-agent\npinentry-mode loopback\nbatch\n' > "${HOME}/.gnupg/gpg.conf"
printf 'allow-loopback-pinentry\ndefault-cache-ttl 7200\nmax-cache-ttl 7200\n' > "${HOME}/.gnupg/gpg-agent.conf"
gpgconf --kill gpg-agent 2>/dev/null || true

# Write passphrase to a temp file so we can pass it to gpg via
# --passphrase-file (avoids command-line exposure and special-char issues).
# For passphrase-less keys the file is empty and gpg signs without it.
PASS_FILE="$(mktemp)"
printf '%s' "${GPG_PASSPHRASE:-}" > "${PASS_FILE}"
chmod 600 "${PASS_FILE}"
trap 'rm -f "${PASS_FILE}"' EXIT

GPG_BIN="$(command -v gpg || echo gpg)"

# Override %__gpg_sign_cmd to inject --pinentry-mode loopback and
# --passphrase-file so gpg never needs a TTY or gpg-agent to unlock the key.
# %%{__filename} and %%{__signature_filename} use double-% so rpm expands
# them at sign time (they are runtime-only macros). A single % would be
# evaluated at macro-file load time when __filename is undefined, leaving a
# literal '%{__filename}' string that gpg can't open.
cat > "${HOME}/.rpmmacros" <<EOF
%_signature gpg
%_gpg_name ${GPG_KEY_ID}
%__gpg_sign_cmd ${GPG_BIN} --batch --no-verbose --yes --no-tty --pinentry-mode loopback --passphrase-file ${PASS_FILE} -u ${GPG_KEY_ID} -o %%{__signature_filename} --detach-sign %%{__filename}
EOF

echo "==> sign rpms"
rpm --addsign "${DIST}"/nexus-cli-*.rpm
echo "==> verify signatures"
rpm --checksig "${DIST}"/nexus-cli-*.rpm

echo "==> createrepo"
cp "${DIST}"/nexus-cli-*.rpm "${REPO_DIR}/"
cp "${DIST}/RPM-GPG-KEY-nexus-cli" "${REPO_DIR}/"
createrepo "${REPO_DIR}"

echo "==> built:"
ls -lh "${DIST}"/nexus-cli-*.rpm
