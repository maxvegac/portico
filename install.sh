#!/bin/bash

# Portico Installation Script
# Usage: 
#   curl -fsSL https://raw.githubusercontent.com/maxvegac/portico/main/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/maxvegac/portico/main/install.sh | bash -s -- --dev

set -e

# Parse arguments
DEV_MODE=false
if [[ "$1" == "--dev" ]]; then
    DEV_MODE=true
    echo "🚀 Installing Portico PaaS (Development Mode)..."
else
    echo "🚀 Installing Portico PaaS..."
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture names
case $ARCH in
    x86_64) ARCH="amd64" ;;
    arm64) ARCH="arm64" ;;
    aarch64) ARCH="arm64" ;;
    *) echo -e "${RED}❌ Unsupported architecture: $ARCH${NC}"; exit 1 ;;
esac

echo -e "${BLUE}📋 Detected: $OS $ARCH${NC}"

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo -e "${YELLOW}🐳 Docker not found. Installing Docker...${NC}"
    curl -fsSL https://get.docker.com -o get-docker.sh
    sudo sh get-docker.sh
    sudo usermod -aG docker $USER
    rm get-docker.sh
else
    echo -e "${GREEN}✅ Docker is already installed${NC}"
fi

# Check if docker compose is available
if ! docker compose version &>/dev/null && ! command -v docker-compose &>/dev/null; then
    echo -e "${YELLOW}🐳 docker compose not found. Installing...${NC}"
    sudo mkdir -p /usr/local/lib/docker/cli-plugins
    sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" \
        -o /usr/local/lib/docker/cli-plugins/docker-compose
    sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
else
    echo -e "${GREEN}✅ docker compose is available${NC}"
fi

# Create portico user if it doesn't exist
if ! id "portico" &>/dev/null; then
    echo -e "${BLUE}👤 Creating portico user...${NC}"
    sudo useradd -m -s /bin/bash portico
    sudo usermod -aG docker portico
fi

# Create directories
echo -e "${BLUE}📁 Creating directories...${NC}"
sudo mkdir -p /home/portico/{apps,reverse-proxy,static,logs}
sudo chown -R portico:portico /home/portico

# Configure group permissions for multi-user access
echo -e "${BLUE}👥 Configuring group permissions...${NC}"

# Ask if user wants to be added to portico group
if ! groups $USER | grep -q '\bportico\b'; then
    echo -e "${YELLOW}❓ Do you want to add user '$USER' to the 'portico' group?${NC}"
    echo -e "${YELLOW}   This will allow you to access Portico files without sudo.${NC}"
    echo -e "${YELLOW}   (y/N): ${NC}"
    read -r response
    if [[ "$response" =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}➕ Adding $USER to portico group...${NC}"
        sudo usermod -aG portico $USER
        echo -e "${GREEN}✅ User $USER has been added to the portico group${NC}"
        echo -e "${YELLOW}⚠️  Note: You may need to log out and log back in for group changes to take effect${NC}"
    else
        echo -e "${BLUE}ℹ️  Skipping group addition. You can add yourself later with:${NC}"
        echo -e "${BLUE}   sudo usermod -aG portico $USER${NC}"
    fi
else
    echo -e "${GREEN}✅ User $USER is already in the portico group${NC}"
fi

# Set group permissions on portico directories
echo -e "${BLUE}🔐 Setting group permissions...${NC}"
sudo chmod -R g+rwX /home/portico
sudo chmod g+s /home/portico/apps  # Set setgid bit so new files inherit group
sudo chmod g+s /home/portico/reverse-proxy  # Set setgid bit so new files inherit group

# Safer URL check: follow redirects, tolerate HEAD-not-allowed, and read final code.
check_url() {
  local url="$1"
  local name="$2"

  # Try HEAD following redirects; if HEAD unsupported, do a minimal GET (range 0-0)
  local code
  code=$(curl -sS -I -L -o /dev/null -w '%{http_code}' --connect-timeout 10 --max-time 30 "$url") || code=0
  if [ "$code" -eq 405 ] || [ "$code" -eq 0 ]; then
    code=$(curl -sS -L -o /dev/null -r 0-0 -w '%{http_code}' --connect-timeout 10 --max-time 30 "$url") || code=0
  fi

  # For availability checks, require a final 200 on GET/ranged GET
  if [ "$code" -eq 200 ]; then
    echo -e "${GREEN}✅ $name is available${NC}"
    return 0
  else
    echo -e "${RED}❌ $name is not available at $url (HTTP ${code})${NC}"
    return 1
  fi
}

download_file() {
  local url="$1"
  local output="$2"
  local name="$3"

  # Follow redirects and fail on 4xx/5xx
  if curl -sS -L --fail-with-body --connect-timeout 15 --max-time 0 -o "$output" "$url"; then
    echo -e "${GREEN}✅ Downloaded $name${NC}"
    return 0
  else
    local ec=$?
    echo -e "${RED}❌ Failed to download $name (curl exit $ec)${NC}"
    # For debugging, uncomment:
    # curl -I -L "$url"
    return 1
  fi
}

# Verify all required files are available
echo -e "${BLUE}🔍 Verifying all required files are available...${NC}"

# Check for releases
LATEST_RELEASE=$(curl -s https://api.github.com/repos/maxvegac/portico/releases/latest | grep "tag_name" | cut -d '"' -f 4)
if [[ -z "$LATEST_RELEASE" ]]; then
    echo -e "${RED}❌ No releases found${NC}"
    echo -e "${YELLOW}💡 Please check: https://github.com/maxvegac/portico/releases${NC}"
    exit 1
fi

# Check binary availability
BINARY_NAME="portico-$OS-$ARCH"
BINARY_URL="https://github.com/maxvegac/portico/releases/download/$LATEST_RELEASE/$BINARY_NAME"
DEV_LATEST_URL="https://github.com/maxvegac/portico/releases/download/dev-latest/portico-dev-latest-$OS-$ARCH"

if [[ "$DEV_MODE" == "true" ]]; then
    # In dev mode, prefer dev-latest
    if ! check_url "$DEV_LATEST_URL" "Portico dev-latest binary"; then
        if ! check_url "$BINARY_URL" "Portico $LATEST_RELEASE binary"; then
            echo -e "${RED}❌ No binaries available for download${NC}"
            echo -e "${YELLOW}💡 Please check: https://github.com/maxvegac/portico/releases${NC}"
            exit 1
        fi
    fi
else
    # In stable mode, prefer stable release
    if ! check_url "$BINARY_URL" "Portico $LATEST_RELEASE binary"; then
        if ! check_url "$DEV_LATEST_URL" "Portico dev-latest binary"; then
            echo -e "${RED}❌ No binaries available for download${NC}"
            echo -e "${YELLOW}💡 Please check: https://github.com/maxvegac/portico/releases${NC}"
            exit 1
        fi
    fi
fi

# Check static files availability
STATIC_FILES=(
  "https://raw.githubusercontent.com/maxvegac/portico/main/static/index.html"   "Welcome page"
  "https://raw.githubusercontent.com/maxvegac/portico/main/static/Caddyfile"    "Caddyfile"
  "https://raw.githubusercontent.com/maxvegac/portico/main/static/config.yml"   "Configuration"
  "https://raw.githubusercontent.com/maxvegac/portico/main/static/docker-compose.yml" "Docker Compose"
)

for ((i=0; i<${#STATIC_FILES[@]}; i+=2)); do
  url=${STATIC_FILES[i]}
  name=${STATIC_FILES[i+1]}
  if ! check_url "$url" "$name"; then
    echo -e "${RED}❌ Required file $name is not available${NC}"
    echo -e "${YELLOW}💡 Please check: https://github.com/maxvegac/portico${NC}"
    exit 1
  fi
done

echo -e "${GREEN}✅ All required files are available${NC}"

# Download Portico CLI binary
echo -e "${BLUE}📦 Downloading Portico CLI...${NC}"

if [[ "$DEV_MODE" == "true" ]]; then
    # In dev mode, prefer dev-latest
    if check_url "$DEV_LATEST_URL" "Portico dev-latest binary"; then
        echo -e "${BLUE}📦 Downloading Portico dev-latest...${NC}"
        if download_file "$DEV_LATEST_URL" "/tmp/portico" "Portico dev-latest"; then
            sudo mv /tmp/portico /usr/local/bin/portico
            sudo chmod +x /usr/local/bin/portico
        else
            exit 1
        fi
    elif check_url "$BINARY_URL" "Portico $LATEST_RELEASE binary"; then
        echo -e "${BLUE}📦 Downloading Portico $LATEST_RELEASE...${NC}"
        if download_file "$BINARY_URL" "/tmp/portico" "Portico $LATEST_RELEASE"; then
            sudo mv /tmp/portico /usr/local/bin/portico
            sudo chmod +x /usr/local/bin/portico
        else
            exit 1
        fi
    else
        echo -e "${RED}❌ No binaries available for download${NC}"
        exit 1
    fi
else
    # In stable mode, prefer stable release
    if check_url "$BINARY_URL" "Portico $LATEST_RELEASE binary"; then
        echo -e "${BLUE}📦 Downloading Portico $LATEST_RELEASE...${NC}"
        if download_file "$BINARY_URL" "/tmp/portico" "Portico $LATEST_RELEASE"; then
            sudo mv /tmp/portico /usr/local/bin/portico
            sudo chmod +x /usr/local/bin/portico
        else
            exit 1
        fi
    elif check_url "$DEV_LATEST_URL" "Portico dev-latest binary"; then
        echo -e "${BLUE}📦 Downloading Portico dev-latest...${NC}"
        if download_file "$DEV_LATEST_URL" "/tmp/portico" "Portico dev-latest"; then
            sudo mv /tmp/portico /usr/local/bin/portico
            sudo chmod +x /usr/local/bin/portico
        else
            exit 1
        fi
    else
        echo -e "${RED}❌ No binaries available for download${NC}"
        exit 1
    fi
fi

# Create welcome page
echo -e "${BLUE}📄 Setting up welcome page...${NC}"

# Download the welcome page from the repository
WELCOME_URL="https://raw.githubusercontent.com/maxvegac/portico/main/static/index.html"
if download_file "$WELCOME_URL" "/tmp/index.html" "Welcome page"; then
    sudo mkdir -p /home/portico/static
    sudo mv /tmp/index.html /home/portico/static/index.html
    sudo chown portico:portico /home/portico/static/index.html
else
    exit 1
fi

# Create initial Caddyfile
echo -e "${BLUE}⚙️  Setting up Caddyfile...${NC}"

# Download the Caddyfile from the repository
CADDYFILE_URL="https://raw.githubusercontent.com/maxvegac/portico/main/static/Caddyfile"
if download_file "$CADDYFILE_URL" "/tmp/Caddyfile" "Caddyfile"; then
    sudo mkdir -p /home/portico/reverse-proxy
    sudo mv /tmp/Caddyfile /home/portico/reverse-proxy/Caddyfile
    sudo chown portico:portico /home/portico/reverse-proxy/Caddyfile
else
    exit 1
fi

# Create portico config
echo -e "${BLUE}📋 Setting up Portico configuration...${NC}"

# Download the config from the repository
CONFIG_URL="https://raw.githubusercontent.com/maxvegac/portico/main/static/config.yml"
if download_file "$CONFIG_URL" "/tmp/config.yml" "Configuration"; then
    sudo mv /tmp/config.yml /home/portico/config.yml
    sudo chown portico:portico /home/portico/config.yml
else
    exit 1
fi

# Create reverse-proxy docker compose
echo -e "${BLUE}🚀 Setting up reverse-proxy with Docker...${NC}"

# Download the docker compose from the repository
COMPOSE_URL="https://raw.githubusercontent.com/maxvegac/portico/main/static/docker-compose.yml"
if download_file "$COMPOSE_URL" "/tmp/docker-compose.yml" "Docker Compose configuration"; then
    sudo mv /tmp/docker-compose.yml /home/portico/reverse-proxy/docker-compose.yml
    sudo chown portico:portico /home/portico/reverse-proxy/docker-compose.yml
else
    exit 1
fi

# Start the reverse-proxy
sudo -u portico bash -c 'cd /home/portico/reverse-proxy && docker compose up -d'

echo ""
echo -e "${GREEN}✅ Portico installation completed!${NC}"
echo ""

# Show group configuration info
echo -e "${BLUE}📋 Group Configuration:${NC}"
if groups $USER | grep -q '\bportico\b'; then
    echo -e "  • User '$USER' is in the 'portico' group"
    echo -e "  • You can access /home/portico directories without sudo"
else
    echo -e "  • User '$USER' is not in the 'portico' group"
    echo -e "  • You may need to use sudo for some Portico commands"
    echo -e "  • To add yourself later: sudo usermod -aG portico $USER"
fi
echo -e "  • You may need to log out and log back in for group changes to take effect"
echo ""

if [[ "$DEV_MODE" == "true" ]]; then
    echo -e "${GREEN}🎉 Congrats! Portico Dev is running${NC}"
    echo -e "${YELLOW}⚠️  Note: This is a development build with latest features${NC}"
else
    echo -e "${GREEN}🎉 Congrats! Portico is running${NC}"
fi

echo ""
echo -e "${BLUE}📋 Next steps:${NC}"
echo "   1. Visit http://localhost to see the welcome page"
echo "   2. Create your first app: portico apps create my-app"
echo "   3. Deploy it: portico apps deploy my-app"
echo ""
echo -e "${BLUE}🔧 Useful commands:${NC}"
echo "   portico apps list          # List all applications"
echo "   portico apps create <name>  # Create new application"
echo "   portico apps deploy <name> # Deploy application"
echo "   portico apps destroy <name> # Destroy application"
echo ""
echo -e "${BLUE}📖 Check the logs:${NC}"
echo "   docker compose -f /home/portico/reverse-proxy/docker-compose.yml logs -f"
echo ""
