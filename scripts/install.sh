#!/bin/sh
# Skrptiq CLI installer
# Usage: curl -fsSL https://hub.skrptiq.ai/install.sh | sh
#
# Detects OS and architecture, downloads the correct binary from
# GitHub Releases, verifies the SHA256 checksum, and installs to
# ~/.local/bin/skrptiq.

set -e

REPO="skrptiq/skrptiq-cli"
BINARY="skrptiq"
INSTALL_DIR="${SKRPTIQ_INSTALL_DIR:-$HOME/.local/bin}"

# Colours for output.
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()  { printf "${BLUE}info${NC}  %s\n" "$1"; }
ok()    { printf "${GREEN}ok${NC}    %s\n" "$1"; }
warn()  { printf "${YELLOW}warn${NC}  %s\n" "$1"; }
error() { printf "${RED}error${NC} %s\n" "$1" >&2; exit 1; }

# Detect OS.
detect_os() {
    case "$(uname -s)" in
        Darwin) echo "darwin" ;;
        Linux)  echo "linux" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) error "Unsupported OS: $(uname -s)" ;;
    esac
}

# Detect architecture.
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *) error "Unsupported architecture: $(uname -m)" ;;
    esac
}

# Get the latest release tag from GitHub.
latest_version() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//'
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//'
    else
        error "Neither curl nor wget found. Please install one."
    fi
}

# Download a file.
download() {
    local url="$1" dest="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "$dest" "$url"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$dest" "$url"
    fi
}

main() {
    local os arch version suffix ext="" url checksum_url tmp_dir

    os=$(detect_os)
    arch=$(detect_arch)
    version="${SKRPTIQ_VERSION:-$(latest_version)}"

    if [ -z "$version" ]; then
        error "Could not determine latest version. Set SKRPTIQ_VERSION manually."
    fi

    suffix="${os}-${arch}"
    if [ "$os" = "windows" ]; then
        ext=".exe"
    fi

    info "Installing skrptiq ${version} (${os}/${arch})"

    url="https://github.com/${REPO}/releases/download/${version}/${BINARY}-${suffix}${ext}"
    checksum_url="https://github.com/${REPO}/releases/download/${version}/${BINARY}-${suffix}.sha256"

    # Create temp directory.
    tmp_dir=$(mktemp -d)
    trap 'rm -rf "$tmp_dir"' EXIT

    # Download binary and checksum.
    info "Downloading ${url}"
    download "$url" "${tmp_dir}/${BINARY}${ext}" || error "Download failed. Check the version and platform."

    info "Downloading checksum"
    download "$checksum_url" "${tmp_dir}/checksum.sha256" || warn "Checksum not available — skipping verification."

    # Verify checksum if available.
    if [ -f "${tmp_dir}/checksum.sha256" ]; then
        info "Verifying checksum"
        cd "$tmp_dir"
        if command -v shasum >/dev/null 2>&1; then
            shasum -a 256 -c checksum.sha256 || error "Checksum verification failed."
        elif command -v sha256sum >/dev/null 2>&1; then
            sha256sum -c checksum.sha256 || error "Checksum verification failed."
        else
            warn "No checksum tool found — skipping verification."
        fi
        cd - >/dev/null
    fi

    # Install.
    mkdir -p "$INSTALL_DIR"
    cp "${tmp_dir}/${BINARY}${ext}" "${INSTALL_DIR}/${BINARY}${ext}"
    chmod +x "${INSTALL_DIR}/${BINARY}${ext}"

    ok "Installed to ${INSTALL_DIR}/${BINARY}${ext}"

    # Check if install dir is in PATH.
    case ":$PATH:" in
        *":${INSTALL_DIR}:"*) ;;
        *)
            warn "${INSTALL_DIR} is not in your PATH."
            echo ""
            echo "  Add it to your shell profile:"
            echo ""
            echo "    export PATH=\"${INSTALL_DIR}:\$PATH\""
            echo ""
            ;;
    esac

    ok "Run 'skrptiq' to get started."
}

main "$@"
