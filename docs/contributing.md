---
title: Contributing
description: Contribute to Agenttra development
---

# Contributing to Agenttra

Thank you for your interest in contributing!

## Development Setup

### Prerequisites

- Node.js v20+
- pnpm v10.28+
- Go v1.26+
- Docker

### Quick Start

```bash
pnpm install
cp .env.example .env
make setup
make start
```

### Testing

```bash
# All checks
make check

# Individual
pnpm typecheck
pnpm test
make test
```

## Pull Requests

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run `make check`
5. Submit a PR

## Code Style

- Go: standard `gofmt`
- TypeScript: strict mode enabled
- Commit messages: conventional format

## License

By contributing, you agree your code is licensed under Apache 2.0.
