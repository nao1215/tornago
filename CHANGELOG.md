# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- `MetricsCollector.MinLatency` keeps a request measured as 0 (a fast request on a coarse clock) as the minimum. It used to treat 0 as "nothing recorded yet" and replace it with the next latency.
- `ErrorsByKind` counts a `TornagoError` that sits inside an `errors.Join`, as `errors.As` finds it.
- `RelayPerformanceTracker.BlockedRelays` and `Stats` no longer delete expired blocks from the tracker's map under a read lock, which let two concurrent callers write the map together (a data race, and a `concurrent map writes` crash). At the instant a block expires they also agree with `IsBlocked`, which already said the relay was free while they still listed it.
- `NewSuccessRate(NaN)` returns 0 instead of NaN, so `RelayThreshold.WithMinSuccessRate(NaN)` no longer builds a threshold that no rate is below.
- `RelayStats.AverageLatency` is exact when the sum of the latencies in the window does not fit in a `time.Duration`. It used to wrap around and could report a very slow relay as fast.

### Removed

- `As`, an exported helper that only followed `Unwrap() error` and missed errors joined with `errors.Join`. Use `errors.As`, which `TornagoError` supports.
- `TestServer` and `StartTestServer` are no longer part of the public API. They exist for this package's integration tests, and they made every importer link the `testing` package. pkg.go.dev lists no importer outside the maintainer's own repositories, and none of them uses either.

### Changed

- CI scans the repository with gosec and Trivy on every pull request (Trivy covers dependency vulnerabilities, committed secrets and misconfigurations, and also runs daily). Workflow actions are recorded in `.github/workflows/actions.lock`, which CI verifies against the pinned commits.
- CI scans the commits of pull requests and of pushes to main for committed secrets with gitleaks (Dependabot pull requests are skipped: they only change version numbers, and their read-only token cannot list the commits) and moves govulncheck to v1.10.0. A new push to a pull request cancels the run it supersedes, and the unit-test matrix no longer repeats the Linux and stable Go run that the Coverage workflow already makes. The Go minimum stays at 1.24.
- Property, metamorphic and differential tests (with the standard library's testing/quick, so the module still has no dependencies) cover the SOCKS5 request, the status-line parsers, the ADD_ONION command and the metrics and relay statistics; a nightly workflow draws 20,000 inputs per property. Two fuzz targets cover the SOCKS5 CONNECT reply and request, and gremlins mutation testing runs weekly and on the lines a pull request changes (75.19% on the first run). See doc/testing.md.

## [0.4.3] - 2026-09-28

### Fixed

- A ControlPort command whose key, value, password or address contains CR or LF is now refused with `ErrInvalidConfig` before anything is sent. Until now the line break ended the command early and Tor ran the rest as a second command, so a value such as `"x\r\nSIGNAL SHUTDOWN\r\nx"` passed to `SetConf` shut Tor down.
- A data reply line that begins with a dot now comes back without the extra dot Tor adds in front of it (control-spec section 2.3); `..hidden` used to be returned as is instead of `.hidden`.
- An asynchronous event (a `650` line) arriving in the middle of a reply is skipped as the code comment always said, instead of failing the command, and a `4xx` reply now fails the command instead of being ignored until the timeout.

## [0.4.2] - 2026-09-12

### Changed

- The supported Go floor is 1.24 instead of 1.25. Nothing in the tree needs a newer Go: 1.24 is where `testing.T.Context` stops existing, which is the only thing 1.23 cannot compile, so that is the real minimum and one more Go release can now use this library. The unit-test matrix runs that floor and the newest release.
- The BSD virtual machines run `cross-platform-actions/action` v1.5.0.

### Removed

- The Go Report Card badge, in the English README and all six translations. The service is retired and its badge now answers `go report: retired`, which is a line of README saying nothing about this library.

## [0.4.1] - 2026-08-25

### Added
- Added OnionScan to the README's related tools.

### Changed
- Updated GitHub Actions dependencies for checkout, Go setup, cross-platform jobs,
  coverage reporting, and GitHub Releases.
- Improved CI resilience for Windows Tor installation and FreeBSD package setup.

### Fixed
- Made SOCKS health checks deterministic by probing the local proxy and applying
  context deadlines to handshake I/O.
- Stabilized Tor integration tests against transient network errors, slow bootstrap,
  process cleanup races, and local port conflicts.
- Allocated hidden service ports atomically to avoid close-and-rebind races.
- Escaped request-derived values in the onion server example and resolved lint and
  security findings across the codebase.

## [0.4.0] - 2025-11-27

### Added
- **Slow Relay Avoidance**: Automatic detection and avoidance of slow Tor relays
  - `WithSlowRelayAvoidance()` option for easy enablement with sensible defaults
  - Configurable thresholds via functional options:
    - `SlowRelayMaxLatency()` - Maximum acceptable latency (default: 5s)
    - `SlowRelayMinSuccessRate()` - Minimum success rate (default: 80%)
    - `SlowRelayBlockDuration()` - How long to block slow relays (default: 30min)
    - `SlowRelayMinSamples()` - Samples needed before evaluation (default: 3)
    - `SlowRelayMonitorInterval()` - Background check interval (default: 30s)
  - `RelayPerformanceStats()` method to query performance statistics
  - Automatic circuit rotation when slow relays are detected
  - Auto-exclude blocked relays from Tor's circuit building via `ExcludeNodes`
- New example: `examples/slow_relay_avoidance/` demonstrating the feature
- Documentation with Mermaid sequence diagram explaining how slow relay avoidance works

### Changed
- Extended Tor bootstrap timeout in `TestTorProcessCrashRecovery` (60s → 120s) for CI stability

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

[Unreleased]: https://github.com/nao1215/tornago/compare/v0.4.3...HEAD
[0.4.3]: https://github.com/nao1215/tornago/compare/v0.4.2...v0.4.3
[0.4.2]: https://github.com/nao1215/tornago/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/nao1215/tornago/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/nao1215/tornago/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/nao1215/tornago/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/nao1215/tornago/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/nao1215/tornago/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/nao1215/tornago/releases/tag/v0.1.0
