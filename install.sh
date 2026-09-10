#!/usr/bin/env bash
set -e

# ==============================================================================
#  ⚡ omp-cost Installer
#  One-line install:
#    curl -fsSL https://raw.githubusercontent.com/faridlamaul/omp-cost/main/install.sh | bash
# ==============================================================================

REPO="faridlamaul/omp-cost"
BINARY="omp-cost"
INSTALL_DIR="${HOME}/.local/bin"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}⚡ Installing omp-cost...${NC}"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "${OS}" in
  darwin*) OS="darwin" ;;
  linux*)  OS="linux" ;;
  *)       echo -e "${RED}Error: Unsupported operating system: ${OS}${NC}" >&2; exit 1 ;;
esac

# Detect Architecture
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)            echo -e "${RED}Error: Unsupported architecture: ${ARCH}${NC}" >&2; exit 1 ;;
esac

mkdir -p "${INSTALL_DIR}"

# 1. Resolve latest release tag without hitting GitHub API rate limits
LATEST_TAG=$(basename "$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" 2>/dev/null || true)")
if [ "${LATEST_TAG}" = "releases" ] || [ "${LATEST_TAG}" = "latest" ] || [ -z "${LATEST_TAG}" ]; then
  # Fallback to API if redirect failed
  LATEST_TAG=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || true)
fi
DOWNLOADED=false

if [ -n "${LATEST_TAG}" ]; then
  TARBALL="omp-cost_${OS}_${ARCH}.tar.gz"
  DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/${TARBALL}"
  echo -e "📦 Fetching latest release ${LATEST_TAG} for ${OS}/${ARCH}..."

  TMP_DIR=$(mktemp -d)
  if curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_DIR}/${TARBALL}" 2>/dev/null; then
    tar -xzf "${TMP_DIR}/${TARBALL}" -C "${TMP_DIR}"
    if [ -f "${TMP_DIR}/${BINARY}" ]; then
      mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
      chmod +x "${INSTALL_DIR}/${BINARY}"
      DOWNLOADED=true
    fi
  fi
  rm -rf "${TMP_DIR}"
fi

# 2. Fallback: Build from source if Go is installed
if [ "${DOWNLOADED}" = false ]; then
  if command -v go >/dev/null 2>&1; then
    echo -e "${YELLOW}ℹ Release asset not yet published. Building from source via Go...${NC}"
    TMP_SRC=$(mktemp -d)
    git clone --depth 1 "https://github.com/${REPO}.git" "${TMP_SRC}" 2>/dev/null || true
    if [ -d "${TMP_SRC}" ] && [ -f "${TMP_SRC}/cmd/omp-cost/main.go" ]; then
      (cd "${TMP_SRC}" && go build -ldflags="-s -w -X main.version=${LATEST_TAG:-dev}" -o "${INSTALL_DIR}/${BINARY}" ./cmd/omp-cost)
      chmod +x "${INSTALL_DIR}/${BINARY}"
      DOWNLOADED=true
    fi
    rm -rf "${TMP_SRC}"
  fi
fi

if [ "${DOWNLOADED}" = false ]; then
  echo -e "${RED}Error: Could not install ${BINARY}. Please ensure 'go' is installed to build from source.${NC}" >&2
  exit 1
fi

echo -e "${GREEN}✓ Successfully installed ${BINARY} to ${INSTALL_DIR}/${BINARY}${NC}"

# Check PATH
if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
  echo -e "${YELLOW}⚠️  Note: ${INSTALL_DIR} is not currently in your \$PATH.${NC}"
  echo -e "Add this to your ~/.zshrc or ~/.bashrc:"
  echo -e "  export PATH=\"${INSTALL_DIR}:\$PATH\""
fi

echo -e "\nRun '${BLUE}omp-cost --help${NC}' to get started!"
