package modem

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// DialResult represents the outcome of a dial attempt.
type DialResult int

const (
	ResultConnect    DialResult = iota
	ResultBusy
	ResultNoCarrier
	ResultNoDialtone
	ResultError
	ResultTimeout
)

func (r DialResult) String() string {
	switch r {
	case ResultConnect:
		return "CONNECT"
	case ResultBusy:
		return "BUSY"
	case ResultNoCarrier:
		return "NO CARRIER"
	case ResultNoDialtone:
		return "NO DIALTONE"
	case ResultError:
		return "ERROR"
	case ResultTimeout:
		return "TIMEOUT"
	default:
		return "UNKNOWN"
	}
}

// Modem represents an open modem device.
type Modem struct {
	dev    *os.File
	path   string
	reader *bufio.Reader
}

// Open opens a modem device at the given path.
func Open(devicePath string) (*Modem, error) {
	f, err := os.OpenFile(devicePath, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("opening modem device %s: %w", devicePath, err)
	}
	return &Modem{
		dev:    f,
		path:   devicePath,
		reader: bufio.NewReader(f),
	}, nil
}

// Reset sends ATZ and waits for OK.
func (m *Modem) Reset(timeout time.Duration) error {
	if _, err := m.dev.Write([]byte("ATZ\r")); err != nil {
		return fmt.Errorf("sending ATZ: %w", err)
	}
	resp, err := m.readUntil(timeout, "OK", "ERROR")
	if err != nil {
		return fmt.Errorf("ATZ response: %w", err)
	}
	if strings.Contains(resp, "ERROR") {
		return fmt.Errorf("ATZ returned ERROR")
	}
	return nil
}

// Dial sends ATDT and returns the result.
func (m *Modem) Dial(phone string, timeout time.Duration) (DialResult, error) {
	cmd := fmt.Sprintf("ATDT%s\r", phone)
	if _, err := m.dev.Write([]byte(cmd)); err != nil {
		return ResultError, fmt.Errorf("sending ATDT: %w", err)
	}
	resp, err := m.readUntil(timeout, "CONNECT", "BUSY", "NO CARRIER", "NO DIALTONE", "ERROR")
	if err != nil {
		return ResultTimeout, nil
	}
	switch {
	case strings.Contains(resp, "CONNECT"):
		return ResultConnect, nil
	case strings.Contains(resp, "BUSY"):
		return ResultBusy, nil
	case strings.Contains(resp, "NO CARRIER"):
		return ResultNoCarrier, nil
	case strings.Contains(resp, "NO DIALTONE"):
		return ResultNoDialtone, nil
	default:
		return ResultError, nil
	}
}

// Hangup sends the escape sequence and ATH to hang up.
func (m *Modem) Hangup() error {
	// Guard time before escape
	time.Sleep(1100 * time.Millisecond)
	if _, err := m.dev.Write([]byte("+++")); err != nil {
		return fmt.Errorf("sending escape: %w", err)
	}
	// Guard time after escape
	time.Sleep(1100 * time.Millisecond)
	if _, err := m.dev.Write([]byte("ATH\r")); err != nil {
		return fmt.Errorf("sending ATH: %w", err)
	}
	m.readUntil(3*time.Second, "OK", "ERROR")
	return nil
}

// ReadWriteCloser returns the underlying device for raw I/O pass-through.
func (m *Modem) ReadWriteCloser() io.ReadWriteCloser {
	return m.dev
}

// Close closes the modem device.
func (m *Modem) Close() error {
	return m.dev.Close()
}

// readUntil reads lines until one contains a match string or timeout.
func (m *Modem) readUntil(timeout time.Duration, matches ...string) (string, error) {
	deadline := time.Now().Add(timeout)
	var accumulated strings.Builder

	for time.Now().Before(deadline) {
		// Set read deadline on the file
		m.dev.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		line, err := m.reader.ReadString('\n')
		accumulated.WriteString(line)

		for _, match := range matches {
			if strings.Contains(accumulated.String(), match) {
				m.dev.SetReadDeadline(time.Time{})
				return accumulated.String(), nil
			}
		}

		if err != nil && !os.IsTimeout(err) {
			return accumulated.String(), err
		}
	}
	m.dev.SetReadDeadline(time.Time{})
	return accumulated.String(), fmt.Errorf("timeout waiting for response")
}
