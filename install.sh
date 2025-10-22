#!/bin/bash

# Portico Installation Script
# Usage: curl -fsSL https://raw.githubusercontent.com/portico/portico/main/install.sh | bash

set -e

echo "🚀 Installing Portico PaaS..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if running as root
if [[ $EUID -eq 0 ]]; then
   echo -e "${RED}❌ Please do not run this script as root${NC}"
   exit 1
fi

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

# Check if docker-compose is available
if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
    echo -e "${YELLOW}🐳 docker-compose not found. Installing...${NC}"
    sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
    sudo chmod +x /usr/local/bin/docker-compose
else
    echo -e "${GREEN}✅ docker-compose is available${NC}"
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

# Download Portico CLI binary
echo -e "${BLUE}📦 Downloading Portico CLI...${NC}"

# Get latest release
LATEST_RELEASE=$(curl -s https://api.github.com/repos/portico/portico/releases/latest | grep "tag_name" | cut -d '"' -f 4)
if [[ -z "$LATEST_RELEASE" ]]; then
    echo -e "${YELLOW}⚠️  No releases found, using development build...${NC}"
    # Fallback to building from source
    if ! command -v go &> /dev/null; then
        echo -e "${BLUE}🔨 Installing Go...${NC}"
        curl -fsSL https://go.dev/dl/go1.21.0.linux-amd64.tar.gz -o go.tar.gz
        sudo tar -C /usr/local -xzf go.tar.gz
        rm go.tar.gz
        export PATH=$PATH:/usr/local/go/bin
    fi
    
    cd /tmp
    git clone https://github.com/portico/portico.git
    cd portico
    go build -o portico ./cmd/portico
    sudo cp portico /usr/local/bin/
    sudo chmod +x /usr/local/bin/portico
    cd /
    rm -rf /tmp/portico
else
    echo -e "${BLUE}📦 Downloading Portico $LATEST_RELEASE...${NC}"
    
    # Download the appropriate binary
    BINARY_NAME="portico-$OS-$ARCH"
    if [[ "$OS" == "windows" ]]; then
        BINARY_NAME="${BINARY_NAME}.exe"
    fi
    
    # Download binary
    DOWNLOAD_URL="https://github.com/portico/portico/releases/download/$LATEST_RELEASE/$BINARY_NAME"
    echo -e "${BLUE}📥 Downloading from: $DOWNLOAD_URL${NC}"
    
    curl -L "$DOWNLOAD_URL" -o /tmp/portico
    sudo mv /tmp/portico /usr/local/bin/portico
    sudo chmod +x /usr/local/bin/portico
fi

# Create welcome page
echo -e "${BLUE}📄 Creating welcome page...${NC}"
sudo tee /home/portico/static/index.html > /dev/null << 'EOF'
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Portico - PaaS Platform</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh; display: flex; align-items: center; justify-content: center; color: white;
        }
        .container { text-align: center; max-width: 600px; padding: 2rem; }
        .logo { font-size: 4rem; margin-bottom: 1rem; font-weight: 300; }
        .title { font-size: 2.5rem; margin-bottom: 1rem; font-weight: 600; }
        .subtitle { font-size: 1.2rem; margin-bottom: 2rem; opacity: 0.9; }
        .status { background: rgba(255, 255, 255, 0.1); border: 1px solid rgba(255, 255, 255, 0.2);
                  border-radius: 12px; padding: 1.5rem; margin: 2rem 0; backdrop-filter: blur(10px); }
        .status-text { font-size: 1.1rem; font-weight: 500; }
        .features { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
                    gap: 1rem; margin-top: 2rem; }
        .feature { background: rgba(255, 255, 255, 0.05); border-radius: 8px; padding: 1rem;
                   border: 1px solid rgba(255, 255, 255, 0.1); }
        .feature-title { font-weight: 600; margin-bottom: 0.5rem; }
        .feature-desc { font-size: 0.9rem; opacity: 0.8; }
        .footer { margin-top: 3rem; opacity: 0.7; font-size: 0.9rem; }
    </style>
</head>
<body>
    <div class="container">
        <div class="logo">🚀</div>
        <h1 class="title">Portico</h1>
        <p class="subtitle">Platform as a Service</p>
        <div class="status">
            <div class="status-text">🎉 Congrats! Portico is running</div>
        </div>
        <div class="features">
            <div class="feature">
                <div class="feature-title">Caddy Proxy</div>
                <div class="feature-desc">Automatic SSL and routing</div>
            </div>
            <div class="feature">
                <div class="feature-title">Docker Compose</div>
                <div class="feature-desc">Container orchestration</div>
            </div>
            <div class="feature">
                <div class="feature-title">Secrets Management</div>
                <div class="feature-desc">Secure configuration</div>
            </div>
            <div class="feature">
                <div class="feature-title">Go CLI</div>
                <div class="feature-desc">Easy application management</div>
            </div>
        </div>
        <div class="footer">
            <p>Ready to deploy your first application?</p>
            <p><code>portico apps create my-app</code></p>
        </div>
    </div>
</body>
</html>
EOF

sudo chown portico:portico /home/portico/static/index.html

# Create initial Caddyfile
echo -e "${BLUE}⚙️  Creating initial Caddyfile...${NC}"
sudo tee /home/portico/reverse-proxy/Caddyfile > /dev/null << 'EOF'
# Portico Caddyfile
# Auto-generated by Portico

# Default catch-all - serve Portico welcome page
localhost {
    root * /home/portico/static
    file_server
    
    # Fallback to index.html for any route
    try_files {path} /index.html
    
    # Logging
    log {
        output file /var/log/caddy/portico.log
        format json
    }
}
EOF

sudo chown portico:portico /home/portico/reverse-proxy/Caddyfile

# Create portico config
echo -e "${BLUE}📋 Creating Portico configuration...${NC}"
sudo tee /home/portico/config.yml > /dev/null << 'EOF'
portico_home: /home/portico
apps_dir: /home/portico/apps
proxy_dir: /home/portico/reverse-proxy
registry:
  type: internal
  url: localhost:5000
EOF

sudo chown portico:portico /home/portico/config.yml

# Create reverse-proxy docker-compose
echo -e "${BLUE}🚀 Creating reverse-proxy with Docker...${NC}"
sudo tee /home/portico/reverse-proxy/docker-compose.yml > /dev/null << 'EOF'
version: '3.8'

services:
  caddy:
    image: caddy:2-alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - /home/portico/static:/home/portico/static
      - caddy_data:/data
      - caddy_config:/config
    restart: unless-stopped

volumes:
  caddy_data:
  caddy_config:
EOF

sudo chown portico:portico /home/portico/reverse-proxy/docker-compose.yml

# Start the reverse-proxy
cd /home/portico/reverse-proxy
sudo -u portico docker-compose up -d

echo ""
echo -e "${GREEN}✅ Portico installation completed!${NC}"
echo ""
echo -e "${GREEN}🎉 Congrats! Portico is running${NC}"
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
echo "   docker-compose -f /home/portico/reverse-proxy/docker-compose.yml logs -f"
echo ""
echo -e "${YELLOW}⚠️  Note: You may need to log out and back in for Docker group changes to take effect.${NC}"
echo ""