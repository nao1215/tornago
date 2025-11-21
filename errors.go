package tornago

import (
	"fmt"
)

// ErrorKind classifies Tornago errors for easier handling and retry decisions.
type ErrorKind string

// ErrorKind values classify tornago errors by their category.
const (
	// ErrInvalidConfig indicates user-supplied configuration is invalid.
	ErrInvalidConfig ErrorKind = "invalid_config"
	// ErrTorBinaryNotFound indicates the tor executable could not be located.
	ErrTorBinaryNotFound ErrorKind = "tor_binary_not_found"
	// ErrTorLaunchFailed indicates tor failed to launch or exited unexpectedly.
	ErrTorLaunchFailed ErrorKind = "tor_launch_failed"
	// ErrSocksDialFailed indicates SOCKS5 dialing failed.
	ErrSocksDialFailed ErrorKind = "socks_dial_failed"
	// ErrControlAuthFailed indicates ControlPort authentication failed.
	ErrControlAuthFailed ErrorKind = "control_auth_failed"
	// ErrControlRequestFail indicates a ControlPort request returned an error.
	ErrControlRequestFail ErrorKind = "control_request_failed"
	// ErrHTTPFailed indicates an HTTP request via Tor failed.
	ErrHTTPFailed ErrorKind = "http_failed"
	// ErrTimeout indicates an operation exceeded its deadline.
	ErrTimeout ErrorKind = "timeout"
	// ErrIO wraps generic I/O errors.
	ErrIO ErrorKind = "io_error"
	// ErrHiddenServiceFailed indicates Hidden Service creation/removal failed.
	ErrHiddenServiceFailed ErrorKind = "hidden_service_failed"
	// ErrListenerClosed indicates an operation was attempted on a closed listener.
	ErrListenerClosed ErrorKind = "listener_closed"
	// ErrListenerCloseFailed indicates the listener failed to close properly.
	ErrListenerCloseFailed ErrorKind = "listener_close_failed"
	// ErrAcceptFailed indicates Accept() failed on a listener.
	ErrAcceptFailed ErrorKind = "accept_failed"
	// ErrUnknown is used when no specific classification is available.
	ErrUnknown ErrorKind = "unknown"
)

// TornagoError wraps an underlying error with a Kind and an optional operation
// label so callers can branch on error type while retaining context.
//
//revive:disable-next-line:exported
type TornagoError struct {
	// Kind classifies the error for programmatic handling.
	Kind ErrorKind
	// Op names the operation during which the error occurred.
	Op string
	// Msg carries an optional human-readable description.
	Msg string
	// Err stores the wrapped underlying error.
	Err error
}

// Error returns a formatted string that includes Kind, Op, and the wrapped error.
func (e *TornagoError) Error() string {
	if e == nil {
		return ""
	}

	message := string(e.Kind)
	if e.Op != "" {
		message = fmt.Sprintf("%s: %s", e.Op, message)
	}
	if e.Msg != "" {
		message = fmt.Sprintf("%s: %s", message, e.Msg)
	}
	if e.Err != nil {
		message = fmt.Sprintf("%s: %s", message, e.Err)
	}
	return message
}

// Unwrap exposes the underlying error for errors.Is / errors.As compatibility.
func (e *TornagoError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Is reports whether target has the same ErrorKind, enabling errors.Is checks.
func (e *TornagoError) Is(target error) bool {
	te, ok := target.(*TornagoError)
	if !ok {
		return false
	}
	if e == nil {
		return false
	}
	return e.Kind != "" && e.Kind == te.Kind
}

// newError constructs a TornagoError, defaulting Kind to ErrUnknown when empty.
func newError(kind ErrorKind, op, msg string, err error) *TornagoError {
	if kind == "" {
		kind = ErrUnknown
	}
	return &TornagoError{
		Kind: kind,
		Op:   op,
		Msg:  msg,
		Err:  err,
	}
}
