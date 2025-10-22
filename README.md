# Portico
**License:** MIT  

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
│   └── your-app/
│       ├── app.yml
│       ├── docker-compose.yml      # Application services
│       ├── caddy.conf
│       └── env/                    # Secrets directory
│           ├── database_password
│           ├── api_key
│           └── jwt_secret
├── static/
│   └── index.html                  # Welcome page - Catch-all
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
curl -fsSL https://raw.githubusercontent.com/maxvegac/portico/main/install.sh | bash

# Install development build
curl -fsSL https://raw.githubusercontent.com/maxvegac/portico/main/install.sh | bash -s -- --dev
```

### Development Install

```bash
# Clone the repository
git clone https://github.com/maxvegac/portico.git
cd portico

# Build from source
make build
sudo make install
```

### Verify Installation

Visit http://localhost or the IP of your server to see the Portico welcome page with "🎉 Congrats! Portico is running"

## Version Management

Portico uses an intelligent versioning system that automatically detects the context:

### 🏷️ **Stable Releases**
- **Trigger**: Git tags (e.g., `v1.0.0`)
- **Binary**: `portico-linux-amd64`
- **Release**: GitHub release with binaries
- **Installation**: `curl -fsSL https://raw.githubusercontent.com/maxvegac/portico/main/install.sh | bash`

### 🚀 **Development Latest**
- **Trigger**: Push to `main` branch
- **Binary**: `portico-dev-latest-linux-amd64`
- **Release**: GitHub prerelease (always updated)
- **Installation**: `curl -fsSL https://raw.githubusercontent.com/maxvegac/portico/main/install.sh | bash`

### 🌿 **Feature Branch Builds**
- **Trigger**: Push to any branch (except `main`)
- **Binary**: `portico-{branch}-{commit}-linux-amd64`
- **Release**: GitHub prerelease
- **Example**: `portico-feature-auth-abc1234-linux-amd64`

### 📦 **Automatic Binary Generation**

The system automatically generates the appropriate binary name based on context:

```bash
# Stable release
portico v1.0.0                    # → portico-linux-amd64

# Development latest  
portico dev-latest                # → portico-dev-latest-linux-amd64

# Feature branch
portico feature-auth-abc1234      # → portico-feature-auth-linux-amd64
```

### Creating Releases

```bash
# Create patch release (1.0.0 -> 1.0.1)
./scripts/version.sh patch

# Create minor release (1.0.0 -> 1.1.0)
./scripts/version.sh minor

# Create major release (1.0.0 -> 2.0.0)
./scripts/version.sh major
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

## Author

**Maximiliano Vega** - [GitHub](https://github.com/maxvegac)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Inspired by [Dokku](https://dokku.com/)
- Built with [Caddy](https://caddyserver.com/)
- Powered by [Docker](https://www.docker.com/)
- Written in [Go](https://golang.org/)
