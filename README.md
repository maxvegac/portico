# Portico
**License:** MIT  

Portico is a Platform as a Service (PaaS) similar to Dokku, but with the following distinctive features:

## Key Features

- **Caddy as reverse proxy**: Single Caddy instance serves static files and routes to application services
- **Docker Compose**: Each application runs its own services (API, database, etc.)
- **Secrets Management**: Secure handling of sensitive data using Docker secrets
- **Go CLI**: Command line tool for managing applications
- **Docker Registry**: Support for external and internal registries
- **Addon System**: Manage databases, cache stores, and tools with ease

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
│       ├── Caddyfile
│       └── env/                    # Secrets directory
│           ├── database_password
│           ├── api_key
│           └── jwt_secret
├── addons/
│   ├── definitions/                # Addon definition YAMLs
│   │   ├── postgresql.yml
│   │   ├── mysql.yml
│   │   ├── redis.yml
│   │   └── ...
│   └── instances/                   # Addon instances
│       └── my-postgres/
│           ├── docker-compose.yml
│           └── data/
├── static/
│   └── index.html                  # Welcome page - Catch-all
└── config.yml
```

## Templates

Portico uses embedded template files for generating configurations. Templates are included in the binary at build time:

- `caddy-app.tmpl` - Individual app Caddy config (used for generating Caddyfiles)
- `docker-compose.tmpl` - Not used (docker-compose.yml is generated directly in Go)
- `app.yml.tmpl` - Not used (app.yml is only read for backwards compatibility)

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

## Usage

### Application Management

```bash
# List all applications
portico list

# Create new application
portico create my-app

# Start application
portico up my-app

# Stop application
portico down my-app

# Reset application (regenerate configs and restart)
portico reset my-app

# Destroy application
portico destroy my-app

# Change to application directory
portico cd my-app

# Preserve docker-compose.yml (prevent Portico from overwriting manual changes)
portico preserve my-app

# Execute command in container
portico exec my-app [service] [command...]

# Open interactive shell in container
portico shell my-app [service] [shell]

# Show application status
portico status my-app
```

### Domain Management

```bash
# Add domain to application
portico domains my-app add example.com

# Remove domain from application
portico domains my-app remove example.com
```

### Port Management

```bash
# Add port mapping
portico ports my-app add 8080 3000 [--name service-name]

# Delete port mapping
portico ports my-app delete 8080 3000 [--name service-name]

# List port mappings
portico ports my-app list [--name service-name]
```

**Note**: If the application has only one service, the `--name` flag is optional.

### Storage Management

```bash
# Add volume mount
portico storage my-app add /host/path /container/path [--name service-name]

# Delete volume mount
portico storage my-app delete /host/path /container/path [--name service-name]

# List volume mounts
portico storage my-app list [--name service-name]
```

**Note**: If the application has only one service, the `--name` flag is optional.

### Addon Management

#### List Available Addons

```bash
# List all available addon types
portico addons list

# List versions for a specific addon type
portico addons list postgresql
```

#### List Addon Instances

```bash
# List all created addon instances
portico addons instances
```

#### Create Addon Instance

```bash
# Create shared addon instance (default)
portico addons create my-postgres --type postgresql --version 18

# Create dedicated addon instance for a specific app
portico addons create my-db --type mysql --version 8.4.7 --mode dedicated --app my-app
```

#### Manage Addon Instances

```bash
# Start addon instance
portico addons up my-postgres

# Stop addon instance
portico addons down my-postgres

# Delete addon instance
portico addons delete my-postgres
```

#### Add Inline Addons (Redis/Valkey)

```bash
# Add Redis as a service within an application
portico addons add my-app redis --version 8

# Add Valkey as a service within an application
portico addons add my-app valkey --version 7.2
```

#### Link Application to Addon

```bash
# Link application to a shared or dedicated addon instance
portico addons link my-app my-postgres
```

This automatically injects environment variables (host, port, database, user, password) into all services of the application.

#### Database Management

```bash
# Create database in addon instance
portico addons database my-postgres create mydb

# Delete database from addon instance
portico addons database my-postgres delete mydb

# List databases in addon instance
portico addons database my-postgres list
```

### Available Addons

- **PostgreSQL**: Versions 15, 16, 17, 18
- **MySQL**: Versions 5.7, 8.4.7, 9.5.0
- **MariaDB**: Versions 10, 11, 12
- **MongoDB**: Versions 6, 7
- **Redis**: Versions 6, 7, 8
- **Valkey**: Versions 7.0, 7.2

### Application Structure

Each application has:
- `app.yml` - Application configuration
- `docker-compose.yml` - Service definitions
- `Caddyfile` - Caddy configuration
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
