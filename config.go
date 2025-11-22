package tornago

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

const (
	defaultTorBinary      = "tor"
	defaultSocksAddr      = ":0"
	defaultControlAddr    = ":0"
	defaultStartupTimeout = 30 * time.Second

	defaultDialTimeout    = 30 * time.Second
	defaultRequestTimeout = time.Minute

	defaultRetryAttempts = 3
	defaultRetryDelay    = 200 * time.Millisecond
	defaultRetryMaxDelay = 5 * time.Second
)

// TorLaunchConfig controls how the Tor daemon is started by Tornago. It is immutable
// after construction via NewTorLaunchConfig.
type TorLaunchConfig struct {
	// torBinary is the tor executable path chosen at construction time.
	torBinary string
	// socksAddr is the address for Tor's SocksPort; ":0" lets Tor pick a free port.
	socksAddr string
	// controlAddr is the address for Tor's ControlPort; ":0" lets Tor pick a free port.
	controlAddr string
	// dataDir points to the Tor DataDirectory when explicitly provided.
	dataDir string
	// torConfigFile optionally specifies a torrc file passed with "-f".
	torConfigFile string
	// logReporter optionally receives Tor log output during startup errors.
	logReporter func(string)
	// extraArgs are additional CLI arguments passed to tor.
	extraArgs []string
	// startupTimeout bounds how long Tornago waits for tor to become ready.
	startupTimeout time.Duration
	// logger provides structured logging for Tor daemon operations.
	logger Logger
}

// TorLaunchOption customizes TorLaunchConfig creation.
type TorLaunchOption func(*TorLaunchConfig)

// NewTorLaunchConfig returns a validated, immutable launch config.
func NewTorLaunchConfig(opts ...TorLaunchOption) (TorLaunchConfig, error) {
	cfg := TorLaunchConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return normalizeTorLaunchConfig(cfg)
}

// TorBinary is the tor executable path; defaults to LookPath("tor") when empty.
func (c TorLaunchConfig) TorBinary() string { return c.torBinary }

// SocksAddr is the address for Tor's SocksPort; ":0" lets Tor pick a free port.
func (c TorLaunchConfig) SocksAddr() string { return c.socksAddr }

// ControlAddr is the address for Tor's ControlPort; ":0" lets Tor pick a free port.
func (c TorLaunchConfig) ControlAddr() string { return c.controlAddr }

// DataDir is the Tor DataDirectory path when explicitly configured.
func (c TorLaunchConfig) DataDir() string { return c.dataDir }

// LogReporter returns the callback registered for Tor log output.
func (c TorLaunchConfig) LogReporter() func(string) { return c.logReporter }

// ExtraArgs are passed through to the tor process at launch.
func (c TorLaunchConfig) ExtraArgs() []string {
	if len(c.extraArgs) == 0 {
		return nil
	}
	out := make([]string, len(c.extraArgs))
	copy(out, c.extraArgs)
	return out
}

// StartupTimeout bounds how long Tornago waits for tor to become ready.
func (c TorLaunchConfig) StartupTimeout() time.Duration { return c.startupTimeout }

// TorConfigFile is the optional tor configuration file path passed with "-f".
func (c TorLaunchConfig) TorConfigFile() string { return c.torConfigFile }

// Logger returns the structured logger for Tor daemon operations.
func (c TorLaunchConfig) Logger() Logger { return c.logger }

// WithTorBinary sets the tor executable path.
func WithTorBinary(path string) TorLaunchOption {
	return func(cfg *TorLaunchConfig) {
		cfg.torBinary = path
	}
}

// WithTorSocksAddr sets the SocksPort listen address.
func WithTorSocksAddr(addr string) TorLaunchOption {
	return func(cfg *TorLaunchConfig) {
		cfg.socksAddr = addr
	}
}

// WithTorControlAddr sets the ControlPort listen address.
func WithTorControlAddr(addr string) TorLaunchOption {
	return func(cfg *TorLaunchConfig) {
		cfg.controlAddr = addr
	}
}

// WithTorDataDir forces Tor to use the provided DataDirectory path.
func WithTorDataDir(path string) TorLaunchOption {
	cleaned := filepath.Clean(path)
	return func(cfg *TorLaunchConfig) {
		cfg.dataDir = cleaned
	}
}

// WithTorConfigFile sets the torrc path passed to tor via "-f".
func WithTorConfigFile(path string) TorLaunchOption {
	cleaned := filepath.Clean(path)
	return func(cfg *TorLaunchConfig) {
		cfg.torConfigFile = cleaned
	}
}

// WithTorLogReporter registers a callback to receive Tor startup logs.
func WithTorLogReporter(fn func(string)) TorLaunchOption {
	return func(cfg *TorLaunchConfig) {
		cfg.logReporter = fn
	}
}

// WithTorExtraArgs appends additional CLI args passed to tor.
func WithTorExtraArgs(args ...string) TorLaunchOption {
	// Defensive copy so callers cannot mutate after creation.
	argsCopy := append([]string(nil), args...)
	return func(cfg *TorLaunchConfig) {
		cfg.extraArgs = append([]string(nil), argsCopy...)
	}
}

// WithTorStartupTimeout sets how long Tornago waits for tor to start.
func WithTorStartupTimeout(timeout time.Duration) TorLaunchOption {
	return func(cfg *TorLaunchConfig) {
		cfg.startupTimeout = timeout
	}
}

// WithTorLogger sets the structured logger for Tor daemon operations.
func WithTorLogger(logger Logger) TorLaunchOption {
	return func(cfg *TorLaunchConfig) {
		cfg.logger = logger
	}
}

// ServerConfig represents addresses of an existing Tor instance. It is immutable
// after construction via NewServerConfig.
type ServerConfig struct {
	// socksAddr is the address of an already running Tor SocksPort.
	socksAddr string
	// controlAddr is the address of an already running Tor ControlPort.
	controlAddr string
}

// ServerOption customizes ServerConfig creation.
type ServerOption func(*ServerConfig)

// NewServerConfig returns a validated, immutable server config.
func NewServerConfig(opts ...ServerOption) (ServerConfig, error) {
	cfg := ServerConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return normalizeServerConfig(cfg)
}

// SocksAddr is the address of an already running Tor SocksPort.
func (c ServerConfig) SocksAddr() string { return c.socksAddr }

// ControlAddr is the address of an already running Tor ControlPort.
func (c ServerConfig) ControlAddr() string { return c.controlAddr }

// WithServerSocksAddr sets the SocksPort address.
// WithServerSocksAddr sets the SocksPort address on ServerConfig.
func WithServerSocksAddr(addr string) ServerOption {
	return func(cfg *ServerConfig) {
		cfg.socksAddr = addr
	}
}

// WithServerControlAddr sets the ControlPort address.
// WithServerControlAddr sets the ControlPort address on ServerConfig.
func WithServerControlAddr(addr string) ServerOption {
	return func(cfg *ServerConfig) {
		cfg.controlAddr = addr
	}
}

// ControlAuth holds ControlPort authentication values. It is immutable after
// creation via the helper functions below.
type ControlAuth struct {
	// password is used for "SAFECOOKIE" or "HASHEDPASSWORD" auth methods.
	password string
	// cookiePath points to the tor control cookie for cookie-based auth.
	cookiePath string
	// cookieBytes stores raw cookie data when the file is inaccessible.
	cookieBytes []byte
}

// ControlAuthFromPassword builds ControlAuth for password-based auth.
// ControlAuthFromPassword constructs ControlAuth for password-based auth.
func ControlAuthFromPassword(password string) ControlAuth {
	return ControlAuth{password: password}
}

// ControlAuthFromCookie builds ControlAuth for cookie-based auth.
// ControlAuthFromCookie constructs ControlAuth for cookie-based auth.
func ControlAuthFromCookie(path string) ControlAuth {
	return ControlAuth{cookiePath: path}
}

// ControlAuthFromCookieBytes constructs ControlAuth from raw cookie data.
func ControlAuthFromCookieBytes(data []byte) ControlAuth {
	return ControlAuth{cookieBytes: append([]byte(nil), data...)}
}

// Password returns the configured control password.
func (a ControlAuth) Password() string { return a.password }

// CookiePath returns the configured control cookie path.
func (a ControlAuth) CookiePath() string { return a.cookiePath }

// CookieBytes returns the raw cookie data if configured.
func (a ControlAuth) CookieBytes() []byte {
	if len(a.cookieBytes) == 0 {
		return nil
	}
	cp := make([]byte, len(a.cookieBytes))
	copy(cp, a.cookieBytes)
	return cp
}

// ClientConfig bundles all knobs for creating a Client. It is immutable after
// construction via NewClientConfig.
type ClientConfig struct {
	// socksAddr is the target SocksPort address used for outbound traffic.
	socksAddr string
	// controlAddr is the ControlPort address used for optional control commands.
	controlAddr string
	// controlAuth carries credentials for the ControlPort.
	controlAuth ControlAuth
	// dialTimeout is the timeout for establishing TCP connections via SOCKS5.
	dialTimeout time.Duration
	// requestTimeout sets the overall timeout for HTTP requests.
	requestTimeout time.Duration

	// retryAttempts is the maximum number of retries when retryOnError returns true.
	retryAttempts uint
	// retryDelay is the initial backoff delay used by retry-go.
	retryDelay time.Duration
	// retryMaxDelay caps backoff delay used by retry-go.
	retryMaxDelay time.Duration
	// retryOnError decides whether an error should trigger a retry.
	retryOnError func(error) bool
	// metrics is an optional collector for request statistics.
	metrics *MetricsCollector
	// rateLimiter is an optional rate limiter for requests.
	rateLimiter *RateLimiter
	// logger is an optional structured logger for debugging and monitoring.
	logger Logger
}

// ClientOption customizes ClientConfig creation.
type ClientOption func(*ClientConfig)

// NewClientConfig returns a validated, immutable client config.
func NewClientConfig(opts ...ClientOption) (ClientConfig, error) {
	cfg := ClientConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return normalizeClientConfig(cfg)
}

// SocksAddr is the target SocksPort address used for outbound traffic.
func (c ClientConfig) SocksAddr() string { return c.socksAddr }

// ControlAddr is the ControlPort address used for optional control commands.
func (c ClientConfig) ControlAddr() string { return c.controlAddr }

// ControlAuth carries credentials for the ControlPort.
func (c ClientConfig) ControlAuth() ControlAuth { return c.controlAuth }

// DialTimeout is the timeout for establishing TCP connections via SOCKS5.
func (c ClientConfig) DialTimeout() time.Duration { return c.dialTimeout }

// RequestTimeout sets the overall timeout for HTTP requests.
func (c ClientConfig) RequestTimeout() time.Duration { return c.requestTimeout }

// RetryAttempts is the maximum number of retries when RetryOnError returns true.
func (c ClientConfig) RetryAttempts() uint { return c.retryAttempts }

// RetryDelay is the initial backoff delay used by retry-go.
func (c ClientConfig) RetryDelay() time.Duration { return c.retryDelay }

// RetryMaxDelay caps backoff delay used by retry-go.
func (c ClientConfig) RetryMaxDelay() time.Duration { return c.retryMaxDelay }

// RetryOnError decides whether an error should trigger a retry.
func (c ClientConfig) RetryOnError() func(error) bool { return c.retryOnError }

// Metrics returns the optional metrics collector.
func (c ClientConfig) Metrics() *MetricsCollector { return c.metrics }

// Logger returns the optional logger instance.
func (c ClientConfig) Logger() Logger { return c.logger }

// RateLimiter returns the optional rate limiter.
func (c ClientConfig) RateLimiter() *RateLimiter { return c.rateLimiter }

// WithClientSocksAddr sets the SocksPort address for the client.
func WithClientSocksAddr(addr string) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.socksAddr = addr
	}
}

// WithClientControlAddr sets the ControlPort address for the client.
func WithClientControlAddr(addr string) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.controlAddr = addr
	}
}

// WithClientControlPassword sets password-based ControlPort authentication.
func WithClientControlPassword(password string) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.controlAuth.password = password
	}
}

// WithClientControlCookie sets cookie-based ControlPort authentication.
func WithClientControlCookie(path string) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.controlAuth.cookiePath = path
	}
}

// WithClientControlCookieBytes sets cookie-based ControlPort authentication using raw cookie bytes.
func WithClientControlCookieBytes(data []byte) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.controlAuth.cookieBytes = append([]byte(nil), data...)
	}
}

// WithClientDialTimeout sets the timeout for dialing via SOCKS5.
func WithClientDialTimeout(timeout time.Duration) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.dialTimeout = timeout
	}
}

// WithClientRequestTimeout sets the overall HTTP request timeout.
func WithClientRequestTimeout(timeout time.Duration) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.requestTimeout = timeout
	}
}

// WithRetryAttempts sets the maximum number of retries.
func WithRetryAttempts(attempts uint) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.retryAttempts = attempts
	}
}

// WithRetryDelay sets the initial backoff delay.
func WithRetryDelay(delay time.Duration) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.retryDelay = delay
	}
}

// WithRetryMaxDelay caps the backoff delay.
func WithRetryMaxDelay(delay time.Duration) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.retryMaxDelay = delay
	}
}

// WithRetryOnError registers a predicate to decide retry eligibility.
func WithRetryOnError(fn func(error) bool) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.retryOnError = fn
	}
}

// WithClientMetrics sets the metrics collector for the client.
func WithClientMetrics(m *MetricsCollector) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.metrics = m
	}
}

// WithClientLogger sets a structured logger for debugging and monitoring.
//
// Example with slog:
//
//	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
//	    Level: slog.LevelDebug,
//	}))
//	cfg, _ := tornago.NewClientConfig(
//	    tornago.WithClientSocksAddr("127.0.0.1:9050"),
//	    tornago.WithClientLogger(tornago.NewSlogAdapter(logger)),
//	)
func WithClientLogger(logger Logger) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.logger = logger
	}
}

// WithClientRateLimiter sets the rate limiter for the client.
func WithClientRateLimiter(r *RateLimiter) ClientOption {
	return func(cfg *ClientConfig) {
		cfg.rateLimiter = r
	}
}

// normalizeTorLaunchConfig applies defaults and validates the given config.
func normalizeTorLaunchConfig(cfg TorLaunchConfig) (TorLaunchConfig, error) {
	cfg = applyTorLaunchDefaults(cfg)
	if err := validateTorLaunchConfig(cfg); err != nil {
		return TorLaunchConfig{}, err
	}
	return cfg, nil
}

// applyTorLaunchDefaults fills empty TorLaunchConfig fields with defaults.
func applyTorLaunchDefaults(cfg TorLaunchConfig) TorLaunchConfig {
	if cfg.torBinary == "" {
		cfg.torBinary = defaultTorBinary
	}
	if cfg.socksAddr == "" {
		cfg.socksAddr = defaultSocksAddr
	}
	if cfg.controlAddr == "" {
		cfg.controlAddr = defaultControlAddr
	}
	if cfg.startupTimeout == 0 {
		cfg.startupTimeout = defaultStartupTimeout
	}
	if cfg.logger == nil {
		cfg.logger = noopLogger{}
	}
	return cfg
}

// validateTorLaunchConfig ensures the launch config has required values.
func validateTorLaunchConfig(cfg TorLaunchConfig) error {
	switch {
	case cfg.torBinary == "":
		return newError(ErrInvalidConfig, "validateTorLaunchConfig",
			"TorBinary is empty. Use WithTorBinary(\"tor\") or ensure tor is in PATH", nil)
	case cfg.socksAddr == "":
		return newError(ErrInvalidConfig, "validateTorLaunchConfig",
			"SocksAddr is empty. Use WithTorSocksAddr(\":9050\") or WithTorSocksAddr(\":0\") for dynamic port", nil)
	case cfg.controlAddr == "":
		return newError(ErrInvalidConfig, "validateTorLaunchConfig",
			"ControlAddr is empty. Use WithTorControlAddr(\":9051\") or WithTorControlAddr(\":0\") for dynamic port", nil)
	case cfg.startupTimeout <= 0:
		return newError(ErrInvalidConfig, "validateTorLaunchConfig",
			fmt.Sprintf("StartupTimeout must be positive, got %v. Use WithTorStartupTimeout(30*time.Second)", cfg.startupTimeout), nil)
	}
	return nil
}

// normalizeServerConfig applies defaults and validates the given config.
func normalizeServerConfig(cfg ServerConfig) (ServerConfig, error) {
	cfg = applyServerDefaults(cfg)
	if err := validateServerConfig(cfg); err != nil {
		return ServerConfig{}, err
	}
	return cfg, nil
}

// applyServerDefaults fills empty ServerConfig fields with defaults.
func applyServerDefaults(cfg ServerConfig) ServerConfig {
	if cfg.socksAddr == "" {
		cfg.socksAddr = defaultSocksAddr
	}
	if cfg.controlAddr == "" {
		cfg.controlAddr = defaultControlAddr
	}
	return cfg
}

// validateServerConfig ensures ServerConfig has required values.
func validateServerConfig(cfg ServerConfig) error {
	switch {
	case cfg.socksAddr == "":
		return newError(ErrInvalidConfig, "validateServerConfig",
			"SocksAddr is empty. Use WithServerSocksAddr(\"127.0.0.1:9050\") to specify Tor SOCKS address", nil)
	case cfg.controlAddr == "":
		return newError(ErrInvalidConfig, "validateServerConfig",
			"ControlAddr is empty. Use WithServerControlAddr(\"127.0.0.1:9051\") to specify Tor control port", nil)
	}
	return nil
}

// normalizeClientConfig applies defaults and validates the given config.
func normalizeClientConfig(cfg ClientConfig) (ClientConfig, error) {
	cfg = applyClientDefaults(cfg)
	if err := validateClientConfig(cfg); err != nil {
		return ClientConfig{}, err
	}
	return cfg, nil
}

// applyClientDefaults fills empty ClientConfig fields with defaults.
func applyClientDefaults(cfg ClientConfig) ClientConfig {
	if cfg.socksAddr == "" {
		cfg.socksAddr = defaultSocksAddr
	}
	if cfg.dialTimeout == 0 {
		cfg.dialTimeout = defaultDialTimeout
	}
	if cfg.requestTimeout == 0 {
		cfg.requestTimeout = defaultRequestTimeout
	}
	if cfg.retryAttempts == 0 {
		cfg.retryAttempts = defaultRetryAttempts
	}
	if cfg.retryDelay == 0 {
		cfg.retryDelay = defaultRetryDelay
	}
	if cfg.retryMaxDelay == 0 {
		cfg.retryMaxDelay = defaultRetryMaxDelay
	}
	if cfg.retryOnError == nil {
		cfg.retryOnError = defaultRetryOnError
	}
	if cfg.logger == nil {
		cfg.logger = noopLogger{}
	}
	return cfg
}

// validateClientConfig ensures ClientConfig has required values and constraints.
func validateClientConfig(cfg ClientConfig) error {
	switch {
	case cfg.socksAddr == "":
		return newError(ErrInvalidConfig, "validateClientConfig",
			"SocksAddr is empty. Use WithClientSocksAddr(\"127.0.0.1:9050\") or ensure Tor is running on default port", nil)
	case cfg.dialTimeout <= 0:
		return newError(ErrInvalidConfig, "validateClientConfig",
			fmt.Sprintf("DialTimeout must be positive, got %v. Use WithClientDialTimeout(30*time.Second)", cfg.dialTimeout), nil)
	case cfg.requestTimeout <= 0:
		return newError(ErrInvalidConfig, "validateClientConfig",
			fmt.Sprintf("RequestTimeout must be positive, got %v. Use WithClientRequestTimeout(60*time.Second)", cfg.requestTimeout), nil)
	case cfg.retryDelay <= 0:
		return newError(ErrInvalidConfig, "validateClientConfig",
			fmt.Sprintf("RetryDelay must be positive, got %v. Use WithClientRetryDelay(200*time.Millisecond)", cfg.retryDelay), nil)
	case cfg.retryMaxDelay < cfg.retryDelay:
		return newError(ErrInvalidConfig, "validateClientConfig",
			fmt.Sprintf("RetryMaxDelay (%v) must be >= RetryDelay (%v). Adjust with WithRetryMaxDelay()", cfg.retryMaxDelay, cfg.retryDelay), nil)
	case cfg.retryOnError == nil:
		return newError(ErrInvalidConfig, "validateClientConfig",
			"RetryOnError must not be nil. Use WithRetryOnError() or accept defaults", nil)
	}
	return nil
}

// defaultRetryOnError skips retries when the caller canceled or timed out the request.
var defaultRetryOnError = func(err error) bool {
	// Avoid retrying when the caller explicitly canceled or timed out.
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}
