# Contributing to Portico

Thank you for your interest in contributing to Portico! This document provides guidelines for contributing to the project.

## Getting Started

### Prerequisites

- Go 1.21 or later
- Docker and Docker Compose
- Git

### Development Setup

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/your-username/portico.git
   cd portico
   ```

3. Build the project:
   ```bash
   make build
   ```

4. Run tests:
   ```bash
   make test
   ```

## Development Workflow

### Branch Strategy

- `main` - Stable releases
- `develop` - Development branch
- `feature/*` - Feature branches
- `bugfix/*` - Bug fix branches

### Making Changes

1. Create a feature branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Make your changes
3. Add tests for new functionality
4. Run tests and linting:
   ```bash
   make test
   make lint
   ```

5. Commit your changes:
   ```bash
   git commit -m "feat: add your feature"
   ```

6. Push to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```

7. Create a Pull Request

## Code Style

### Go Code

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable and function names
- Add comments for exported functions
- Keep functions small and focused

### Commit Messages

Use conventional commits:

- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation changes
- `style:` - Code style changes
- `refactor:` - Code refactoring
- `test:` - Test additions/changes
- `chore:` - Maintenance tasks

Examples:
```
feat: add support for custom domains
fix: resolve Caddy configuration issue
docs: update installation instructions
```

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific test
go test ./internal/app/...
```

### Writing Tests

- Write unit tests for new functionality
- Use table-driven tests when appropriate
- Mock external dependencies
- Aim for high test coverage

## Documentation

### Code Documentation

- Document all exported functions
- Use clear, concise comments
- Provide examples for complex functions

### User Documentation

- Update README.md for user-facing changes
- Add examples for new features
- Keep installation instructions current

## Release Process

### Version Management

Use the version script to manage releases:

```bash
# Patch release (1.0.0 -> 1.0.1)
./scripts/version.sh patch

# Minor release (1.0.0 -> 1.1.0)
./scripts/version.sh minor

# Major release (1.0.0 -> 2.0.0)
./scripts/version.sh major
```

### Release Types

- **Stable releases**: Tagged releases from `main`
- **Dev releases**: Pre-releases from `develop`
- **Development builds**: Built from source

## Reporting Issues

### Bug Reports

When reporting bugs, please include:

1. Portico version
2. Operating system
3. Steps to reproduce
4. Expected behavior
5. Actual behavior
6. Error messages/logs

### Feature Requests

For feature requests, please include:

1. Use case description
2. Proposed solution
3. Alternative solutions considered
4. Additional context

## Code Review Process

### For Contributors

1. Ensure all tests pass
2. Update documentation if needed
3. Respond to review feedback promptly
4. Keep PRs focused and small

### For Reviewers

1. Test the changes locally
2. Check code quality and style
3. Verify tests and documentation
4. Provide constructive feedback

## Community Guidelines

- Be respectful and inclusive
- Help others learn and grow
- Follow the code of conduct
- Welcome newcomers

## Getting Help

- GitHub Issues for bug reports and feature requests
- GitHub Discussions for questions and ideas
- Documentation in the `docs/` directory

Thank you for contributing to Portico! 🚀
