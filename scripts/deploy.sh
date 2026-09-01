#!/usr/bin/env bash
set -e

# ==============================================================================
# Lodestone Automated Remote Deployment Script
# ==============================================================================

BOLD='\033[1m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG_FILE="${ROOT_DIR}/.deploy.env"

# Load saved configuration if present
if [ -f "$CONFIG_FILE" ]; then
    # shellcheck disable=SC1090
    source "$CONFIG_FILE"
fi

DEPLOY_HOST="${DEPLOY_HOST:-}"
DEPLOY_DIR="${DEPLOY_DIR:-}"
DEPLOY_ARCH="${DEPLOY_ARCH:-arm64}"
DEPLOY_SERVICE="${DEPLOY_SERVICE:-lodestone}"
DEPLOY_PORT="${DEPLOY_PORT:-22}"
DEPLOY_KEY="${DEPLOY_KEY:-}"
SKIP_BUILD=false
SKIP_FRONTEND=false
RESTART_SERVICE=true
SAVE_CONFIG=false
DRY_RUN=false

usage() {
    echo -e "${BOLD}Usage:${NC} $0 [OPTIONS]"
    echo ""
    echo -e "${BOLD}Deployment Options:${NC}"
    echo "  -h, --host <user@ip>      Remote SSH target (e.g. ubuntu@192.168.1.50 or mc.server.com)"
    echo "  -d, --dir <remote_path>   Remote destination directory (e.g. /opt/lodestone or ~/lodestone)"
    echo "  -a, --arch <arch>         Target architecture: arm64 [default], amd64, arm"
    echo "  -s, --service <name>      Systemd service name to restart (default: lodestone, use 'none' to skip)"
    echo "  -p, --port <port>         SSH port (default: 22)"
    echo "  -i, --identity <key_path> Path to private SSH key"
    echo "      --skip-build          Skip binary compilation (use existing binary in bin/)"
    echo "      --skip-frontend       Skip rebuilding React UI bundle during build"
    echo "      --no-restart          Do not restart systemd service on remote server"
    echo "      --save                Save deployment configuration to .deploy.env for future runs"
    echo "      --dry-run             Show what would be executed without transferring files"
    echo "      --help                Show this help message"
    echo ""
    echo -e "${BOLD}Examples:${NC}"
    echo "  $0 --host ubuntu@192.168.1.100 --dir /opt/lodestone --arch arm64"
    echo "  $0 -h root@mc-node1.net -d /home/minecraft/lodestone -a amd64 --service lodestone"
    echo "  $0                      (Interactive mode using .deploy.env if saved)"
    exit 0
}

# Parse Command Line Arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--host)
            DEPLOY_HOST="$2"
            shift 2
            ;;
        -d|--dir)
            DEPLOY_DIR="$2"
            shift 2
            ;;
        -a|--arch)
            DEPLOY_ARCH="$2"
            shift 2
            ;;
        -s|--service)
            DEPLOY_SERVICE="$2"
            shift 2
            ;;
        -p|--port)
            DEPLOY_PORT="$2"
            shift 2
            ;;
        -i|--identity)
            DEPLOY_KEY="$2"
            shift 2
            ;;
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        --skip-frontend)
            SKIP_FRONTEND=true
            shift
            ;;
        --no-restart)
            RESTART_SERVICE=false
            shift
            ;;
        --save)
            SAVE_CONFIG=true
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --help)
            usage
            ;;
        *)
            echo -e "${RED}Unknown argument: $1${NC}"
            usage
            ;;
    esac
done

INTERACTIVE=false
if [ -z "$DEPLOY_HOST" ]; then
    INTERACTIVE=true
    echo -e "${CYAN}${BOLD}🧭 Lodestone Deployment Setup${NC}"
    echo ""
    read -rp "Enter remote SSH target (e.g. user@192.168.1.50): " DEPLOY_HOST
    if [ -z "$DEPLOY_HOST" ]; then
        echo -e "${RED}Error: Host cannot be empty.${NC}"
        exit 1
    fi
fi

if [ -z "$DEPLOY_DIR" ]; then
    read -rp "Enter remote install directory [default: /opt/lodestone]: " input_dir
    DEPLOY_DIR="${input_dir:-/opt/lodestone}"
fi

# Ask to save config if in interactive mode and not already saved
if [ "$INTERACTIVE" = true ] && [ ! -f "$CONFIG_FILE" ] && [ "$SAVE_CONFIG" = false ]; then
    read -rp "Save configuration to .deploy.env for fast future deploys? (y/N): " save_prompt
    if [[ "$save_prompt" =~ ^[Yy]$ ]]; then
        SAVE_CONFIG=true
    fi
fi

if [ "$SAVE_CONFIG" = true ]; then
    cat <<EOF > "$CONFIG_FILE"
# Lodestone Deployment Configuration
DEPLOY_HOST="${DEPLOY_HOST}"
DEPLOY_DIR="${DEPLOY_DIR}"
DEPLOY_ARCH="${DEPLOY_ARCH}"
DEPLOY_SERVICE="${DEPLOY_SERVICE}"
DEPLOY_PORT="${DEPLOY_PORT}"
DEPLOY_KEY="${DEPLOY_KEY}"
EOF
    echo -e "${GREEN}✓ Saved configuration to ${CONFIG_FILE}${NC}"
fi

BINARY_NAME="lodestone-linux-${DEPLOY_ARCH}"
LOCAL_BINARY="${ROOT_DIR}/bin/${BINARY_NAME}"

# Construct SSH & SCP/Rsync command flags
SSH_OPTS=("-p" "${DEPLOY_PORT}")
SCP_OPTS=("-P" "${DEPLOY_PORT}")
if [ -n "$DEPLOY_KEY" ]; then
    SSH_OPTS+=("-i" "${DEPLOY_KEY}")
    SCP_OPTS+=("-i" "${DEPLOY_KEY}")
fi

echo ""
echo -e "${BOLD}📦 Deployment Target Summary:${NC}"
echo -e "  Host:         ${CYAN}${DEPLOY_HOST}${NC}"
echo -e "  Directory:    ${CYAN}${DEPLOY_DIR}${NC}"
echo -e "  Architecture: ${CYAN}linux/${DEPLOY_ARCH}${NC}"
echo -e "  Service:      ${CYAN}${DEPLOY_SERVICE}${NC}"
echo -e "  SSH Port:     ${CYAN}${DEPLOY_PORT}${NC}"
echo ""

# 1. Build Phase
if [ "$SKIP_BUILD" = false ]; then
    echo -e "${CYAN}🔨 Step 1: Building ${BINARY_NAME}...${NC}"
    BUILD_ARGS=("$DEPLOY_ARCH")
    if [ "$SKIP_FRONTEND" = true ]; then
        BUILD_ARGS+=("--skip-frontend")
    fi
    "${ROOT_DIR}/scripts/build.sh" "${BUILD_ARGS[@]}"
else
    echo -e "${YELLOW}⚡ Skipping build step (--skip-build). Using existing ${LOCAL_BINARY}${NC}"
    if [ ! -f "$LOCAL_BINARY" ]; then
        echo -e "${RED}Error: Binary ${LOCAL_BINARY} not found! Please build it first or remove --skip-build.${NC}"
        exit 1
    fi
fi

if [ "$DRY_RUN" = true ]; then
    echo -e "${YELLOW}[DRY-RUN] Would test SSH connectivity to ${DEPLOY_HOST}${NC}"
    echo -e "${YELLOW}[DRY-RUN] Would upload ${LOCAL_BINARY} to ${DEPLOY_HOST}:${DEPLOY_DIR}/lodestone${NC}"
    if [ "$RESTART_SERVICE" = true ] && [ "$DEPLOY_SERVICE" != "none" ]; then
        echo -e "${YELLOW}[DRY-RUN] Would restart remote service ${DEPLOY_SERVICE}${NC}"
    fi
    echo -e "${GREEN}Dry-run completed successfully.${NC}"
    exit 0
fi

# 2. Test SSH Connection
echo ""
echo -e "${CYAN}🔌 Step 2: Testing SSH connection to ${DEPLOY_HOST}...${NC}"
if ! ssh -o ConnectTimeout=8 "${SSH_OPTS[@]}" "${DEPLOY_HOST}" "echo 'Connection established'" >/dev/null 2>&1; then
    echo -e "${RED}❌ Failed to connect to ${DEPLOY_HOST} over SSH on port ${DEPLOY_PORT}.${NC}"
    echo -e "${YELLOW}Please check your network, SSH credentials, or SSH key path.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Connection verified!${NC}"

# 3. Create Remote Directory
echo ""
echo -e "${CYAN}📁 Step 3: Preparing remote directory ${DEPLOY_DIR}...${NC}"
ssh "${SSH_OPTS[@]}" "${DEPLOY_HOST}" "mkdir -p '${DEPLOY_DIR}'"
echo -e "${GREEN}✓ Remote directory ready.${NC}"

# 4. Upload Binary Atomically
echo ""
echo -e "${CYAN}🚀 Step 4: Transferring binary to remote host...${NC}"
if command -v rsync >/dev/null 2>&1; then
    RSYNC_SSH="ssh -p ${DEPLOY_PORT}"
    if [ -n "$DEPLOY_KEY" ]; then
        RSYNC_SSH="ssh -p ${DEPLOY_PORT} -i ${DEPLOY_KEY}"
    fi
    rsync -avz --progress -e "${RSYNC_SSH}" "${LOCAL_BINARY}" "${DEPLOY_HOST}:${DEPLOY_DIR}/lodestone.new"
else
    scp "${SCP_OPTS[@]}" "${LOCAL_BINARY}" "${DEPLOY_HOST}:${DEPLOY_DIR}/lodestone.new"
fi

# Set executable permissions and atomic swap
ssh "${SSH_OPTS[@]}" "${DEPLOY_HOST}" "chmod +x '${DEPLOY_DIR}/lodestone.new' && mv '${DEPLOY_DIR}/lodestone.new' '${DEPLOY_DIR}/lodestone'"
echo -e "${GREEN}✓ Binary deployed to ${DEPLOY_DIR}/lodestone${NC}"

# 5. Restart Remote Service (if requested)
if [ "$RESTART_SERVICE" = true ] && [ "$DEPLOY_SERVICE" != "none" ]; then
    echo ""
    echo -e "${CYAN}🔄 Step 5: Restarting remote systemd service '${DEPLOY_SERVICE}'...${NC}"
    if ssh "${SSH_OPTS[@]}" "${DEPLOY_HOST}" "sudo systemctl restart '${DEPLOY_SERVICE}' 2>/dev/null || systemctl --user restart '${DEPLOY_SERVICE}' 2>/dev/null"; then
        echo -e "${GREEN}✓ Service '${DEPLOY_SERVICE}' restarted successfully!${NC}"
    else
        echo -e "${YELLOW}⚠️ Could not restart systemd service '${DEPLOY_SERVICE}' automatically.${NC}"
        echo -e "${YELLOW}You can start/restart it manually with: ssh ${DEPLOY_HOST} 'sudo systemctl restart ${DEPLOY_SERVICE}'${NC}"
    fi
fi

echo ""
echo -e "${GREEN}${BOLD}🎉 Deployment finished successfully!${NC}"
echo -e "To inspect live remote logs, run:"
echo -e "  ${BOLD}ssh ${DEPLOY_HOST} 'journalctl -u ${DEPLOY_SERVICE} -f -n 50'${NC}"
