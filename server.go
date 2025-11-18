package tornago

// Server exposes Tor SocksPort and ControlPort addresses for clients to use.
type Server interface {
	// SocksAddr returns the Tor SocksPort address.
	SocksAddr() string
	// ControlAddr returns the Tor ControlPort address.
	ControlAddr() string
}

// server is the default Server implementation backed by ServerConfig.
type server struct {
	// cfg holds the resolved server configuration.
	cfg ServerConfig
}

// NewServer builds a Server from the given configuration.
func NewServer(cfg ServerConfig) (Server, error) {
	cfg, err := normalizeServerConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &server{cfg: cfg}, nil
}

// SocksAddr returns the Tor SocksPort address.
func (s *server) SocksAddr() string {
	return s.cfg.SocksAddr()
}

// ControlAddr returns the Tor ControlPort address.
func (s *server) ControlAddr() string {
	return s.cfg.ControlAddr()
}
