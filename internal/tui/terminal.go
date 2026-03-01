package tui

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/gbm-dev/pots/internal/modem"
	"github.com/gbm-dev/pots/internal/session"
)

// TerminalSession is a tea.ExecCommand that runs raw bidirectional I/O
// between the user's terminal and the modem, with ~. escape detection.
type TerminalSession struct {
	modem    *modem.Modem
	device   string
	siteName string
	lock     *modem.DeviceLock
	logger   *session.Logger
	logDir   string

	stdin  io.Reader // set by tea.Exec via SetStdin
	stdout io.Writer // set by tea.Exec via SetStdout
}

// NewTerminalSession creates a terminal pass-through session.
func NewTerminalSession(mdm *modem.Modem, device, siteName, logDir string, lock *modem.DeviceLock) *TerminalSession {
	return &TerminalSession{
		modem:    mdm,
		device:   device,
		siteName: siteName,
		lock:     lock,
		logDir:   logDir,
	}
}

// Run implements tea.ExecCommand. It takes over stdin/stdout for raw I/O.
func (t *TerminalSession) Run() error {
	// Create session logger
	var err error
	t.logger, err = session.NewLogger(t.logDir, t.siteName, t.device)
	if err != nil {
		return fmt.Errorf("creating session logger: %w", err)
	}
	defer t.cleanup()

	rwc := t.modem.ReadWriteCloser()

	// Use the I/O provided by tea.Exec (SSH channel), fall back to os std.
	stdin := t.stdin
	stdout := t.stdout
	if stdin == nil {
		stdin = os.Stdin
	}
	if stdout == nil {
		stdout = os.Stdout
	}

	// Print connection banner
	banner := fmt.Sprintf("\r\n*** CONNECTED to %s — Press Enter then ~. to disconnect ***\r\n\r\n", t.siteName)
	fmt.Fprint(stdout, banner)

	// Modem→user: tee to logger, track when we first receive data
	loggedReader := t.logger.TeeReader(rwc)
	var gotData atomic.Bool

	done := make(chan error, 2)

	// Modem → user
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := loggedReader.Read(buf)
			if n > 0 {
				gotData.Store(true)
				if _, werr := stdout.Write(buf[:n]); werr != nil {
					done <- werr
					return
				}
			}
			if err != nil {
				done <- err
				return
			}
		}
	}()

	// User → modem (with ~. escape detection)
	go func() {
		done <- t.userToModem(stdin, rwc)
	}()

	// Send Enter every 2s until remote responds, then stop.
	// Keeps modem carrier alive and wakes the remote terminal.
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		// Send first Enter immediately
		if _, err := rwc.Write([]byte("\r")); err != nil {
			slog.Debug("wake: initial enter failed", "err", err)
			return
		}
		slog.Debug("wake: sent initial enter")
		for range ticker.C {
			if gotData.Load() {
				slog.Debug("wake: got data from remote, stopping")
				return
			}
			if _, err := rwc.Write([]byte("\r")); err != nil {
				slog.Debug("wake: enter failed", "err", err)
				return
			}
			slog.Debug("wake: sent enter")
		}
	}()

	// Wait for either direction to finish
	return <-done
}

// SetStdin stores the SSH session's stdin for use in Run().
func (t *TerminalSession) SetStdin(r io.Reader) { t.stdin = r }

// SetStdout stores the SSH session's stdout for use in Run().
func (t *TerminalSession) SetStdout(w io.Writer) { t.stdout = w }

// SetStderr is required by tea.ExecCommand.
func (t *TerminalSession) SetStderr(w io.Writer) {}

// userToModem reads from user and writes to modem, detecting ~. escape.
func (t *TerminalSession) userToModem(r io.Reader, w io.Writer) error {
	buf := make([]byte, 1)
	var prevWasEnter, prevWasTilde bool

	for {
		n, err := r.Read(buf)
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}

		b := buf[0]

		// Escape sequence: Enter, ~, .
		if prevWasTilde && b == '.' {
			return nil // disconnect
		}
		if prevWasEnter && b == '~' {
			prevWasTilde = true
			prevWasEnter = false
			continue // Don't forward ~ yet
		}

		// If we had a pending ~ that wasn't followed by ., forward it
		if prevWasTilde {
			w.Write([]byte{'~'})
			prevWasTilde = false
		}

		prevWasEnter = (b == '\r' || b == '\n')
		prevWasTilde = false

		if _, err := w.Write(buf[:n]); err != nil {
			return err
		}
	}
}

func (t *TerminalSession) cleanup() {
	if t.logger != nil {
		t.logger.Close()
	}
	t.modem.Hangup()
	t.modem.Close()
	t.lock.Release()
}
