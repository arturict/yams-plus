#!/bin/sh
set -eu

# /etc/os-release is sourced below and defines VERSION itself, so the
# requested release must not be held in a variable of that name.
RELEASE_VERSION="${YAMSPLUS_VERSION:-}"
BASE_URL="${YAMSPLUS_RELEASE_BASE_URL:-https://github.com/arturict/yams-plus/releases/download}"
LOCAL_DIR=""
SKIP_SIGNATURE=false

usage() {
  echo "Usage: sudo sh ./install.sh [--version VERSION] [--local-dir DIR] [--skip-signature]"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version) RELEASE_VERSION=$2; shift 2 ;;
    --local-dir) LOCAL_DIR=$2; shift 2 ;;
    --skip-signature) SKIP_SIGNATURE=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

[ "$(id -u)" -eq 0 ] || { echo "Please run this bootstrapper with sudo." >&2; exit 1; }
[ "$(uname -m)" = "x86_64" ] || { echo "Beta supports amd64 only." >&2; exit 1; }

# Fixed, trusted OS metadata path.
# shellcheck disable=SC1091
. /etc/os-release
case "${ID}:${VERSION_ID}" in
  debian:13|ubuntu:24.04) ;;
  *) echo "Beta supports Debian 13 and Ubuntu 24.04; found ${ID} ${VERSION_ID}." >&2; exit 1 ;;
esac

for command in curl sha256sum tar; do
  command -v "$command" >/dev/null 2>&1 || { echo "Missing required command: $command" >&2; exit 1; }
done

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is missing. Installing Docker Engine from Docker's official apt repository."
  apt-get update
  apt-get install -y ca-certificates curl
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL "https://download.docker.com/linux/${ID}/gpg" -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/${ID} ${VERSION_CODENAME} stable" > /etc/apt/sources.list.d/docker.list
  apt-get update
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
fi

docker version >/dev/null
docker compose version >/dev/null

WORK_DIR=$(mktemp -d /tmp/yamsplus-bootstrap.XXXXXX)
trap 'rm -rf "$WORK_DIR"' EXIT INT TERM

if [ -n "$LOCAL_DIR" ]; then
  cp "$LOCAL_DIR"/yamsplus_*_linux_amd64.tar.gz "$WORK_DIR"/
  cp "$LOCAL_DIR"/checksums.txt "$WORK_DIR"/
  [ ! -f "$LOCAL_DIR/checksums.txt.sigstore.json" ] || cp "$LOCAL_DIR"/checksums.txt.sigstore.json "$WORK_DIR"/
else
  [ -n "$RELEASE_VERSION" ] || { echo "--version is required for a release install." >&2; exit 1; }
  TAG="v${RELEASE_VERSION#v}"
  ASSET="yamsplus_${RELEASE_VERSION#v}_linux_amd64.tar.gz"
  RELEASE_URL="${BASE_URL}/${TAG}"
  curl -fL "${RELEASE_URL}/${ASSET}" -o "$WORK_DIR/$ASSET"
  curl -fL "${RELEASE_URL}/checksums.txt" -o "$WORK_DIR/checksums.txt"
  curl -fL "${RELEASE_URL}/checksums.txt.sigstore.json" -o "$WORK_DIR/checksums.txt.sigstore.json"
fi

if [ "$SKIP_SIGNATURE" = false ]; then
  command -v cosign >/dev/null 2>&1 || {
    echo "cosign is required to verify the signed release. Install it from https://docs.sigstore.dev/cosign/system_config/installation/" >&2
    exit 1
  }
  if [ ! -f "$WORK_DIR/checksums.txt.sigstore.json" ]; then
    echo "The signed checksum bundle is missing. Refusing the install." >&2
    exit 1
  fi
  cosign verify-blob \
    --bundle "$WORK_DIR/checksums.txt.sigstore.json" \
    --certificate-identity-regexp '^https://github.com/arturict/yams-plus/.github/workflows/release.yml@refs/tags/v' \
    --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
    "$WORK_DIR/checksums.txt"
else
  echo "WARNING: signature verification was explicitly skipped. Use only for a local development artifact."
fi

cd "$WORK_DIR"
ARCHIVE=$(find . -maxdepth 1 -name 'yamsplus_*_linux_amd64.tar.gz' -print -quit)
[ -n "$ARCHIVE" ] || { echo "Release archive is missing." >&2; exit 1; }
grep -E "  $(basename "$ARCHIVE")\$" checksums.txt | sha256sum --check -
tar -xzf "$ARCHIVE" yamsplus
install -m 0755 yamsplus /usr/local/bin/yamsplus

echo "Installed $(/usr/local/bin/yamsplus version). Starting the friendly part."
exec /usr/local/bin/yamsplus install
