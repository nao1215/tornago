# Contributing Guide

## Introduction

Thank you for considering contributing to the tornago project! This document explains how to contribute to the project. We welcome all forms of contributions, including code contributions, documentation improvements, bug reports, and feature suggestions.

> [!IMPORTANT]
> **Legal Notice**: This library is intended for legitimate purposes only. All contributions must comply with applicable laws and must not facilitate illegal activities. Contributors are responsible for ensuring their contributions align with these principles.

## Setting Up Development Environment

### Prerequisites

#### Installing Go

tornago development requires Go 1.25 or later.

**macOS (using Homebrew)**
```bash
brew install go
```

**Linux (for Ubuntu)**
```bash
# Using snap
sudo snap install go --classic

# Or download from official site
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
source ~/.profile
```

**Windows**
Download and run the installer from the [official Go website](https://go.dev/dl/).

Verify installation:
```bash
go version
```

#### Installing Tor

Tornago requires the Tor daemon to be installed for integration tests.

**Ubuntu/Debian**
```bash
sudo apt update
sudo apt install tor
```

**macOS (using Homebrew)**
```bash
brew install tor
```

**Verify Tor installation:**
```bash
tor --version
```

### Cloning the Project

```bash
git clone https://github.com/nao1215/tornago.git
cd tornago
```

### Verification

To verify that your development environment is set up correctly, run the following commands:

```bash
# Run unit tests (fast, no Tor required)
make test

# Run integration tests (requires Tor, slower)
make integration-test

# Run linter
golangci-lint run
```

## Development Workflow

### Branch Strategy

- `main` branch is the latest stable version
- Create new branches from `main` for new features or bug fixes
- Branch naming examples:
  - `feature/add-circuit-rotation` - New feature
  - `fix/issue-123` - Bug fix
  - `docs/update-readme` - Documentation update

### Coding Standards

This project follows these standards:

1. **Conform to [Effective Go](https://go.dev/doc/effective_go)**
2. **Use immutable configuration pattern** - All configs use functional options and are immutable after construction
3. **Always add comments to public functions, variables, and structs** - Follow godoc conventions
4. **Error wrapping with context** - Use the `newError()` helper with `ErrorKind`, `Op`, and message
5. **Defensive copying** - Return copies of slices/maps from getters to maintain immutability
6. **Thread-safety** - Use mutexes where needed (see `ControlClient` example)

### Architecture Guidelines

Please refer to [CLAUDE.md](CLAUDE.md) for detailed architecture information:

- Configuration pattern (immutable with functional options)
- Error handling conventions (`TornagoError` with `ErrorKind`)
- Testing infrastructure (`TestServer` usage)
- Core component interactions

### Writing Tests

Tests are important. Please follow these guidelines:

1. **Unit tests**: Aim for 80% or higher coverage (enforced by `.octocov.yml`)
2. **Integration tests**: Mark with `if testing.Short() { t.Skip() }` and `TORNAGO_INTEGRATION=1` environment variable
3. **Test readability**: Write clear test cases with descriptive names
4. **Use TestServer**: For integration tests requiring a Tor instance

Test example:
```go
func TestClient_Dial(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    ts := tornago.StartTestServer(t)
    defer ts.Stop()

    client := ts.Client(t)

    conn, err := client.Dial("tcp", "example.com:80")
    if err != nil {
        t.Fatalf("Dial failed: %v", err)
    }
    defer conn.Close()

    // Test connection...
}
```

### Running Tests

```bash
# Unit tests only (fast)
make test

# Integration tests (requires Tor, slower)
make integration-test

# Using external Tor for integration tests
TOR_USE_EXTERNAL=1 \
TORNAGO_TOR_CONTROL=127.0.0.1:9051 \
TORNAGO_TOR_SOCKS=127.0.0.1:9050 \
TORNAGO_TOR_COOKIE=$HOME/.tor/control.authcookie \
make integration-test

# Check coverage
go tool cover -html=coverage.out
```

## Using AI Assistants (LLMs)

We actively encourage the use of AI coding assistants to improve productivity and code quality. Tools like Claude Code, GitHub Copilot, and Cursor are welcome for:

- Writing boilerplate code
- Generating comprehensive test cases
- Improving documentation
- Refactoring existing code
- Finding potential bugs
- Suggesting performance optimizations
- Translating documentation

### Guidelines for AI-Assisted Development

1. **Review all generated code**: Always review and understand AI-generated code before committing
2. **Maintain consistency**: Ensure AI-generated code follows our coding standards in [CLAUDE.md](CLAUDE.md)
3. **Test thoroughly**: AI-generated code must pass all tests and linting
4. **Security awareness**: Be especially careful with security-sensitive code (authentication, control commands)
5. **Use project configuration**: We provide `CLAUDE.md` to help AI assistants understand our project standards

## Creating Pull Requests

### Preparation

1. **Check or Create Issues**
   - Check if there are existing issues
   - For major changes, we recommend discussing the approach in an issue first
   - **Security considerations**: Discuss features that could be misused

2. **Write Tests**
   - Always add tests for new features
   - For bug fixes, create tests that reproduce the bug
   - Include both unit tests and integration tests where appropriate
   - AI tools can help generate comprehensive test cases

3. **Quality Check**
   ```bash
   # Ensure all tests pass
   make test
   make integration-test

   # Linter check
   golangci-lint run

   # Check coverage (80% or higher)
   go test -cover ./...
   ```

### Submitting Pull Request

1. Create a Pull Request from your forked repository to the main repository
2. PR title should briefly describe the changes
3. Include the following in PR description:
   - Purpose and content of changes
   - Related issue number (if any)
   - Test method (including integration test results)
   - Reproduction steps for bug fixes
   - **Security implications** (if any)

### About CI/CD

GitHub Actions automatically checks the following items:

- **Cross-platform testing**: Test execution on Linux (primary support)
- **Linter check**: Static analysis with golangci-lint (40+ linters enabled)
- **Test coverage**: Maintain 80% or higher coverage (excludes `cmd/` and `examples/`)
- **Build verification**: Successful builds

Merging is not possible unless all checks pass.

## Bug Reports

When you find a bug, please create an issue with the following information:

1. **Environment Information**
   - OS (Linux/macOS/Windows) and version
   - Go version
   - Tor version (`tor --version`)
   - tornago version

2. **Reproduction Steps**
   - Minimal code example to reproduce the bug
   - Whether using launched Tor or existing Tor instance

3. **Expected and Actual Behavior**

4. **Error Messages or Stack Traces** (if any)
   - Include full `TornagoError` details with `ErrorKind`

## Security Considerations

When contributing to tornago, please keep in mind:

1. **Minimize attack surface**: This library intentionally keeps a minimal feature set
2. **No convenience features that could be easily misused**
3. **Proper authentication handling**: Always use proper Tor control authentication
4. **Input validation**: Validate all user-provided configuration
5. **Report security issues privately**: Email maintainers for security vulnerabilities

## Contributing Outside of Coding

The following activities are also greatly welcomed:

### Activities that Boost Motivation

- **Give a GitHub Star**: Show your interest in the project
- **Promote the Project**: Introduce it in blogs, social media, study groups, etc. (for legitimate security research and privacy protection use cases)
- **Become a GitHub Sponsor**: Support available at [https://github.com/sponsors/nao1215](https://github.com/sponsors/nao1215)

### Other Ways to Contribute

- **Documentation Improvements**: Fix typos, improve clarity of explanations
- **Translations**: Translate documentation to new languages
- **Add Examples**: Provide practical sample code for legitimate use cases
- **Feature Suggestions**: Share new feature ideas in issues (with security implications considered)

## Community

### Questions and Reports

- **GitHub Issues**: Bug reports and feature suggestions
- **Security Issues**: Report privately to maintainers

## License

Contributions to this project are considered to be released under the project's license (MIT License).

---

Thank you again for considering contributing! We sincerely look forward to your participation in building a useful tool for privacy protection and security research.
