# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.1] - 2025-11-23

### Added
- Support for Windows platform
- Support for BSD family (FreeBSD, OpenBSD, NetBSD, DragonFly BSD)
- GitHub Actions workflows for Windows and BSD variants
- `make lint` target for running golangci-lint
- Automated GitHub release workflow triggered by version tags
- Multilingual documentation (Japanese, Spanish, French, Korean, Russian, Chinese)

### Changed
- Enhanced CI/CD coverage across 7 platforms (Linux, macOS, Windows, 4 BSD variants)
- Improved test robustness: relaxed timeout assertions for CI environment tolerance
- Enhanced test cleanup: added deferred cleanup in `TestTorProcessCrashRecovery` to prevent process leaks
- Improved test transparency: added logging for negative memory growth in stability tests

### Fixed
- Fixed incorrect API reference in `circuit_rotation` example documentation (api.ipify.org)
- Corrected timeout boundary tests to be more resilient under CI load (100ms → 500ms tolerance)

## [0.3.0] - 2025-01-XX

### Added
- Comprehensive documentation with "How Tor Works" section using Mermaid diagrams
- 11 working examples demonstrating various use cases:
  - `simple_client` - Basic HTTP requests through Tor
  - `onion_client` - Accessing .onion sites
  - `onion_server` - Creating Hidden Services
  - `existing_tor` - Connecting to system Tor daemon
  - `circuit_rotation` - Rotating circuits to change exit IP
  - `circuit_management` - Advanced circuit management
  - `error_handling` - Proper error handling patterns
  - `metrics_ratelimit` - Metrics collection and rate limiting
  - `persistent_onion` - Hidden Service with persistent key
  - `observability` - Structured logging, metrics, and health checks
  - `security` - Security verification features
- Logger interface for structured logging
- Health check functionality (`Check()`, `CheckDNSLeak()`, `CheckTorDaemon()`)
- Security verification (`VerifyTorConnection()`)
- Enhanced README with visual diagrams and detailed examples

### Changed
- Improved test performance and reliability
- Enhanced documentation comments throughout codebase

### Security
- Added DNS leak detection
- Added Tor connection verification via check.torproject.org

## [0.2.0] - 2025-01-XX

### Added
- `net.Listener` compatible interface for Hidden Services
- `net.Dialer` compatible interface for Tor connections
- Enhanced `ControlClient` with additional commands:
  - `GETCONF` / `SETCONF` for Tor configuration management
  - `GETINFO circuit-status` for circuit information
  - `GETINFO stream-status` for stream monitoring
  - `MAPADDRESS` for address mapping
- Enhanced Hidden Service features:
  - Persistent private key save/load (`SavePrivateKey`, `LoadPrivateKey`)
  - Convenient port mapping helpers (`WithHiddenServiceSamePort`, `WithHiddenServiceHTTP`, `WithHiddenServiceHTTPS`)
  - Hidden Service status monitoring (`GetHiddenServiceStatus`)
- Metrics collection interface (`MetricsCollector`)
- Rate limiting with token bucket algorithm (`RateLimiter`)
- Circuit management (`CircuitManager`)

### Changed
- Enhanced client with better retry logic and timeout handling
- Improved hidden service examples with more detailed documentation

## [0.1.0] - 2025-01-XX

### Added
- Initial release of tornago library
- Core Tor client functionality:
  - HTTP/TCP traffic routing through Tor's SOCKS5 proxy
  - Automatic retries with exponential backoff
  - Connection timeout management
- Tor daemon management:
  - Launch and manage Tor processes programmatically
  - Dynamic port allocation
  - Graceful shutdown and cleanup
- ControlPort client implementation:
  - Cookie and password authentication
  - Basic commands: AUTHENTICATE, GETINFO, SIGNAL NEWNYM, ADD_ONION, DEL_ONION
  - Thread-safe command execution
- Hidden Service (onion service) support:
  - ED25519-V3 onion address creation
  - Port mapping configuration
  - Service lifecycle management
- Configuration via functional options pattern
- Structured error handling with `TornagoError`
- Support for macOS and Linux platforms
- Comprehensive test suite with TestServer infrastructure
- Zero external Go dependencies (standard library only)

### Changed
- Removed initial Windows support (re-added in later version)

[Unreleased]: https://github.com/nao1215/tornago/compare/v0.3.1...HEAD
[0.3.1]: https://github.com/nao1215/tornago/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/nao1215/tornago/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/nao1215/tornago/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/nao1215/tornago/releases/tag/v0.1.0
