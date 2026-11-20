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
│       ├── docker-compose.yml      # Application services (source of truth, metadata in x-portico)
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
├── repos/
│   └── your-app.git                # Bare git repo for push-to-deploy
├── templates/                      # Customizable templates (extracted by `portico init`)
├── www/
│   └── index.html                  # Welcome page - Catch-all
├── logs/                           # Caddy access logs and app log mounts
└── config.yml
```

## Templates

Portico uses embedded template files for generating configurations. Templates are included in the binary at build time:

- `caddy-app.tmpl` - Individual app Caddy config (used for generating Caddyfiles)
- `docker-compose.tmpl` - App docker-compose.yml (Portico merges back any fields it doesn't manage)

`portico init` extracts them to `/home/portico/templates/`, where they can be customized
(files there take precedence over the embedded ones).

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

# Destroy application (asks for confirmation; use --force in scripts)
portico destroy my-app

# Build the Dockerfile in the current directory and deploy it
portico deploy my-app [--dockerfile Dockerfile.prod] [--build-arg KEY=VALUE] [--service web]

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

### Deploying

```bash
# On the server: authorize your key (git push only; --admin grants a full shell)
portico ssh add ~/.ssh/id_ed25519.pub my-laptop
portico ssh list
portico ssh remove my-laptop

# Push to deploy (builds the Dockerfile in the repository root;
# the app and its repository are created on the first push)
git remote add portico portico@your-server:my-app.git
git push portico main

# Deploy a pre-built image (e.g. from CI)
portico service my-app web image ghcr.io/user/my-app:v1.2.3 [--port 3000]

# Background worker (no HTTP)
portico service my-app worker image ghcr.io/user/worker:latest --no-http-port

# Scale a service
portico service my-app web scale 3
```

### HTTP Configuration

```bash
# Internal container port Caddy proxies to (default: 8080)
portico set my-app http-port 3000

# Service that receives HTTP traffic (default: "web", or the only service)
portico set my-app http-service api

# Enable / disable HTTP
portico set my-app http on
portico set my-app http off

# IP used to generate default domains (appname.IP.sslip.io)
portico set external-ip 203.0.113.10
```

### Environment Variables and Secrets

```bash
portico env my-app [service-name] add KEY value
portico env my-app [service-name] list
portico env my-app [service-name] del KEY

# Secrets are files in env/, mounted at /run/secrets/<name>
portico secrets my-app [service-name] add db_password s3cr3t
portico secrets my-app [service-name] list
portico secrets my-app [service-name] del db_password
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
# Publish container port 5432 on host port 5433
portico ports my-app [service-name] add 5432 5433

# Remove a published port (host:container)
portico ports my-app [service-name] delete 5433:5432

# List published ports
portico ports my-app [service-name] list
```

**Note**: If the application has only one service, `service-name` is optional.
Services don't need published ports to receive HTTP traffic: Caddy reaches them over the internal network.

### Storage Management

```bash
# Add volume mount
portico storage my-app [service-name] add /host/path /container/path

# Delete volume mount
portico storage my-app [service-name] delete /host/path /container/path

# List volume mounts
portico storage my-app [service-name] list
```

**Note**: If the application has only one service, `service-name` is optional.

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
portico addons my-postgres up

# Stop addon instance
portico addons my-postgres down

# Delete addon instance and its data (asks for confirmation)
portico addons my-postgres delete
```

#### Add Inline Addons (Redis/Valkey)

```bash
# Add Redis as a service within an application
portico addons my-app add redis --version 8

# Add Valkey as a service within an application
portico addons my-app add valkey --version 7.2
```

#### Link Application to Addon

```bash
# Link application to a shared or dedicated addon instance
portico addons my-app link my-postgres [--database mydb]
```

This creates the database (default: the app name) if needed and injects environment variables
(host, port, database, user, password and a connection URL, e.g. `POSTGRES_URL`) into all services of the application.

Addon passwords are randomly generated, and addon ports are only published on `127.0.0.1`
(apps reach them over the internal Docker network).

#### Admin Tools (Adminer / mongo-express)

```bash
# Adminer (PostgreSQL, MySQL, MariaDB): only on 127.0.0.1, reach it through an SSH tunnel
portico addons create dbadmin --type adminer
portico addons dbadmin up
ssh -N -L 9080:127.0.0.1:9080 you@server   # then open http://localhost:9080

# Also publish it on a domain (HTTPS + basic auth with a generated password)
portico addons create dbadmin --type adminer --target my-postgres --domain db.example.com

# mongo-express for a MongoDB instance (--target is required)
portico addons create mongoadmin --type mongo-express --target my-mongo
```

In Adminer, log in with the database credentials (stored in
`addons/instances/<db>/secrets/`); the server is the instance host shown by
`portico addons instances` (e.g. `addon-my-postgres`). The tunnel needs a key added with
`portico ssh add --admin` (git-only keys can't forward ports). The generated
password is printed once and stored in `addons/instances/<tool>/secrets/admin_password`.
Deleting the tool (`portico addons dbadmin delete`) also removes its proxy config.

Installs made before admin tools existed must refresh the reverse proxy once so
it loads the published tools: `portico init && cd /home/portico/reverse-proxy && docker compose up -d`.

#### Database Management

```bash
# Create database in addon instance
portico addons my-postgres database create mydb

# Delete database from addon instance
portico addons my-postgres database delete mydb

# List databases in addon instance
portico addons my-postgres database list
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
- `docker-compose.yml` - Service definitions; Portico metadata (domain, HTTP port/service, replicas) lives under `x-portico`
- `Caddyfile` - Caddy configuration (regenerated by Portico; manual edits are detected)
- `env/` - Secret files

Every service joins the shared `portico-network` with the alias `<app>-<service>`,
which is what Caddy proxies to.

### Upgrading an existing server

After updating the binary, refresh the templates and the reverse proxy
(this keeps your `config.yml`):

```bash
sudo -u portico portico init
cd /home/portico/reverse-proxy && sudo -u portico docker compose up -d
# Regenerate each app's compose file and Caddyfile
sudo -u portico portico reset my-app
```

## Author

**Maximiliano Vega** - [GitHub](https://github.com/maxvegac)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Inspired by [Dokku](https://dokku.com/)
- Built with [Caddy](https://caddyserver.com/)
- Powered by [Docker](https://www.docker.com/)
- Written in [Go](https://golang.org/)
