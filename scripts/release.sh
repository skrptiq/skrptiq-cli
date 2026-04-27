#!/bin/sh
# Release script — finds next version tag, creates it, pushes to trigger build.
#
# Usage:
#   ./scripts/release.sh              # auto-increment patch (v0.1.0 → v0.1.1)
#   ./scripts/release.sh minor        # increment minor (v0.1.1 → v0.2.0)
#   ./scripts/release.sh major        # increment major (v0.2.0 → v1.0.0)
#   ./scripts/release.sh v0.3.0       # explicit version
#   ./scripts/release.sh v0.3.0-beta.1  # pre-release

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()  { printf "${BLUE}info${NC}  %s\n" "$1"; }
ok()    { printf "${GREEN}ok${NC}    %s\n" "$1"; }
warn()  { printf "${YELLOW}warn${NC}  %s\n" "$1"; }
error() { printf "${RED}error${NC} %s\n" "$1" >&2; exit 1; }

# Ensure we're on main and clean.
BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$BRANCH" != "main" ]; then
    error "Must be on main branch (currently on ${BRANCH})"
fi

if [ -n "$(git status --porcelain)" ]; then
    error "Working directory is not clean. Commit or stash changes first."
fi

# Get the latest tag.
LATEST=$(git tag -l 'v*' --sort=-v:refname | head -1)
if [ -z "$LATEST" ]; then
    LATEST="v0.0.0"
    info "No existing tags found. Starting from v0.0.0"
else
    info "Latest tag: ${LATEST}"
fi

# Parse major.minor.patch from latest tag.
# Strip 'v' prefix and any pre-release suffix.
BASE=$(echo "$LATEST" | sed 's/^v//' | sed 's/-.*//')
MAJOR=$(echo "$BASE" | cut -d. -f1)
MINOR=$(echo "$BASE" | cut -d. -f2)
PATCH=$(echo "$BASE" | cut -d. -f3)

# Determine next version.
ARG="${1:-patch}"
case "$ARG" in
    major)
        MAJOR=$((MAJOR + 1))
        MINOR=0
        PATCH=0
        NEXT="v${MAJOR}.${MINOR}.${PATCH}"
        ;;
    minor)
        MINOR=$((MINOR + 1))
        PATCH=0
        NEXT="v${MAJOR}.${MINOR}.${PATCH}"
        ;;
    patch)
        PATCH=$((PATCH + 1))
        NEXT="v${MAJOR}.${MINOR}.${PATCH}"
        ;;
    v*)
        # Explicit version.
        NEXT="$ARG"
        ;;
    *)
        error "Unknown argument: ${ARG}. Use major, minor, patch, or v<version>"
        ;;
esac

# Confirm.
echo ""
info "Current: ${LATEST}"
info "Next:    ${NEXT}"
echo ""

printf "Create tag ${GREEN}${NEXT}${NC} and push to trigger release build? [y/N] "
read -r CONFIRM
if [ "$CONFIRM" != "y" ] && [ "$CONFIRM" != "Y" ]; then
    echo "Cancelled."
    exit 0
fi

# Pull latest.
info "Pulling latest..."
git pull --rebase origin main

# Create and push tag.
info "Creating tag ${NEXT}..."
git tag -a "$NEXT" -m "Release ${NEXT}"

info "Pushing tag..."
git push origin "$NEXT"

ok "Tag ${NEXT} pushed. GitHub Actions release workflow will build platform binaries."
echo ""
echo "  Monitor: https://github.com/skrptiq/skrptiq-cli/actions"
echo "  Release: https://github.com/skrptiq/skrptiq-cli/releases/tag/${NEXT}"
