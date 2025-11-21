package tornago

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// HiddenServiceConfig describes the desired onion service to create via Tor.
type HiddenServiceConfig struct {
	// keyType defines the onion key type requested via ADD_ONION (e.g. ED25519-V3).
	keyType string
	// privateKey holds an optional Tor-formatted private key blob for reuse.
	privateKey string
	// targetPort maps virtual onion ports to local target ports.
	targetPort map[int]int
	// clientAuth stores optional per-client authorization entries.
	clientAuth []HiddenServiceAuth
}

// HiddenServiceOption customizes HiddenServiceConfig creation.
type HiddenServiceOption func(*HiddenServiceConfig)

// NewHiddenServiceConfig returns a validated, immutable configuration.
func NewHiddenServiceConfig(opts ...HiddenServiceOption) (HiddenServiceConfig, error) {
	cfg := HiddenServiceConfig{
		targetPort: make(map[int]int),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return normalizeHiddenServiceConfig(cfg)
}

// KeyType returns the key type (e.g. "ED25519-V3").
func (c HiddenServiceConfig) KeyType() string { return c.keyType }

// PrivateKey returns the optional private key blob.
func (c HiddenServiceConfig) PrivateKey() string { return c.privateKey }

// Ports returns a copy of the configured virtual -> target port mapping.
func (c HiddenServiceConfig) Ports() map[int]int {
	cp := make(map[int]int, len(c.targetPort))
	for k, v := range c.targetPort {
		cp[k] = v
	}
	return cp
}

// ClientAuth returns a copy of the configured client authorization entries.
func (c HiddenServiceConfig) ClientAuth() []HiddenServiceAuth {
	cp := make([]HiddenServiceAuth, len(c.clientAuth))
	copy(cp, c.clientAuth)
	return cp
}

// WithHiddenServiceKeyType sets the key type (default: "ED25519-V3").
func WithHiddenServiceKeyType(keyType string) HiddenServiceOption {
	return func(cfg *HiddenServiceConfig) {
		cfg.keyType = keyType
	}
}

// WithHiddenServicePrivateKey uses an existing private key blob.
func WithHiddenServicePrivateKey(privateKey string) HiddenServiceOption {
	return func(cfg *HiddenServiceConfig) {
		cfg.privateKey = privateKey
	}
}

// WithHiddenServicePort maps a virtual port to a local target port.
func WithHiddenServicePort(virtualPort, targetPort int) HiddenServiceOption {
	return func(cfg *HiddenServiceConfig) {
		if cfg.targetPort == nil {
			cfg.targetPort = make(map[int]int)
		}
		cfg.targetPort[virtualPort] = targetPort
	}
}

// WithHiddenServicePorts sets the entire virtual -> target port mapping.
func WithHiddenServicePorts(ports map[int]int) HiddenServiceOption {
	return func(cfg *HiddenServiceConfig) {
		if cfg.targetPort == nil {
			cfg.targetPort = make(map[int]int, len(ports))
		}
		for k, v := range ports {
			cfg.targetPort[k] = v
		}
	}
}

// WithHiddenServiceClientAuth appends client authorization entries.
func WithHiddenServiceClientAuth(auth ...HiddenServiceAuth) HiddenServiceOption {
	return func(cfg *HiddenServiceConfig) {
		cfg.clientAuth = append(cfg.clientAuth, auth...)
	}
}

// WithHiddenServiceSamePort maps a port to itself (virtualPort == targetPort).
// This is a convenience for common cases where you don't need port translation.
func WithHiddenServiceSamePort(port int) HiddenServiceOption {
	return WithHiddenServicePort(port, port)
}

// WithHiddenServiceHTTP maps port 80 to the specified local port.
// This is a convenience for hosting HTTP services.
func WithHiddenServiceHTTP(localPort int) HiddenServiceOption {
	return WithHiddenServicePort(80, localPort)
}

// WithHiddenServiceHTTPS maps port 443 to the specified local port.
// This is a convenience for hosting HTTPS services.
func WithHiddenServiceHTTPS(localPort int) HiddenServiceOption {
	return WithHiddenServicePort(443, localPort)
}

// HiddenServiceAuth describes Tor v3 client authorization information.
type HiddenServiceAuth struct {
	// clientName is the name assigned to this authorized client.
	clientName string
	// key is the base32-encoded client auth key.
	key string
}

// NewHiddenServiceAuth returns a client auth entry.
func NewHiddenServiceAuth(clientName, key string) HiddenServiceAuth {
	return HiddenServiceAuth{
		clientName: clientName,
		key:        key,
	}
}

// ClientName returns the configured auth client name.
func (a HiddenServiceAuth) ClientName() string { return a.clientName }

// Key returns the authorization key.
func (a HiddenServiceAuth) Key() string { return a.key }

// HiddenService represents a provisioned Hidden Service (also known as an onion service).
// A hidden service allows you to host a server that's accessible only through the Tor network,
// identified by a .onion address.
//
// Benefits of hidden services:
//   - No need for public IP address or DNS registration
//   - Server location and IP remain anonymous
//   - End-to-end encryption through Tor network
//   - Censorship resistance (difficult to block .onion addresses)
//
// Example usage:
//
//	// Create hidden service mapping port 80 to local port 8080
//	cfg, _ := tornago.NewHiddenServiceConfig(
//	    tornago.WithHiddenServicePort(80, 8080),
//	)
//	hs, _ := controlClient.CreateHiddenService(context.Background(), cfg)
//	defer hs.Remove(context.Background())
//
//	fmt.Printf("Your service is at: %s\n", hs.OnionAddress())
//	// Example output: "abc123xyz456.onion"
type HiddenService interface {
	// OnionAddress returns the .onion address where the service is accessible.
	OnionAddress() string
	// PrivateKey returns the private key in Tor's format for re-registering this service.
	PrivateKey() string
	// Ports returns the virtual port to local port mapping.
	Ports() map[int]int
	// ClientAuth returns the client authorization entries if configured.
	ClientAuth() []HiddenServiceAuth
	// Remove deletes this hidden service from Tor. The .onion address becomes inaccessible.
	Remove(ctx context.Context) error
	// SavePrivateKey saves the private key to a file for later reuse.
	SavePrivateKey(path string) error
}

type hiddenService struct {
	// control references the ControlClient used to manage this service.
	control *ControlClient
	// address is the .onion address assigned by Tor.
	address string
	// privateKey is the Tor-formatted private key for re-registration.
	privateKey string
	// ports keeps the virtual -> target port mapping.
	ports map[int]int
	// auth holds client authorization entries.
	auth []HiddenServiceAuth
}

// OnionAddress returns the .onion address.
func (h *hiddenService) OnionAddress() string { return h.address }

// PrivateKey returns the private key (type-prefixed) for re-registration.
func (h *hiddenService) PrivateKey() string { return h.privateKey }

// Ports returns the configured port mapping.
func (h *hiddenService) Ports() map[int]int {
	cp := make(map[int]int, len(h.ports))
	for k, v := range h.ports {
		cp[k] = v
	}
	return cp
}

// ClientAuth returns the configured client authorization entries.
func (h *hiddenService) ClientAuth() []HiddenServiceAuth {
	cp := make([]HiddenServiceAuth, len(h.auth))
	copy(cp, h.auth)
	return cp
}

// Remove deletes the Hidden Service via Tor's DEL_ONION command.
func (h *hiddenService) Remove(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := h.control.ensureAuthenticated(); err != nil {
		return err
	}
	// DEL_ONION expects service ID without .onion suffix
	serviceID := strings.TrimSuffix(h.address, ".onion")
	cmd := "DEL_ONION " + serviceID
	_, err := h.control.execCommand(ctx, cmd)
	if err != nil {
		return newError(ErrHiddenServiceFailed, opControlClient, "failed to remove hidden service", err)
	}
	return nil
}

// CreateHiddenService issues ADD_ONION and returns a HiddenService handle.
func (c *ControlClient) CreateHiddenService(ctx context.Context, cfg HiddenServiceConfig) (HiddenService, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	cfg, err := normalizeHiddenServiceConfig(cfg)
	if err != nil {
		return nil, err
	}

	cmd := buildAddOnionCommand(cfg)

	lines, err := c.execCommand(ctx, cmd)
	if err != nil {
		return nil, err
	}

	var serviceID string
	privateKey := cfg.PrivateKey()
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "ServiceID="):
			serviceID = strings.TrimPrefix(line, "ServiceID=")
		case strings.HasPrefix(line, "PrivateKey="):
			privateKey = strings.TrimPrefix(line, "PrivateKey=")
		}
	}

	if serviceID == "" {
		return nil, newError(ErrHiddenServiceFailed, opControlClient, "tor did not return ServiceID", nil)
	}

	return &hiddenService{
		control:    c,
		address:    serviceID + ".onion",
		privateKey: privateKey,
		ports:      cfg.Ports(),
		auth:       cfg.ClientAuth(),
	}, nil
}

// normalizeHiddenServiceConfig applies defaults and validates the configuration.
// It returns a normalized copy of the configuration or an error if validation fails.
func normalizeHiddenServiceConfig(cfg HiddenServiceConfig) (HiddenServiceConfig, error) {
	cfg = applyHiddenServiceDefaults(cfg)
	if err := validateHiddenServiceConfig(cfg); err != nil {
		return HiddenServiceConfig{}, err
	}
	cfg.targetPort = cfg.Ports()
	cfg.clientAuth = cfg.ClientAuth()
	return cfg, nil
}

// applyHiddenServiceDefaults sets default values for unset configuration fields.
// Currently sets keyType to "ED25519-V3" if not specified.
func applyHiddenServiceDefaults(cfg HiddenServiceConfig) HiddenServiceConfig {
	if cfg.keyType == "" {
		cfg.keyType = "ED25519-V3"
	}
	return cfg
}

// validateHiddenServiceConfig checks that all required fields are set and valid.
// Returns an error if keyType is empty, no ports are configured, ports are out of range,
// or client auth entries are incomplete.
func validateHiddenServiceConfig(cfg HiddenServiceConfig) error {
	if cfg.keyType == "" {
		return newError(ErrInvalidConfig, "validateHiddenServiceConfig", "KeyType is empty", nil)
	}
	if len(cfg.targetPort) == 0 {
		return newError(ErrInvalidConfig, "validateHiddenServiceConfig", "TargetPorts must not be empty", nil)
	}
	for virt, tgt := range cfg.targetPort {
		if virt <= 0 || virt > 65535 {
			return newError(ErrInvalidConfig, "validateHiddenServiceConfig", fmt.Sprintf("virtual port %d out of range", virt), nil)
		}
		if tgt <= 0 || tgt > 65535 {
			return newError(ErrInvalidConfig, "validateHiddenServiceConfig", fmt.Sprintf("target port %d out of range", tgt), nil)
		}
	}
	for _, auth := range cfg.clientAuth {
		if auth.clientName == "" {
			return newError(ErrInvalidConfig, "validateHiddenServiceConfig", "ClientAuth client name is empty", nil)
		}
		if auth.key == "" {
			return newError(ErrInvalidConfig, "validateHiddenServiceConfig", "ClientAuth key is empty", nil)
		}
	}
	return nil
}

// HiddenServiceStatus represents the status of a hidden service.
type HiddenServiceStatus struct {
	// ServiceID is the onion address without .onion suffix.
	ServiceID string
	// Ports lists the configured port mappings.
	Ports []string
}

// GetHiddenServiceStatus retrieves information about all active hidden services.
// This is useful for monitoring and debugging hidden service configurations.
func (c *ControlClient) GetHiddenServiceStatus(ctx context.Context) ([]HiddenServiceStatus, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}
	lines, err := c.execCommand(ctx, "GETINFO onions/current")
	if err != nil {
		// If no hidden services exist, Tor may return an error.
		// We treat this as "no services" rather than an error.
		return []HiddenServiceStatus{}, nil //nolint:nilerr // expected behavior when no services exist
	}

	var services []HiddenServiceStatus
	for _, line := range lines {
		if strings.HasPrefix(line, "onions/current=") {
			ids := strings.TrimPrefix(line, "onions/current=")
			if ids == "" {
				continue
			}
			for _, id := range strings.Split(ids, "\n") {
				id = strings.TrimSpace(id)
				if id != "" {
					services = append(services, HiddenServiceStatus{ServiceID: id})
				}
			}
		}
	}
	return services, nil
}

// SavePrivateKey saves the hidden service's private key to a file.
// The key can later be loaded with LoadPrivateKey to recreate the same .onion address.
// The file is created with 0600 permissions for security.
//
// Example:
//
//	hs, _ := ctrl.CreateHiddenService(ctx, cfg)
//	if err := hs.SavePrivateKey("/path/to/key"); err != nil {
//	    log.Fatal(err)
//	}
func (h *hiddenService) SavePrivateKey(path string) error {
	if h.privateKey == "" {
		return newError(ErrInvalidConfig, "SavePrivateKey", "private key is empty", nil)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return newError(ErrIO, "SavePrivateKey", "failed to create directory", err)
	}
	// #nosec G306 -- 0600 is secure for private key files
	if err := os.WriteFile(path, []byte(h.privateKey), 0600); err != nil {
		return newError(ErrIO, "SavePrivateKey", "failed to write private key", err)
	}
	return nil
}

// LoadPrivateKey reads a private key from a file and returns it as a string
// suitable for use with WithHiddenServicePrivateKey.
//
// Example:
//
//	key, _ := tornago.LoadPrivateKey("/path/to/key")
//	cfg, _ := tornago.NewHiddenServiceConfig(
//	    tornago.WithHiddenServicePrivateKey(key),
//	    tornago.WithHiddenServicePort(80, 8080),
//	)
func LoadPrivateKey(path string) (string, error) {
	// #nosec G304 -- path is user-provided and expected to be trusted
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return "", newError(ErrIO, "LoadPrivateKey", "failed to read private key", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// WithHiddenServicePrivateKeyFile loads a private key from a file and uses it.
// This is a convenience option that combines LoadPrivateKey and WithHiddenServicePrivateKey.
func WithHiddenServicePrivateKeyFile(path string) HiddenServiceOption {
	return func(cfg *HiddenServiceConfig) {
		key, err := LoadPrivateKey(path)
		if err == nil && key != "" {
			cfg.privateKey = key
		}
	}
}

// buildAddOnionCommand constructs the ADD_ONION command string from the configuration.
// The command format is: ADD_ONION KeyType:Key Port=virt,target [ClientAuth=name:key]
func buildAddOnionCommand(cfg HiddenServiceConfig) string {
	key := cfg.KeyType()
	if cfg.PrivateKey() == "" {
		key = "NEW:" + key
	} else {
		key = key + ":" + cfg.PrivateKey()
	}
	ports := cfg.Ports()
	auths := cfg.ClientAuth()
	parts := make([]string, 0, 2+len(ports)+len(auths))
	parts = append(parts, "ADD_ONION", key)

	var virts = make([]int, 0, len(ports))
	for virt := range ports {
		virts = append(virts, virt)
	}
	sort.Ints(virts)
	for _, virt := range virts {
		target := ports[virt]
		parts = append(parts, fmt.Sprintf("Port=%d,127.0.0.1:%d", virt, target))
	}

	for _, auth := range auths {
		parts = append(parts, fmt.Sprintf("ClientAuth=%s:%s", auth.ClientName(), auth.Key()))
	}

	return strings.Join(parts, " ")
}
