package tornago

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// opStartTorDaemon labels errors originating from StartTorDaemon.
	opStartTorDaemon = "StartTorDaemon"
)

// TorProcess represents a running tor daemon launched by Tornago. It is immutable
// and exposes read-only accessors for its properties.
type TorProcess struct {
	// pid is the process identifier of the launched tor daemon.
	pid int
	// socksAddr is the resolved address of the SocksPort.
	socksAddr string
	// controlAddr is the resolved address of the ControlPort.
	controlAddr string
	// cmd references the exec.Cmd used to launch tor so we can stop it later.
	cmd *exec.Cmd
	// process points to the running os.Process for cleanup.
	process *os.Process
	// dataDir stores the temporary Tor data directory for cleanup.
	dataDir string
	// cleanupDataDir signals whether Tornago owns the data directory lifecycle.
	cleanupDataDir bool
}

// PID returns the process identifier of the launched tor daemon.
func (p TorProcess) PID() int { return p.pid }

// SocksAddr returns the resolved SocksPort address of the launched tor daemon.
func (p TorProcess) SocksAddr() string { return p.socksAddr }

// ControlAddr returns the resolved ControlPort address of the launched tor daemon.
func (p TorProcess) ControlAddr() string { return p.controlAddr }

// DataDir returns the Tor data directory path used by this process.
func (p TorProcess) DataDir() string { return p.dataDir }

// Stop terminates the tor process and cleans up temporary resources.
func (p *TorProcess) Stop() error {
	if p == nil {
		return nil
	}
	var err error
	if p.cmd != nil {
		if stopErr := terminateCmd(p.cmd); stopErr != nil {
			err = errors.Join(err, stopErr)
		}
		p.cmd = nil
		p.process = nil
	} else if p.process != nil {
		killErr := p.process.Kill()
		if killErr != nil {
			err = errors.Join(err, killErr)
		}
		p.process = nil
	}
	if p.cleanupDataDir && p.dataDir != "" {
		if rmErr := os.RemoveAll(p.dataDir); rmErr != nil {
			err = errors.Join(err, rmErr)
		}
		p.dataDir = ""
		p.cleanupDataDir = false
	}
	return err
}

// StartTorDaemon launches the tor binary as a child process using the provided
// configuration. It waits until both the SocksPort and ControlPort become
// reachable or until StartupTimeout elapses.
//
// This function is useful when you want your application to manage its own Tor instance
// rather than relying on a system-wide Tor daemon. StartTorDaemon handles:
//   - Finding the tor binary in PATH (install via: apt install tor, brew install tor, choco install tor)
//   - Allocating free ports when using ":0" addresses
//   - Configuring cookie authentication automatically
//   - Waiting for Tor to become ready before returning
//   - Creating/managing the Tor DataDirectory
//
// The returned TorProcess must be stopped via Stop() to cleanly terminate Tor and
// clean up resources. Use defer torProc.Stop() to ensure cleanup.
//
// Example usage:
//
//	cfg, _ := tornago.NewTorLaunchConfig(
//	    tornago.WithTorSocksAddr(":0"),     // Auto-select free port
//	    tornago.WithTorControlAddr(":0"),   // Auto-select free port
//	)
//	torProc, err := tornago.StartTorDaemon(cfg)
//	if err != nil {
//	    log.Fatalf("failed to start tor: %v", err)
//	}
//	defer torProc.Stop()
//
//	// Use torProc.SocksAddr() and torProc.ControlAddr() to connect
//	clientCfg, _ := tornago.NewClientConfig(
//	    tornago.WithClientSocksAddr(torProc.SocksAddr()),
//	)
//	client, _ := tornago.NewClient(clientCfg)
//	defer client.Close()
func StartTorDaemon(cfg TorLaunchConfig) (_ *TorProcess, err error) {
	cfg, err = normalizeTorLaunchConfig(cfg)
	if err != nil {
		return nil, err
	}

	logger := cfg.Logger()
	logger.Log("info", "starting Tor daemon", "socks_addr", cfg.SocksAddr(), "control_addr", cfg.ControlAddr())

	dataDir := cfg.DataDir()
	cleanupDataDir := false
	if dataDir == "" {
		dataDir, err = os.MkdirTemp("", "tornago-tor-data-*")
		if err != nil {
			logger.Log("error", "failed to create data directory", "error", err)
			return nil, newError(ErrIO, opStartTorDaemon, "failed to create data directory", err)
		}
		cleanupDataDir = true
		logger.Log("debug", "created temporary data directory", "path", dataDir)
	} else {
		dataDir = filepath.Clean(dataDir)
		if err := os.MkdirAll(dataDir, 0o700); err != nil {
			msg := "failed to create data directory " + dataDir
			logger.Log("error", msg, "error", err)
			return nil, newError(ErrIO, opStartTorDaemon, msg, err)
		}
		logger.Log("debug", "using persistent data directory", "path", dataDir)
	}

	cleanupOnFail := cleanupDataDir
	defer func() {
		if cleanupOnFail && dataDir != "" {
			if rmErr := os.RemoveAll(dataDir); rmErr != nil {
				err = errors.Join(err, rmErr)
			}
		}
	}()

	binPath, err := exec.LookPath(cfg.TorBinary())
	if err != nil {
		msg := fmt.Sprintf("tor binary not found. Install tor via your package manager (e.g. apt-get install tor, brew install tor, pacman -S tor). attempted: %q", cfg.TorBinary())
		return nil, newError(ErrTorBinaryNotFound, opStartTorDaemon, msg, err)
	}

	socksAddr, err := resolveAddr(cfg.SocksAddr())
	if err != nil {
		return nil, newError(ErrInvalidConfig, opStartTorDaemon, "invalid SocksAddr", err)
	}
	controlAddr, err := resolveAddr(cfg.ControlAddr())
	if err != nil {
		return nil, newError(ErrInvalidConfig, opStartTorDaemon, "invalid ControlAddr", err)
	}

	cmdArgs := make([]string, 0)
	if torConfig := cfg.TorConfigFile(); torConfig != "" {
		// When using torrc file, only pass -f and extra args
		cmdArgs = append(cmdArgs, "-f", torConfig)
		cmdArgs = append(cmdArgs, cfg.ExtraArgs()...)
	} else {
		// When not using torrc, pass all settings as command-line args
		cookiePath := filepath.Join(dataDir, "control_auth_cookie")
		args := []string{
			"--SocksPort", socksAddr,
			"--ControlPort", controlAddr,
			"--CookieAuthentication", "1",
			"--CookieAuthFile", cookiePath,
			"--RunAsDaemon", "0",
			"--DataDirectory", dataDir,
			"--Log", "notice stdout",
		}
		args = append(args, cfg.ExtraArgs()...)
		cmdArgs = append(cmdArgs, args...)
	}

	// #nosec G204 -- arguments are fully controlled by validated TorLaunchConfig.
	// NOTE: We use exec.Command (not CommandContext) because the tor process should
	// stay alive after StartTorDaemon returns. The context is only for waiting for ports.
	cmd := exec.Command(binPath, cmdArgs...) //nolint:noctx
	var stdoutBuf, stderrBuf bytes.Buffer

	logReporter := cfg.LogReporter()
	if logReporter != nil {
		// Use teeWriter to both capture and report logs in real-time
		cmd.Stdout = &teeWriter{buf: &stdoutBuf, reporter: logReporter}
		cmd.Stderr = &teeWriter{buf: &stderrBuf, reporter: logReporter}
	} else {
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
	}

	logOutput := func() string {
		return strings.TrimSpace(stdoutBuf.String() + "\n" + stderrBuf.String())
	}
	reportLogs := func() string {
		return logOutput()
	}
	attachLogs := func(msg string) string {
		logged := reportLogs()
		if logged != "" {
			return msg + ": " + logged
		}
		return msg
	}

	if startErr := cmd.Start(); startErr != nil {
		logger.Log("error", "failed to start tor process", "error", startErr)
		err = newError(ErrTorLaunchFailed, opStartTorDaemon, attachLogs("failed to start tor"), startErr)
		return nil, err
	}

	logger.Log("debug", "tor process started", "pid", cmd.Process.Pid)

	// Create a context for waiting for ports to become ready
	ctx, cancel := context.WithTimeout(context.Background(), cfg.StartupTimeout())
	defer cancel()

	logger.Log("debug", "waiting for tor ports to become ready", "timeout", cfg.StartupTimeout())

	if waitErr := waitForPorts(ctx, socksAddr, controlAddr); waitErr != nil {
		if stopErr := terminateCmd(cmd); stopErr != nil {
			waitErr = errors.Join(waitErr, stopErr)
		}
		logger.Log("error", "tor ports did not become ready", "error", waitErr)
		err = newError(ErrTorLaunchFailed, opStartTorDaemon, attachLogs("tor process exited before ports became reachable"), waitErr)
		return nil, err
	}

	proc := &TorProcess{
		pid:            cmd.Process.Pid,
		socksAddr:      socksAddr,
		controlAddr:    controlAddr,
		process:        cmd.Process,
		dataDir:        dataDir,
		cleanupDataDir: cleanupDataDir,
		cmd:            cmd,
	}
	cleanupOnFail = false
	logger.Log("info", "Tor daemon started successfully", "pid", proc.pid, "socks_addr", proc.socksAddr, "control_addr", proc.controlAddr)
	return proc, nil
}

// waitForPorts polls for SocksPort/ControlPort reachability or timeout.
func waitForPorts(ctx context.Context, socksAddr, controlAddr string) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return newError(ErrTimeout, "waitForPorts", "timed out waiting for tor to become ready", ctx.Err())
		case <-ticker.C:
			if portsReachable(socksAddr, controlAddr) {
				return nil
			}
		}
	}
}

// teeWriter writes to a buffer and reports each line via callback.
type teeWriter struct {
	buf      *bytes.Buffer
	reporter func(string)
	partial  []byte
}

// Write implements io.Writer, buffering lines and reporting them.
func (w *teeWriter) Write(p []byte) (int, error) {
	// Write to buffer
	n, err := w.buf.Write(p)

	// Report lines to callback
	if w.reporter != nil {
		data := append(w.partial, p...)
		lines := bytes.Split(data, []byte("\n"))

		// All but the last element are complete lines
		for i := range len(lines) - 1 {
			w.reporter(string(lines[i]))
		}

		// Keep the last partial line for next write
		w.partial = lines[len(lines)-1]
	}

	return n, err
}

// terminateCmd kills the process associated with cmd and waits for it to exit.
func terminateCmd(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if killErr := cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
		return killErr
	}
	if waitErr := cmd.Wait(); waitErr != nil && !errors.Is(waitErr, os.ErrProcessDone) {
		return waitErr
	}
	return nil
}

// portsReachable checks whether both tor ports accept TCP connections.
func portsReachable(socksAddr, controlAddr string) bool {
	dialer := &net.Dialer{Timeout: 500 * time.Millisecond}
	check := func(addr string) bool {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}
	return check(socksAddr) && check(controlAddr)
}

// resolveAddr resolves the given address, assigning a free port when port is zero.
func resolveAddr(addr string) (string, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return "", err
	}
	if tcpAddr.Port != 0 {
		return tcpAddr.String(), nil
	}
	host := tcpAddr.IP.String()
	if host == "<nil>" || host == "" {
		host = "127.0.0.1"
	}
	lc := net.ListenConfig{}
	l, err := lc.Listen(context.Background(), "tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return "", err
	}
	defer l.Close()
	tcpAddr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		return "", newError(ErrUnknown, "resolveAddr", "unexpected listener address type", nil)
	}
	port := tcpAddr.Port
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}
