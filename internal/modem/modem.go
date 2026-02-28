package modem

import (
	"bufio"
	"fmt"
	"io"
	"log"
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

// DialResponse holds the result and raw AT transcript from a dial attempt.
type DialResponse struct {
	Result     DialResult
	Transcript string // raw AT command/response exchange
}

// Modem represents an open modem device.
type Modem struct {
	dev    *os.File
	path   string
	reader *bufio.Reader
	log    strings.Builder // accumulates the full AT transcript
}

// Open opens a modem device at the given path.
func Open(devicePath string) (*Modem, error) {
	f, err := os.OpenFile(devicePath, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("opening modem device %s: %w", devicePath, err)
	}
	m := &Modem{
		dev:    f,
		path:   devicePath,
		reader: bufio.NewReader(f),
	}
	log.Printf("[modem] opened %s", devicePath)
	return m, nil
}

// Reset sends ATZ and waits for OK.
func (m *Modem) Reset(timeout time.Duration) error {
	m.logCmd("ATZ")
	if _, err := m.dev.Write([]byte("ATZ\r")); err != nil {
		return fmt.Errorf("sending ATZ: %w", err)
	}
	resp, err := m.readUntil(timeout, "OK", "ERROR")
	m.logResp(resp)
	if err != nil {
		return fmt.Errorf("ATZ: no response (%w)", err)
	}
	if strings.Contains(resp, "ERROR") {
		return fmt.Errorf("ATZ returned ERROR: %s", cleanResponse(resp))
	}
	return nil
}

// Dial sends ATDT and returns the result with full transcript.
func (m *Modem) Dial(phone string, timeout time.Duration) (DialResponse, error) {
	cmd := fmt.Sprintf("ATDT%s", phone)
	m.logCmd(cmd)
	if _, err := m.dev.Write([]byte(cmd + "\r")); err != nil {
		return DialResponse{Result: ResultError, Transcript: m.log.String()},
			fmt.Errorf("sending ATDT: %w", err)
	}

	resp, err := m.readUntil(timeout, "CONNECT", "BUSY", "NO CARRIER", "NO DIALTONE", "ERROR")
	m.logResp(resp)

	transcript := m.log.String()

	if err != nil {
		log.Printf("[modem] %s dial timeout\n%s", m.path, transcript)
		return DialResponse{Result: ResultTimeout, Transcript: transcript}, nil
	}

	var result DialResult
	switch {
	case strings.Contains(resp, "CONNECT"):
		result = ResultConnect
	case strings.Contains(resp, "BUSY"):
		result = ResultBusy
	case strings.Contains(resp, "NO CARRIER"):
		result = ResultNoCarrier
	case strings.Contains(resp, "NO DIALTONE"):
		result = ResultNoDialtone
	default:
		result = ResultError
	}

	log.Printf("[modem] %s dial result: %s\n%s", m.path, result, transcript)
	return DialResponse{Result: result, Transcript: transcript}, nil
}

// Transcript returns the accumulated AT command log.
func (m *Modem) Transcript() string {
	return m.log.String()
}

// Hangup sends the escape sequence and ATH to hang up.
func (m *Modem) Hangup() error {
	log.Printf("[modem] %s hangup", m.path)
	time.Sleep(1100 * time.Millisecond)
	if _, err := m.dev.Write([]byte("+++")); err != nil {
		return fmt.Errorf("sending escape: %w", err)
	}
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
	log.Printf("[modem] %s closed", m.path)
	return m.dev.Close()
}

func (m *Modem) logCmd(cmd string) {
	line := fmt.Sprintf(">>> %s\n", cmd)
	m.log.WriteString(line)
	log.Printf("[modem] %s send: %s", m.path, cmd)
}

func (m *Modem) logResp(resp string) {
	cleaned := cleanResponse(resp)
	if cleaned != "" {
		line := fmt.Sprintf("<<< %s\n", cleaned)
		m.log.WriteString(line)
		log.Printf("[modem] %s recv: %s", m.path, cleaned)
	}
}

// readUntil reads lines until one contains a match string or timeout.
func (m *Modem) readUntil(timeout time.Duration, matches ...string) (string, error) {
	deadline := time.Now().Add(timeout)
	var accumulated strings.Builder

	for time.Now().Before(deadline) {
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
	return accumulated.String(), fmt.Errorf("timeout after %s", timeout)
}

// cleanResponse strips control chars and excess whitespace from modem output.
func cleanResponse(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}
