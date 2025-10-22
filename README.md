# Portico

Portico is a Platform as a Service (PaaS) similar to Dokku, but with the following distinctive features:

## Key Features

- **Caddy as reverse proxy**: Single Caddy instance serves static files and routes to application services
- **Docker Compose**: Each application runs its own services (API, database, etc.)
- **Secrets Management**: Secure handling of sensitive data using Docker secrets
- **Go CLI**: Command line tool for managing applications
- **Docker Registry**: Support for external and internal registries

## System Structure

```
/home/portico/
├── reverse-proxy/
│   ├── Caddyfile
│   └── docker-compose.yml          # Caddy reverse proxy
├── apps/
│   └── WHATEVERAPP/
│       ├── app.yml
│       ├── docker-compose.yml      # Application services
│       ├── caddy.conf
│       └── env/                    # Secrets directory
│           ├── database_password
│           ├── api_key
│           └── jwt_secret
├── static/
│   └── index.html                  # Welcome page
└── config.yml
```

## Templates

Portico uses template files for generating configurations:

```
templates/
├── caddyfile.tmpl          # Main Caddyfile template
├── caddy-app.tmpl          # Individual app Caddy config
├── docker-compose.tmpl     # Docker Compose template
└── app.yml.tmpl            # App configuration template
```

## Installation

### One-Line Install (Recommended)

```bash
# Install latest stable release
curl -fsSL https://raw.githubusercontent.com/portico/portico/main/install.sh | bash
```

### Development Install

```bash
# Clone the repository
git clone https://github.com/portico/portico.git
cd portico

# Build from source
make build
sudo make install
```

### Manual Installation

```bash
# 1. Create portico user
sudo useradd -m -s /bin/bash portico

# 2. Create directories
sudo mkdir -p /home/portico/{apps,reverse-proxy,static}
sudo chown -R portico:portico /home/portico

# 3. Build and install CLI
make build
sudo make install

# 4. Setup environment
make setup

# 5. Start Caddy service
sudo systemctl start portico-caddy
```

### Verify Installation

Visit http://localhost to see the Portico welcome page with "🎉 Congrats! Portico is running"

## Version Management

Portico uses semantic versioning with automatic releases:

### Version Types

- **Stable releases** (`v1.0.0`): Tagged releases from `main` branch
- **Dev releases** (`v1.0.0-dev-abc123`): Pre-releases from `develop` branch
- **Development builds**: Built from source for development

### Creating Releases

```bash
# Create patch release (1.0.0 -> 1.0.1)
./scripts/version.sh patch

# Create minor release (1.0.0 -> 1.1.0)
./scripts/version.sh minor

# Create major release (1.0.0 -> 2.0.0)
./scripts/version.sh major

# Create dev release
./scripts/version.sh dev

# Create custom release
./scripts/version.sh release 1.2.3
```

### Automatic Builds

- **Push to `main`**: Creates stable release
- **Push to `develop`**: Creates dev release  
- **Create tag `v*`**: Creates stable release
- **Pull Request**: Creates development build

## Usage

### Basic Commands

```bash
# List all applications
portico apps list

# Create new application
portico apps create my-app

# Deploy application
portico apps deploy my-app

# Destroy application
portico apps destroy my-app

# Show version
portico version
```

### Application Structure

Each application has:
- `app.yml` - Application configuration
- `docker-compose.yml` - Service definitions
- `caddy.conf` - Caddy configuration
- `env/` - Secret files
