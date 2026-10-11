#!/bin/sh
# rommate installer: downloads the right binary for this machine from GitHub
# Releases, verifies its SHA-256 checksum and installs it.
#
#   curl -fsSL https://arunoruto.github.io/rommate/install.sh | sh
#
# Options (environment variables):
#   ROMMATE_VERSION      release tag to install, e.g. v0.1.0 (default: latest)
#   ROMMATE_INSTALL_DIR  where to put the binary (default: ~/.local/bin)
#   ROMMATE_DOWNLOAD_URL base URL holding the release assets (mirrors, testing)
#
#   curl -fsSL https://arunoruto.github.io/rommate/install.sh | ROMMATE_VERSION=v0.1.0 sh

set -eu

REPO="arunoruto/rommate"
VERSION="${ROMMATE_VERSION:-latest}"
INSTALL_DIR="${ROMMATE_INSTALL_DIR:-$HOME/.local/bin}"

say() { printf '%s\n' "$*"; }
fail() { printf 'rommate installer: %s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || fail "'$1' is required but was not found"; }

detect_target() {
	os="$(uname -s)"
	arch="$(uname -m)"
	case "$os" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	MINGW* | MSYS* | CYGWIN*)
		fail "on Windows, download rommate-windows-amd64.exe from https://github.com/$REPO/releases/latest" ;;
	*) fail "unsupported operating system: $os" ;;
	esac
	case "$arch" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) fail "unsupported architecture: $arch" ;;
	esac
	if [ "$os" = darwin ] && [ "$arch" = amd64 ]; then
		fail "Intel Macs are not supported; rommate ships macOS builds for Apple Silicon only"
	fi
	ASSET="rommate-$os-$arch"
}

download() { # url dest
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL --retry 3 -o "$2" "$1"
	elif command -v wget >/dev/null 2>&1; then
		wget -q -O "$2" "$1"
	else
		fail "either curl or wget is required"
	fi
}

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | cut -d' ' -f1
	else
		fail "sha256sum or shasum is required to verify the download"
	fi
}

main() {
	need uname
	detect_target

	if [ -n "${ROMMATE_DOWNLOAD_URL:-}" ]; then
		base="${ROMMATE_DOWNLOAD_URL%/}"
	elif [ "$VERSION" = latest ]; then
		base="https://github.com/$REPO/releases/latest/download"
	else
		base="https://github.com/$REPO/releases/download/$VERSION"
	fi

	tmp="$(mktemp -d 2>/dev/null || mktemp -d -t rommate)"
	trap 'rm -rf "$tmp"' EXIT INT TERM

	say "Downloading $ASSET ($VERSION)…"
	download "$base/$ASSET" "$tmp/$ASSET" || fail "could not download $base/$ASSET"
	download "$base/checksums.txt" "$tmp/checksums.txt" || fail "could not download checksums.txt"

	expected="$(awk -v f="$ASSET" '$2 == f { print $1 }' "$tmp/checksums.txt")"
	[ -n "$expected" ] || fail "no checksum for $ASSET in checksums.txt"
	actual="$(sha256 "$tmp/$ASSET")"
	[ "$expected" = "$actual" ] || fail "checksum mismatch for $ASSET (expected $expected, got $actual)"

	mkdir -p "$INSTALL_DIR"
	chmod +x "$tmp/$ASSET"
	mv "$tmp/$ASSET" "$INSTALL_DIR/rommate"
	if [ "$(uname -s)" = Darwin ]; then
		# Downloads via curl are not quarantined, but clear it in case.
		xattr -d com.apple.quarantine "$INSTALL_DIR/rommate" 2>/dev/null || true
	fi

	say "Installed $("$INSTALL_DIR/rommate" --version) to $INSTALL_DIR/rommate"
	case ":$PATH:" in
	*":$INSTALL_DIR:"*) say "Run 'rommate init --server https://… --token rmm_…' to get started." ;;
	*) say "Note: $INSTALL_DIR is not on your PATH. Add it, e.g.: export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
	esac
}

main "$@"
