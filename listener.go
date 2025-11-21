package tornago

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

// OnionAddr represents a .onion address that implements net.Addr.
type OnionAddr struct {
	// address is the full .onion address (e.g., "abc123.onion:80").
	address string
	// port is the virtual port on the onion service.
	port int
}

// Network returns the network type, always "onion".
func (a *OnionAddr) Network() string {
	return "onion"
}

// String returns the full address in "host:port" format.
func (a *OnionAddr) String() string {
	return a.address
}

// Port returns the virtual port number.
func (a *OnionAddr) Port() int {
	return a.port
}

// TorListener implements net.Listener for Tor Hidden Services.
// It wraps a local TCP listener and exposes it as a Tor onion service.
//
// Example usage:
//
//	client, _ := tornago.NewClient(tornago.NewClientConfig(...))
//	listener, _ := client.Listen(ctx, 80, 8080) // onion:80 -> local:8080
//	defer listener.Close()
//
//	for {
//	    conn, err := listener.Accept()
//	    if err != nil {
//	        break
//	    }
//	    go handleConnection(conn)
//	}
type TorListener struct {
	// underlying is the local TCP listener that accepts connections.
	underlying net.Listener
	// hiddenService is the Tor hidden service backing this listener.
	hiddenService HiddenService
	// onionAddr is the .onion address with port.
	onionAddr *OnionAddr
	// virtualPort is the port exposed on the onion address.
	virtualPort int
	// closed indicates whether the listener has been closed.
	closed bool
	// mu protects the closed field.
	mu sync.Mutex
}

// Accept waits for and returns the next connection to the listener.
// This implements net.Listener.
func (l *TorListener) Accept() (net.Conn, error) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil, newError(ErrListenerClosed, "TorListener.Accept", "listener is closed", nil)
	}
	underlying := l.underlying
	l.mu.Unlock()

	if underlying == nil {
		return nil, newError(ErrAcceptFailed, "TorListener.Accept", "underlying listener is nil", nil)
	}

	conn, err := underlying.Accept()
	if err != nil {
		return nil, newError(ErrAcceptFailed, "TorListener.Accept", "failed to accept connection", err)
	}
	return conn, nil
}

// Close stops listening and removes the hidden service from Tor.
// This implements net.Listener.
func (l *TorListener) Close() error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	l.mu.Unlock()

	var errs []error

	// Remove the hidden service from Tor with a bounded timeout.
	if l.hiddenService != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := l.hiddenService.Remove(ctx); err != nil {
			errs = append(errs, err)
		}
		cancel()
	}

	// Close the underlying listener.
	if l.underlying != nil {
		if err := l.underlying.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return newError(ErrListenerCloseFailed, "TorListener.Close", "failed to close listener", errors.Join(errs...))
	}
	return nil
}

// Addr returns the .onion address of the listener.
// This implements net.Listener.
func (l *TorListener) Addr() net.Addr {
	return l.onionAddr
}

// OnionAddress returns the full .onion address (e.g., "abc123.onion").
func (l *TorListener) OnionAddress() string {
	if l.hiddenService == nil {
		return ""
	}
	return l.hiddenService.OnionAddress()
}

// HiddenService returns the underlying HiddenService.
// This can be used to access the private key or other hidden service details.
func (l *TorListener) HiddenService() HiddenService {
	return l.hiddenService
}

// VirtualPort returns the port exposed on the .onion address.
func (l *TorListener) VirtualPort() int {
	return l.virtualPort
}
