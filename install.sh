#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

GITHUB_REPO="lifinance/lifi-mcp"
BINARY_NAME="lifi-mcp"
INSTALL_DIR="$HOME/.local/bin"

# Check if we're being piped or redirected
is_terminal() {
  [ -t 0 ]
}

# Non-interactive mode check
if ! is_terminal; then
  echo "Running in non-interactive mode. To use interactive mode, run: bash install.sh"
  # In non-interactive mode, we'll proceed with defaults
  INTERACTIVE=false
else
  INTERACTIVE=true
fi

# Detect OS and architecture
detect_platform() {
  PLATFORM="$(uname -s)"
  case "${PLATFORM}" in
    Linux*)     OS="linux";;
    Darwin*)    OS="mac";;
    MINGW*|MSYS*|CYGWIN*)  
                OS="windows"
                echo -e "${YELLOW}Windows installation script is experimental.${NC}"
                echo "For Windows, we recommend downloading the zip file manually."
                INSTALL_DIR="$HOME/bin"
                BINARY_NAME="lifi-mcp.exe"
                ;;
    *)          echo -e "${RED}Unsupported platform: ${PLATFORM}${NC}" && exit 1;;
  esac

  ARCH="$(uname -m)"
  case "${ARCH}" in
    x86_64|amd64)  ARCH="amd64";;
    arm64|aarch64) ARCH="arm64";;
    *)             echo -e "${RED}Unsupported architecture: ${ARCH}${NC}" && exit 1;;
  esac

  echo -e "${GREEN}Detected platform: ${OS}_${ARCH}${NC}"
}

# Check if the installation directory is in PATH
check_path() {
  if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo -e "${YELLOW}Warning: $INSTALL_DIR is not in your PATH.${NC}"
    echo -e "To add it to your PATH, run one of the following commands based on your shell:"
    
    echo -e "\n${BLUE}For bash:${NC}"
    echo "echo 'export PATH=\"\$PATH:$INSTALL_DIR\"' >> ~/.bashrc && source ~/.bashrc"
    
    echo -e "\n${BLUE}For zsh:${NC}"
    echo "echo 'export PATH=\"\$PATH:$INSTALL_DIR\"' >> ~/.zshrc && source ~/.zshrc"
    
    echo -e "\n${BLUE}For fish:${NC}"
    echo "echo 'set -gx PATH \$PATH $INSTALL_DIR' >> ~/.config/fish/config.fish && source ~/.config/fish/config.fish"
    
    echo -e "\n${YELLOW}After installation, you might need to open a new terminal or reload your shell configuration to use the binary.${NC}"
    
    if [ "$INTERACTIVE" = true ]; then
      read -p "Continue with installation? [y/N] " -n 1 -r
      echo
      if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}Installation cancelled.${NC}"
        exit 0
      fi
    else
      echo "Automatically continuing with installation (non-interactive mode)."
    fi
  fi
}

# Get the latest release version
get_latest_release() {
  echo "Fetching latest release..."
  LATEST_RELEASE=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | 
                  grep '"tag_name":' | 
                  sed -E 's/.*"([^"]+)".*/\1/')
  
  if [ -z "$LATEST_RELEASE" ]; then
    echo -e "${RED}Failed to fetch latest release. Please check your internet connection or visit the repository.${NC}"
    exit 1
  fi
  
  echo -e "${GREEN}Latest release: ${LATEST_RELEASE}${NC}"
}

# Download the latest release
download_release() {
  echo "Downloading ${BINARY_NAME} ${LATEST_RELEASE}..."
  
  if [ "$OS" = "windows" ]; then
    ARCHIVE_EXT="zip"
  else
    ARCHIVE_EXT="tar.gz"
  fi
  
  DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${LATEST_RELEASE}/${BINARY_NAME}_${OS}_${ARCH}.${ARCHIVE_EXT}"
  TMP_DIR=$(mktemp -d)
  
  echo "Downloading from: ${DOWNLOAD_URL}"
  
  if ! curl -L -o "${TMP_DIR}/${BINARY_NAME}.${ARCHIVE_EXT}" "${DOWNLOAD_URL}"; then
    echo -e "${RED}Failed to download binary. Please check your internet connection or visit the repository.${NC}"
    rm -rf "${TMP_DIR}"
    exit 1
  fi
  
  echo "Extracting archive..."
  
  if [ "$OS" = "windows" ]; then
    mkdir -p "${TMP_DIR}/extract"
    unzip -q "${TMP_DIR}/${BINARY_NAME}.${ARCHIVE_EXT}" -d "${TMP_DIR}/extract"
  else
    mkdir -p "${TMP_DIR}/extract"
    tar -xzf "${TMP_DIR}/${BINARY_NAME}.${ARCHIVE_EXT}" -C "${TMP_DIR}/extract"
  fi
  
  BINARY_PATH="${TMP_DIR}/extract/${BINARY_NAME}"
}

# Install the binary
install_binary() {
  # First check if already installed
  if [ -f "${INSTALL_DIR}/${BINARY_NAME}" ]; then
    echo -e "${YELLOW}${BINARY_NAME} is already installed at ${INSTALL_DIR}/${BINARY_NAME}${NC}"
    if [ "$INTERACTIVE" = true ]; then
      read -p "Do you want to overwrite it? [y/N] " -n 1 -r
      echo
      if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}Installation cancelled.${NC}"
        rm -rf "${TMP_DIR}"
        exit 0
      fi
    else
      echo "Automatically overwriting existing installation (non-interactive mode)."
    fi
  fi
  
  # Make sure the install directory exists
  if [ ! -d "${INSTALL_DIR}" ]; then
    echo "Creating install directory: ${INSTALL_DIR}"
    mkdir -p "${INSTALL_DIR}" 
  fi

  # Final confirmation before installation
  echo -e "${YELLOW}Ready to install ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}${NC}"
  if [ "$INTERACTIVE" = true ]; then
    read -p "Proceed with installation? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
      echo -e "${YELLOW}Installation cancelled.${NC}"
      rm -rf "${TMP_DIR}"
      exit 0
    fi
  else
    echo "Automatically proceeding with installation (non-interactive mode)."
  fi
  
  # Move the binary to the install directory
  echo "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."
  if ! mv "${BINARY_PATH}" "${INSTALL_DIR}/${BINARY_NAME}" 2>/dev/null; then
    echo -e "${RED}Failed to move binary to ${INSTALL_DIR}.${NC}"
    echo -e "${YELLOW}Trying with sudo...${NC}"
    sudo mv "${BINARY_PATH}" "${INSTALL_DIR}/${BINARY_NAME}" || { 
      echo -e "${RED}Failed to install binary. Please try running the script with sudo.${NC}"
      rm -rf "${TMP_DIR}"
      exit 1 
    }
  fi
  
  # Make the binary executable
  chmod +x "${INSTALL_DIR}/${BINARY_NAME}" || { 
    echo -e "${YELLOW}Trying with sudo...${NC}"
    sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}" || {
      echo -e "${RED}Failed to make binary executable. Please try running the script with sudo.${NC}"
      rm -rf "${TMP_DIR}"
      exit 1
    }
  }
  
  echo -e "${GREEN}Installation completed successfully!${NC}"
  echo -e "${GREEN}Run '${BINARY_NAME}' to get started.${NC}"
  
  # Clean up
  rm -rf "${TMP_DIR}"
}

# Main function
main() {
  echo "LI.FI MCP Installer"
  echo "==================="
  
  detect_platform
  check_path
  get_latest_release
  download_release
  install_binary
}

main
