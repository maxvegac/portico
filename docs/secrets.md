# Portico Secrets Management

Portico provides a secure way to manage application secrets using Docker secrets.

## How it works

1. **Secrets are stored** in the `env/` directory of each application
2. **Docker Compose** automatically mounts these secrets into containers
3. **Secrets are accessible** at `/run/secrets/` inside containers

## Directory Structure

```
/home/portico/apps/my-app/
├── app.yml
├── docker-compose.yml
├── Caddyfile
└── env/                    # Secrets directory
    ├── database_password
    ├── api_key
    ├── jwt_secret
    └── ssl_certificate
```

## Using Secrets in Applications

### In your application code:

```javascript
// Node.js example
const fs = require('fs');

// Read secret from mounted path
const databasePassword = fs.readFileSync('/run/secrets/database_password', 'utf8').trim();
const apiKey = fs.readFileSync('/run/secrets/api_key', 'utf8').trim();

console.log('Database password:', databasePassword);
```

```python
# Python example
import os

# Read secret from mounted path
with open('/run/secrets/database_password', 'r') as f:
    database_password = f.read().strip()

with open('/run/secrets/api_key', 'r') as f:
    api_key = f.read().strip()

print(f"Database password: {database_password}")
```

### In Docker Compose:

```yaml
version: '3.8'

services:
  app:
    image: my-app:latest
    secrets:
      - database_password
      - api_key
    volumes:
      - ./env:/run/secrets:ro

secrets:
  database_password:
    file: ./env/database_password
  api_key:
    file: ./env/api_key
```

## Security Best Practices

1. **Never commit secrets** to version control
2. **Use .gitignore** to exclude the `env/` directory
3. **Set proper permissions** on secret files (600)
4. **Rotate secrets** regularly
5. **Use different secrets** for different environments

## Managing Secrets

### Create a new secret:
```bash
echo "my-secret-value" > /home/portico/apps/my-app/env/new_secret
chmod 600 /home/portico/apps/my-app/env/new_secret
```

### Update docker-compose.yml:
```yaml
services:
  app:
    secrets:
      - new_secret

secrets:
  new_secret:
    file: ./env/new_secret
```

### Deploy the updated application:
```bash
portico apps deploy my-app
```

## Environment Variables vs Secrets

- **Environment Variables**: For non-sensitive configuration
- **Secrets**: For sensitive data (passwords, API keys, certificates)

Example:
```yaml
# app.yml - Environment variables
environment:
  NODE_ENV: production
  PORT: 3000
  DEBUG: false

# env/ directory - Secrets
database_password
api_key
jwt_secret
```
