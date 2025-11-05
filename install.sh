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

# Download file function
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
    return 1
  fi
}

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

# Detect Linux distribution
DISTRO=""
if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO=$ID
    if [ -z "$DISTRO" ]; then
        DISTRO=$ID_LIKE
    fi
fi

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo -e "${YELLOW}🐳 Docker not found. Installing Docker...${NC}"
    
    # Install Docker based on distribution
    case $DISTRO in
        almalinux|rocky|centos|rhel|fedora|oracle)
            echo -e "${BLUE}📦 Installing Docker for RHEL-based distribution...${NC}"
            # Install required packages
            if command -v dnf &> /dev/null; then
                sudo dnf install -y dnf-plugins-core
                sudo dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
                sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            elif command -v yum &> /dev/null; then
                sudo yum install -y yum-utils
                sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
                sudo yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            else
                echo -e "${RED}❌ Package manager (dnf/yum) not found${NC}"
                exit 1
            fi
            # Start and enable Docker
            sudo systemctl start docker
            sudo systemctl enable docker
            ;;
        ubuntu|debian)
            echo -e "${BLUE}📦 Installing Docker for Debian-based distribution...${NC}"
            curl -fsSL https://get.docker.com -o get-docker.sh
            sudo sh get-docker.sh
            rm get-docker.sh
            ;;
        *)
            echo -e "${YELLOW}⚠️  Unknown distribution, trying generic Docker installation...${NC}"
            curl -fsSL https://get.docker.com -o get-docker.sh
            sudo sh get-docker.sh
            rm get-docker.sh
            ;;
    esac
    
    # Add user to docker group
    sudo usermod -aG docker $USER
    
    echo -e "${GREEN}✅ Docker installed successfully${NC}"
    echo -e "${YELLOW}⚠️  Note: You may need to log out and back in for group changes to take effect${NC}"
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
sudo mkdir -p /home/portico/{apps,reverse-proxy,templates,www,logs,addons/definitions,addons/instances,repos,bin,.tmp}
sudo chown -R portico:portico /home/portico

# Create Docker network
echo -e "${BLUE}🐳 Creating Docker network...${NC}"
if ! docker network ls | grep -q portico-network; then
    docker network create portico-network
    echo -e "${GREEN}✅ Created portico-network${NC}"
else
    echo -e "${GREEN}✅ portico-network already exists${NC}"
fi

# Configure group permissions for multi-user access
echo -e "${BLUE}👥 Configuring group permissions...${NC}"

# Ask if user wants to be added to portico group (skip for root)
if [[ "$USER" != "root" ]]; then
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
else
    echo -e "${BLUE}ℹ️  Running as root - skipping group addition (root already has full access)${NC}"
fi

# Set group permissions on portico directories
echo -e "${BLUE}🔐 Setting group permissions...${NC}"
sudo chmod -R g+rwX /home/portico
sudo chmod g+s /home/portico/apps  # Set setgid bit so new files inherit group
sudo chmod g+s /home/portico/reverse-proxy  # Set setgid bit so new files inherit group
sudo chmod g+s /home/portico/addons  # Set setgid bit so new files inherit group

# Addon definitions are now embedded in the binary and will be extracted by portico init
echo -e "${GREEN}✅ Addon definitions are embedded in the binary${NC}"

# Setup Git Deployment
echo -e "${BLUE}🔧 Setting up Git Deployment...${NC}"

# Setup SSH for portico user
sudo -u portico mkdir -p /home/portico/.ssh
sudo -u portico chmod 700 /home/portico/.ssh

if [ ! -f /home/portico/.ssh/authorized_keys ]; then
    sudo -u portico touch /home/portico/.ssh/authorized_keys
    sudo -u portico chmod 600 /home/portico/.ssh/authorized_keys
fi

echo -e "${GREEN}✅ Git deployment configured${NC}"


# Verify all required files are available
echo -e "${BLUE}🔍 Verifying all required files are available...${NC}"

# Check for releases (including pre-releases)
LATEST_RELEASE=$(curl -s https://api.github.com/repos/maxvegac/portico/releases/latest | grep "tag_name" | cut -d '"' -f 4)
if [[ -z "$LATEST_RELEASE" ]]; then
    # Try to get any release (including pre-releases)
    LATEST_RELEASE=$(curl -s https://api.github.com/repos/maxvegac/portico/releases | grep "tag_name" | head -1 | cut -d '"' -f 4)
    if [[ -z "$LATEST_RELEASE" ]]; then
        echo -e "${YELLOW}⚠️  No releases found, will build from source${NC}"
        echo -e "${YELLOW}💡 You can check releases at: https://github.com/maxvegac/portico/releases${NC}"
        LATEST_RELEASE="v1.0.0"  # Fallback version
    else
        echo -e "${GREEN}✅ Found release: $LATEST_RELEASE${NC}"
    fi
else
    echo -e "${GREEN}✅ Found latest release: $LATEST_RELEASE${NC}"
fi

# Check binary availability
BINARY_NAME="portico-$OS-$ARCH"
BINARY_URL="https://github.com/maxvegac/portico/releases/download/$LATEST_RELEASE/$BINARY_NAME"
DEV_LATEST_URL="https://github.com/maxvegac/portico/releases/download/dev-latest/portico-dev-latest-$OS-$ARCH"

BINARY_AVAILABLE=false
if [[ "$DEV_MODE" == "true" ]]; then
    # In dev mode, prefer dev-latest
    if check_url "$DEV_LATEST_URL" "Portico dev-latest binary"; then
        BINARY_AVAILABLE=true
    elif check_url "$BINARY_URL" "Portico $LATEST_RELEASE binary"; then
        BINARY_AVAILABLE=true
    fi
else
    # In stable mode, prefer stable release
    if check_url "$BINARY_URL" "Portico $LATEST_RELEASE binary"; then
        BINARY_AVAILABLE=true
    elif check_url "$DEV_LATEST_URL" "Portico dev-latest binary"; then
        BINARY_AVAILABLE=true
    fi
fi

if [[ "$BINARY_AVAILABLE" == "false" ]]; then
    echo -e "${YELLOW}⚠️  No binaries available for download${NC}"
    echo -e "${YELLOW}💡 Will build from source instead${NC}"
    echo -e "${YELLOW}💡 You can check releases at: https://github.com/maxvegac/portico/releases${NC}"
fi



# Download or build Portico CLI binary
echo -e "${BLUE}📦 Setting up Portico CLI...${NC}"

if [[ "$BINARY_AVAILABLE" == "true" ]]; then
    # Download binary
    if [[ "$DEV_MODE" == "true" ]]; then
        # In dev mode, prefer dev-latest
        if check_url "$DEV_LATEST_URL" "Portico dev-latest binary"; then
            echo -e "${BLUE}📦 Downloading Portico dev-latest...${NC}"
            if download_file "$DEV_LATEST_URL" "/tmp/portico" "Portico dev-latest"; then
                sudo mv /tmp/portico /usr/local/bin/portico
                sudo chmod +x /usr/local/bin/portico
            else
                echo -e "${YELLOW}⚠️  Download failed, will build from source${NC}"
                BINARY_AVAILABLE=false
            fi
        elif check_url "$BINARY_URL" "Portico $LATEST_RELEASE binary"; then
            echo -e "${BLUE}📦 Downloading Portico $LATEST_RELEASE...${NC}"
            if download_file "$BINARY_URL" "/tmp/portico" "Portico $LATEST_RELEASE"; then
                sudo mv /tmp/portico /usr/local/bin/portico
                sudo chmod +x /usr/local/bin/portico
            else
                echo -e "${YELLOW}⚠️  Download failed, will build from source${NC}"
                BINARY_AVAILABLE=false
            fi
        fi
    else
        # In stable mode, prefer stable release
        if check_url "$BINARY_URL" "Portico $LATEST_RELEASE binary"; then
            echo -e "${BLUE}📦 Downloading Portico $LATEST_RELEASE...${NC}"
            if download_file "$BINARY_URL" "/tmp/portico" "Portico $LATEST_RELEASE"; then
                sudo mv /tmp/portico /usr/local/bin/portico
                sudo chmod +x /usr/local/bin/portico
            else
                echo -e "${YELLOW}⚠️  Download failed, will build from source${NC}"
                BINARY_AVAILABLE=false
            fi
        elif check_url "$DEV_LATEST_URL" "Portico dev-latest binary"; then
            echo -e "${BLUE}📦 Downloading Portico dev-latest...${NC}"
            if download_file "$DEV_LATEST_URL" "/tmp/portico" "Portico dev-latest"; then
                sudo mv /tmp/portico /usr/local/bin/portico
                sudo chmod +x /usr/local/bin/portico
            else
                echo -e "${YELLOW}⚠️  Download failed, will build from source${NC}"
                BINARY_AVAILABLE=false
            fi
        fi
    fi
fi

if [[ "$BINARY_AVAILABLE" == "false" ]]; then
    # Build from source
    echo -e "${BLUE}🔨 Building Portico from source...${NC}"
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        echo -e "${BLUE}📦 Installing Go...${NC}"
        # Install Go (Ubuntu/Debian)
        if command -v apt-get &> /dev/null; then
            sudo apt-get update
            sudo apt-get install -y golang-go
        # Install Go (CentOS/RHEL)
        elif command -v yum &> /dev/null; then
            sudo yum install -y golang
        # Install Go (macOS)
        elif command -v brew &> /dev/null; then
            brew install go
        else
            echo -e "${RED}❌ Go is not installed and package manager not found${NC}"
            echo -e "${YELLOW}💡 Please install Go manually: https://golang.org/doc/install${NC}"
            exit 1
        fi
    fi
    
    # Clone repository and build
    echo -e "${BLUE}📥 Cloning Portico repository...${NC}"
    cd /tmp
    if [[ -d "portico" ]]; then
        rm -rf portico
    fi
    git clone https://github.com/maxvegac/portico.git
    cd portico
    
    echo -e "${BLUE}🔨 Building Portico...${NC}"
    go build -o portico ./src/cmd/portico
    
    if [[ -f "portico" ]]; then
        sudo mv portico /usr/local/bin/portico
        sudo chmod +x /usr/local/bin/portico
        echo -e "${GREEN}✅ Portico built and installed successfully${NC}"
    else
        echo -e "${RED}❌ Failed to build Portico${NC}"
        exit 1
    fi
fi

# Extract static files from embedded binary
echo -e "${BLUE}📄 Extracting static files from embedded binary...${NC}"

# Use portico init to extract all embedded static files
# Use full path to binary since PATH may not include /usr/local/bin for portico user
if sudo -u portico /usr/local/bin/portico init; then
    echo -e "${GREEN}✅ Static files extracted successfully${NC}"
else
    echo -e "${RED}❌ Failed to extract static files${NC}"
    echo -e "${YELLOW}💡 Make sure portico binary is installed and accessible${NC}"
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
echo "   1. Visit http://YOUR-IP-ADDRESS to see the welcome page"
echo "   2. Create your first app: portico create my-app"
echo "   3. Deploy it: portico up my-app"
echo ""
echo -e "${BLUE}🔧 Useful commands:${NC}"
echo "   portico list              # List all applications"
echo "   portico create <name>     # Create new application"
echo "   portico up <name>         # Start application"
echo "   portico down <name>       # Stop application"
echo "   portico status <name>     # Show application status"
echo ""
echo -e "${BLUE}🚀 Git Deployment:${NC}"
echo "   1. Add your SSH key: portico ssh add ~/.ssh/id_rsa.pub \"my-key\""
echo "   2. Create app (creates git repo automatically): portico create my-app"
echo "   3. Push from your machine: git remote add portico portico@$(hostname):my-app.git"
echo "   4. Deploy: git push portico main"
echo ""
echo -e "${BLUE}📦 Addon commands:${NC}"
echo "   portico addons list                    # List available addons"
echo "   portico addons list postgresql         # List versions for an addon"
echo "   portico addons instances                # List created instances"
echo "   portico addons create <name> --type postgresql --version 16 --shared"
echo "   portico addons database <instance> create <db-name>"
echo "   portico addons link <app> <instance> --database <db-name>"
echo "   portico addons add <app> redis --version 7"
echo ""
echo -e "${BLUE}📖 Check the logs:${NC}"
echo "   docker compose -f /home/portico/reverse-proxy/docker-compose.yml logs -f"
echo ""
