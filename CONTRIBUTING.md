# Contributing to ShellCN

Thank you for your interest in contributing to ShellCN! We welcome contributions from the community.

## Code of Conduct

Please be respectful and constructive in all interactions. We aim to maintain a welcoming and inclusive community.

## How to Contribute

### Reporting Bugs

1. Check if the bug has already been reported in [Issues](https://github.com/yourusername/shellcn/issues)
2. If not, create a new issue with:
   - Clear title and description
   - Steps to reproduce
   - Expected vs actual behavior
   - Environment details (OS, Go version, etc.)

### Suggesting Features

1. Check [Discussions](https://github.com/yourusername/shellcn/discussions) for similar suggestions
2. Create a new discussion or issue with:
   - Clear use case
   - Proposed solution
   - Any alternatives considered

### Pull Requests

1. Fork the repository
2. Create a feature branch from `main`
3. Make your changes
4. Add tests if applicable
5. Ensure all tests pass
6. Update documentation
7. Submit a pull request

## Development Setup

See [README.md](README.md#development) for development setup instructions.

## Guidelines

### Code Style

- **Go:** Follow standard Go conventions (`gofmt`, `golint`)
- **TypeScript/React:** Use Prettier and ESLint
- **Commits:** Use clear, descriptive commit messages

### Testing

- Write unit tests for new features
- Ensure existing tests pass
- Run `make test` before submitting

### Documentation

- Update relevant documentation files
- Add code comments for complex logic
- Update README.md if needed

## Module Development

When adding new modules:

1. Create module directory in `internal/modules/`
2. Register permissions in `permissions.go` with `init()`
3. Follow existing module patterns (see SSH module)
4. Add comprehensive tests
5. Update documentation

## Permission System

All new features must use the permission system:

- Register permissions in module's `init()`
- Define permission dependencies correctly
- Use middleware for API route protection
- Test permission checks

## Questions?

Feel free to ask questions in [Discussions](https://github.com/yourusername/shellcn/discussions) or open an issue.

Thank you for contributing! 🚀
