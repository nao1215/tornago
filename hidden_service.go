package tornago

import (
	"context"
	"fmt"
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

// HiddenService represents a provisioned Hidden Service.
type HiddenService interface {
	OnionAddress() string
	PrivateKey() string
	Ports() map[int]int
	ClientAuth() []HiddenServiceAuth
	Remove(ctx context.Context) error
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

func normalizeHiddenServiceConfig(cfg HiddenServiceConfig) (HiddenServiceConfig, error) {
	cfg = applyHiddenServiceDefaults(cfg)
	if err := validateHiddenServiceConfig(cfg); err != nil {
		return HiddenServiceConfig{}, err
	}
	cfg.targetPort = cfg.Ports()
	cfg.clientAuth = cfg.ClientAuth()
	return cfg, nil
}

func applyHiddenServiceDefaults(cfg HiddenServiceConfig) HiddenServiceConfig {
	if cfg.keyType == "" {
		cfg.keyType = "ED25519-V3"
	}
	return cfg
}

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
