#!/bin/sh
set -e

# OpenWRT Configurator installation script
# Usage: curl -sfL https://raw.githubusercontent.com/drummonds/openwrt-configurator/main/install.sh | sh

REPO="drummonds/openwrt-configurator"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS and architecture
detect_platform() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "$OS" in
        Linux*)
            OS="Linux"
            ;;
        Darwin*)
            OS="Darwin"
            ;;
        *)
            echo "Unsupported operating system: $OS"
            exit 1
            ;;
    esac

    case "$ARCH" in
        x86_64|amd64)
            ARCH="x86_64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        armv7l|armv6l)
            ARCH="arm"
            ;;
        *)
            echo "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    echo "Detected platform: $OS $ARCH"
}

# Get latest release version
get_latest_version() {
    VERSION=$(curl -sfL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/')

    if [ -z "$VERSION" ]; then
        echo "Failed to get latest version"
        exit 1
    fi

    echo "Latest version: v$VERSION"
}

# Download and install
install_binary() {
    FILENAME="openwrt-configurator_${VERSION}_${OS}_${ARCH}.tar.gz"
    URL="https://github.com/$REPO/releases/download/v${VERSION}/${FILENAME}"

    echo "Downloading from: $URL"

    # Create temporary directory
    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"

    # Download
    if ! curl -sfL "$URL" -o "$FILENAME"; then
        echo "Failed to download $URL"
        rm -rf "$TMP_DIR"
        exit 1
    fi

    # Extract
    tar -xzf "$FILENAME"

    # Install
    echo "Installing to $INSTALL_DIR/openwrt-configurator"

    if [ -w "$INSTALL_DIR" ]; then
        mv openwrt-configurator "$INSTALL_DIR/"
        chmod +x "$INSTALL_DIR/openwrt-configurator"
    else
        echo "Root privileges required for installation to $INSTALL_DIR"
        sudo mv openwrt-configurator "$INSTALL_DIR/"
        sudo chmod +x "$INSTALL_DIR/openwrt-configurator"
    fi

    # Cleanup
    cd -
    rm -rf "$TMP_DIR"

    echo "Successfully installed openwrt-configurator v$VERSION"
    echo "Run 'openwrt-configurator --version' to verify installation"
}

# Main
main() {
    detect_platform
    get_latest_version
    install_binary
}

main
