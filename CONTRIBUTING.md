# Contributing to Nitro

Thank you for your interest in contributing to Nitro! This document provides guidelines and information for contributors.

Please follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## How to Contribute

### Reporting Bugs

1. Check existing [issues](https://github.com/rodrigocelsius/go-nitro/issues) to avoid duplicates
2. Create a new issue with a clear title and description
3. Include:
   - Go version (`go version`)
   - Nitro version
   - Steps to reproduce
   - Expected vs actual behavior
   - Code samples if applicable

### Suggesting Features

1. Check existing issues and discussions
2. Create a new issue with the `enhancement` label
3. Describe the feature, use case, and benefits

### Pull Requests

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Make your changes
4. Add tests for new functionality
5. Ensure tests pass (`make test`, or the nested module you changed)
6. Update documentation if needed
7. Submit a pull request

## Development Setup

### Prerequisites

- Go 1.25 or higher
- Git

### Clone and Setup

```bash
git clone https://github.com/rodrigocelsius/go-nitro.git
cd go-nitro
make test-pkg
```

### Run Tests

This repository is **multi-module**. `go test ./...` from the **repo root does not work**. Each of `pkg/`, `cmd/nitrogen/`, `blinq/`, `adapters/fasthttp/`, … has its own `go.mod`. Use the root Makefile (same as CI):

```bash
make test          # pkg + CLI + blinq + fasthttp + pgx adapter
make test-pkg
make test-cli
make test-blinq
make test-http

# inside a single module
cd pkg && go test ./...
cd adapters/fasthttp && go test ./...
cd pkg && go test -cover ./...
```

### Run Benchmarks

```bash
cd bench
go test -bench=. -benchmem -count=3
```

## Code Style

### Go Conventions

- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Use `gofmt` to format code
- Use `go vet` to check for issues
- Keep functions focused and small
- Write meaningful comments in English

### Naming Conventions

- Exported names: `PascalCase`
- Unexported names: `camelCase`
- Interfaces: `-er` suffix (e.g., `Handler`, `Logger`)
- Constants: `PascalCase` or `ALL_CAPS` for HTTP headers

### Documentation

- All exported functions must have godoc comments
- Comments should explain **what** and **why**, not **how**
- Keep comments up-to-date with code changes

## Project Structure

```
go-nitro/
├── pkg/                 # Public facade (import github.com/rodrigocelsius/go-nitro/pkg/v2)
├── domain/              # Core types (zero dependencies)
├── ports/               # Interfaces
├── usecases/            # Framework logic
├── adapters/            # fasthttp, auth, cache, otel, …
├── data/                # Gateway, repository, pgx
├── blinq/               # Typed DSL
├── codegen/             # Code generation
├── cmd/nitrogen/        # CLI
├── examples/            # Index + v1/ recipes
└── docs/                # English canonical + docs/pt/
```

### Adding New Features

1. **Domain layer**: Add types to `domain/` (no external dependencies)
2. **Ports**: Define interfaces in `ports/`
3. **Usecases**: Implement business logic in `usecases/`
4. **Adapters**: Create concrete implementations in `adapters/`
5. **Facade**: Export via `pkg/nitro.go`

### Adding Middleware

1. Create a new file in `adapters/fasthttp/`
2. Implement the middleware function
3. Add to `pkg/nitro.go` as a public function
4. Add tests
5. Update documentation

## Testing Guidelines

### Unit Tests

- Test one thing per test function
- Use table-driven tests
- Mock external dependencies
- Aim for >80% coverage

```go
func TestUser_GetID(t *testing.T) {
    tests := []struct {
        name     string
        user     User
        expected string
    }{
        {"with ID", User{ID: "123"}, "123"},
        {"without ID", User{}, ""},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            if got := tt.user.GetID(); got != tt.expected {
                t.Errorf("GetID() = %v, want %v", got, tt.expected)
            }
        })
    }
}
```

### Integration Tests

- Test real HTTP requests with `app.Test()`
- Test database operations with test containers
- Test authentication flows end-to-end

### Benchmarks

- Benchmark performance-critical code
- Use `b.ReportAllocs()` for memory analysis
- Compare before/after changes

## Commit Messages

Use conventional commits:

```
feat: add new middleware for request logging
fix: resolve race condition in context pool
docs: update API reference for auth
test: add unit tests for JWT provider
perf: optimize string allocation in handler
refactor: simplify error handling in router
```

## Pull Request Guidelines

### Before Submitting

- [ ] Code compiles without errors
- [ ] All tests pass
- [ ] No new lint warnings
- [ ] Documentation updated
- [ ] Commit messages follow conventions

### PR Description

Include:
- Summary of changes
- Motivation and context
- Type of change (bug fix, feature, breaking change)
- Testing performed
- Checklist

## Code Review

### What We Look For

- Code correctness and clarity
- Test coverage
- Performance implications
- Security considerations
- Documentation completeness
- Adherence to project conventions

### Responding to Feedback

- Address all comments
- Ask for clarification if needed
- Make incremental commits for review

## Release Process

1. Update version in relevant files
2. Update CHANGELOG.md
3. Create a release tag
4. Publish release notes

## Questions?

- Open a [discussion](https://github.com/rodrigocelsius/go-nitro/discussions)
- Check the [FAQ](docs/extra/faq.md)
- Review existing [issues](https://github.com/rodrigocelsius/go-nitro/issues)
- Read the public [README](README.md) and [AGENTS.md](AGENTS.md) contracts

Thank you for contributing to Nitro!
