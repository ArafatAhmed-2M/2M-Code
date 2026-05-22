#!/usr/bin/env bash
# 2M Code Installer
# Installs the Go binary and Python agent engine dependencies.
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/2mcode/2mcode/main/scripts/install.sh | bash
#
# Requirements:
#   - Go 1.22+ (for building from source)
#   - Python 3.11+ (for the agent engine)
#   - git

set -e

# Configuration
REPO="https://github.com/2mcode/2mcode"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="$HOME/.2mcode"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

info() {
    echo -e "${CYAN}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Check for required tools
check_requirements() {
    info "Checking requirements..."

    # Check for Go
    if ! command -v go &> /dev/null; then
        error "Go is not installed. Install Go 1.22+ from https://go.dev/dl/"
    fi

    GO_VERSION=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+')
    info "Found Go $GO_VERSION"

    # Check for Python
    PYTHON=""
    if command -v python3 &> /dev/null; then
        PYTHON="python3"
    elif command -v python &> /dev/null; then
        PYTHON="python"
    else
        error "Python 3 is not installed. Install Python 3.11+ from https://python.org"
    fi

    PY_VERSION=$($PYTHON --version 2>&1 | grep -oP '[0-9]+\.[0-9]+')
    info "Found Python $PY_VERSION"

    # Check for git
    if ! command -v git &> /dev/null; then
        error "git is not installed. Install git from https://git-scm.com"
    fi

    success "All requirements met"
}

# Clone and build
install_from_source() {
    info "Cloning 2M Code repository..."

    TEMP_DIR=$(mktemp -d)
    git clone --depth 1 "$REPO" "$TEMP_DIR/2mcode"
    cd "$TEMP_DIR/2mcode"

    info "Installing Python dependencies..."
    $PYTHON -m pip install -r requirements.txt --quiet

    info "Building Go binary..."
    go build -o bin/2m ./cmd/2m

    info "Installing binary to $INSTALL_DIR..."
    if [ -w "$INSTALL_DIR" ]; then
        cp bin/2m "$INSTALL_DIR/2m"
    else
        sudo cp bin/2m "$INSTALL_DIR/2m"
    fi

    # Copy agent engine to config directory
    info "Installing agent engine..."
    mkdir -p "$CONFIG_DIR"
    cp -r agent_engine "$CONFIG_DIR/agent_engine"
    cp -r config "$CONFIG_DIR/config"

    # Clean up
    rm -rf "$TEMP_DIR"

    success "Binary installed to $INSTALL_DIR/2m"
}

# Create initial config
setup_config() {
    info "Setting up configuration..."

    mkdir -p "$CONFIG_DIR/teams"
    mkdir -p "$CONFIG_DIR/sessions"

    # Create config template if it doesn't exist
    if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
        cat > "$CONFIG_DIR/config.yaml" << 'EOF'
# 2M Code Configuration
# See: https://github.com/2mcode/2mcode

# Default team to use when none is specified
# default_team: fullstack

# Default provider for new teams
# default_provider: anthropic

# Enable verbose output
verbose: false
EOF
        chmod 600 "$CONFIG_DIR/config.yaml"
        success "Created config at $CONFIG_DIR/config.yaml"
    else
        info "Config already exists at $CONFIG_DIR/config.yaml"
    fi
}

# Print next steps
print_next_steps() {
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║       2M Code installed successfully! 🎉      ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════╝${NC}"
    echo ""
    echo "Next steps:"
    echo ""
    echo "  1. Set up at least one API key:"
    echo -e "     ${CYAN}export ANTHROPIC_API_KEY='your-key'${NC}"
    echo -e "     ${CYAN}export GOOGLE_API_KEY='your-key'${NC}"
    echo -e "     ${CYAN}export OPENAI_API_KEY='your-key'${NC}"
    echo ""
    echo "  2. Create your first team:"
    echo -e "     ${CYAN}2m new-team${NC}"
    echo ""
    echo "  3. Or try a bundled team:"
    echo -e "     ${CYAN}2m run fullstack \"Build a hello world REST API in Go\"${NC}"
    echo ""
    echo "  4. Start chatting:"
    echo -e "     ${CYAN}2m chat fullstack${NC}"
    echo ""
    echo "Documentation: https://github.com/2mcode/2mcode"
    echo ""
}

# Main
main() {
    echo ""
    echo -e "${CYAN}  ___  __  __    ____          _      ${NC}"
    echo -e "${CYAN} |__ \\|  \\/  |  / ___|___   __| | ___ ${NC}"
    echo -e "${CYAN}   ) | |\\/| | | |   / _ \\ / _\` |/ _ \\${NC}"
    echo -e "${CYAN}  / /| |  | | | |__| (_) | (_| |  __/${NC}"
    echo -e "${CYAN} |___|_|  |_|  \\____\\___/ \\__,_|\\___|${NC}"
    echo ""
    echo "  Installing 2M Code — The AI coding platform that thinks in teams"
    echo ""

    check_requirements
    install_from_source
    setup_config
    print_next_steps
}

main "$@"
