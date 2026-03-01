package tui

import (
	"fmt"
	"io"
	"os"

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

	// Print connection banner
	banner := fmt.Sprintf("\r\n*** CONNECTED to %s — Press Enter then ~. to disconnect ***\r\n\r\n", t.siteName)
	os.Stdout.WriteString(banner)

	// Modem→user: tee to logger
	loggedReader := t.logger.TeeReader(rwc)

	done := make(chan error, 2)

	// Modem → user
	go func() {
		_, err := io.Copy(os.Stdout, loggedReader)
		done <- err
	}()

	// User → modem (with ~. escape detection)
	go func() {
		done <- t.userToModem(os.Stdin, rwc)
	}()

	// Wait for either direction to finish
	return <-done
}

// SetStdin is required by tea.ExecCommand but we manage stdin ourselves.
func (t *TerminalSession) SetStdin(r io.Reader) {}

// SetStdout is required by tea.ExecCommand but we manage stdout ourselves.
func (t *TerminalSession) SetStdout(w io.Writer) {}

// SetStderr is required by tea.ExecCommand but we manage stderr ourselves.
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
