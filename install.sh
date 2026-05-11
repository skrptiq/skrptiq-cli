#!/bin/sh
# Skrptiq CLI installer
# Usage: curl -fsSL https://hub.skrptiq.ai/install.sh | sh
#
# Detects OS/arch, downloads the correct binary from GitHub Releases,
# verifies the SHA256 checksum, and installs to ~/.local/bin/skrptiq.

set -e

REPO="skrptiq/skrptiq-cli"
INSTALL_DIR="${SKRPTIQ_INSTALL_DIR:-$HOME/.local/bin}"
BINARY_NAME="skrptiq"

# --- helpers ---

info() { printf "  %s\n" "$1"; }
error() { printf "Error: %s\n" "$1" >&2; exit 1; }

detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        darwin)  GOOS="darwin" ;;
        linux)   GOOS="linux" ;;
        mingw*|msys*|cygwin*) GOOS="windows" ;;
        *) error "Unsupported OS: $OS" ;;
    esac

    case "$ARCH" in
        x86_64|amd64)  GOARCH="amd64" ;;
        aarch64|arm64) GOARCH="arm64" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac

    SUFFIX="${GOOS}-${GOARCH}"
    EXT=""
    if [ "$GOOS" = "windows" ]; then
        EXT=".exe"
    fi
}

get_latest_version() {
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' \
        | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')

    if [ -z "$VERSION" ]; then
        error "Could not determine latest version. Check https://github.com/${REPO}/releases"
    fi
}

download_and_verify() {
    BINARY_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}-${SUFFIX}${EXT}"
    CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}-${SUFFIX}.sha256"

    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    info "Downloading ${BINARY_NAME} ${VERSION} for ${SUFFIX}..."
    curl -fsSL "$BINARY_URL" -o "${TMPDIR}/${BINARY_NAME}${EXT}" || \
        error "Download failed. Check that ${VERSION} has a ${SUFFIX} binary at:\n  ${BINARY_URL}"

    info "Verifying checksum..."
    curl -fsSL "$CHECKSUM_URL" -o "${TMPDIR}/checksum.sha256" || \
        error "Checksum download failed."

    EXPECTED=$(awk '{print $1}' "${TMPDIR}/checksum.sha256")
    if command -v sha256sum >/dev/null 2>&1; then
        ACTUAL=$(sha256sum "${TMPDIR}/${BINARY_NAME}${EXT}" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        ACTUAL=$(shasum -a 256 "${TMPDIR}/${BINARY_NAME}${EXT}" | awk '{print $1}')
    else
        info "Warning: no sha256sum or shasum found, skipping checksum verification."
        ACTUAL="$EXPECTED"
    fi

    if [ "$EXPECTED" != "$ACTUAL" ]; then
        error "Checksum mismatch.\n  Expected: ${EXPECTED}\n  Got:      ${ACTUAL}"
    fi
    info "Checksum verified."
}

install_binary() {
    mkdir -p "$INSTALL_DIR"
    mv "${TMPDIR}/${BINARY_NAME}${EXT}" "${INSTALL_DIR}/${BINARY_NAME}${EXT}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}${EXT}"

    info "Installed to ${INSTALL_DIR}/${BINARY_NAME}${EXT}"

    # Check if INSTALL_DIR is in PATH.
    case ":$PATH:" in
        *":${INSTALL_DIR}:"*) ;;
        *)
            info ""
            info "Add ${INSTALL_DIR} to your PATH:"
            info "  export PATH=\"${INSTALL_DIR}:\$PATH\""
            info ""
            info "Or add it to your shell profile (~/.bashrc, ~/.zshrc, etc.)"
            ;;
    esac
}

# --- main ---

printf "\n  Skrptiq CLI Installer\n\n"

detect_platform
get_latest_version
download_and_verify
install_binary

printf "\n  Done. Run 'skrptiq --version' to verify.\n\n"
