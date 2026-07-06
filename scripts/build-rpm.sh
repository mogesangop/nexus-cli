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
printf 'allow-loopback-pinentry\n' > "${HOME}/.gnupg/gpg-agent.conf"
gpgconf --kill gpg-agent 2>/dev/null || true

# If the key has a passphrase, preset it into gpg-agent so rpm --addsign
# (which calls gpg non-interactively) can unlock the key without a TTY.
# No-op for passphrase-less keys.
if [ -n "${GPG_PASSPHRASE:-}" ]; then
  KEYGRIP=$(gpg --list-secret-keys --with-keygrip "${GPG_KEY_ID}" 2>/dev/null \
            | awk '/Keygrip/ {print $3; exit}')
  for preset in /usr/lib/gnupg2/gpg-preset-passphrase \
                /usr/libexec/gpg-preset-passphrase; do
    if [ -x "$preset" ] && [ -n "${KEYGRIP}" ]; then
      printf '%s' "${GPG_PASSPHRASE}" | "$preset" --passphrase-fd 0 --preset "${KEYGRIP}" || true
      break
    fi
  done
fi

echo "==> sign rpms"
# Do NOT override %__gpg_sign_cmd: rpm's built-in default uses the correct
# platform-specific filename macro. Overriding it broke on Ubuntu's rpm
# (%{__filename} was passed literally to gpg). We only set the key identity.
cat > "${HOME}/.rpmmacros" <<EOF
%_signature gpg
%_gpg_name ${GPG_KEY_ID}
EOF
rpm --addsign "${DIST}"/nexus-cli-*.rpm
rpm --checksig "${DIST}"/nexus-cli-*.rpm

echo "==> createrepo"
cp "${DIST}"/nexus-cli-*.rpm "${REPO_DIR}/"
cp "${DIST}/RPM-GPG-KEY-nexus-cli" "${REPO_DIR}/"
createrepo "${REPO_DIR}"

echo "==> built:"
ls -lh "${DIST}"/nexus-cli-*.rpm
