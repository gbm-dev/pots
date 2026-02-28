package modem

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
	"unicode"
)

// DialResult represents the outcome of a dial attempt.
type DialResult int

const (
	ResultConnect DialResult = iota
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
	dev  *os.File
	path string
	log  strings.Builder // accumulates the full AT transcript
}

// Open opens a modem device at the given path.
func Open(devicePath string) (*Modem, error) {
	f, err := os.OpenFile(devicePath, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("opening modem device %s: %w", devicePath, err)
	}
	m := &Modem{
		dev:  f,
		path: devicePath,
	}
	log.Printf("[modem] opened %s", devicePath)
	return m, nil
}

// Init sends ATE0 (disable echo) and ATZ (reset). Call after Open.
func (m *Modem) Init(timeout time.Duration) error {
	// Drain any stale data in the buffer
	m.drain()

	// Reset modem first. ATZ can restore default settings.
	resp, err := m.runAT("ATZ", timeout, "OK", "ERROR")
	if err != nil {
		return fmt.Errorf("ATZ: no response (%w)", err)
	}
	if strings.Contains(resp, "ERROR") {
		return fmt.Errorf("ATZ returned ERROR: %s", cleanResponse(resp))
	}
	m.drain()

	// Disable echo after reset so it stays off for dial commands.
	resp, err = m.runAT("ATE0", timeout, "OK", "ERROR")
	if err != nil {
		return fmt.Errorf("ATE0: no response (%w)", err)
	}
	if strings.Contains(resp, "ERROR") {
		return fmt.Errorf("ATE0 returned ERROR: %s", cleanResponse(resp))
	}

	// d-modem needs explicit no-dialtone mode and stable modulation limits.
	if err := m.runInitCmd("ATX3", timeout); err != nil {
		return err
	}

	modulation := os.Getenv("DMODEM_AT_MS")
	if strings.TrimSpace(modulation) == "" {
		modulation = "AT+MS=132,0,4800,9600"
	}
	if err := m.runInitCmd(modulation, timeout); err != nil {
		return err
	}

	// Drain again after reset to clear any echo/noise
	m.drain()
	return nil
}

func (m *Modem) runInitCmd(cmd string, timeout time.Duration) error {
	resp, err := m.runAT(cmd, timeout, "OK", "ERROR")
	if err != nil {
		return fmt.Errorf("%s: no response (%w)", cmd, err)
	}
	if strings.Contains(resp, "ERROR") {
		return fmt.Errorf("%s returned ERROR: %s", cmd, cleanResponse(resp))
	}
	return nil
}

// Dial sends ATDT and returns the result with full transcript.
func (m *Modem) Dial(phone string, timeout time.Duration) (DialResponse, error) {
	// Drain before dialing to ensure clean buffer
	m.drain()

	dialPrefix := strings.TrimSpace(os.Getenv("MODEM_DIAL_PREFIX"))
	if dialPrefix == "" {
		// d-modem interprets ATDT as a literal leading "T" in SIP user part.
		// Use ATD by default so PSTN numbers are sent cleanly.
		dialPrefix = "ATD"
	}
	cmd := fmt.Sprintf("%s%s", dialPrefix, phone)
	m.logCmd(cmd)
	if _, err := m.dev.Write([]byte(cmd + "\r")); err != nil {
		return DialResponse{Result: ResultError, Transcript: m.log.String()},
			fmt.Errorf("sending dial command: %w", err)
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

// drain reads and discards any buffered data from the modem.
func (m *Modem) drain() {
	m.dev.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	buf := make([]byte, 1024)
	for {
		n, err := m.dev.Read(buf)
		if n > 0 {
			log.Printf("[modem] %s drain: %d bytes", m.path, n)
		}
		if err != nil {
			break
		}
	}
	m.dev.SetReadDeadline(time.Time{})
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

func (m *Modem) runAT(cmd string, timeout time.Duration, matches ...string) (string, error) {
	m.logCmd(cmd)
	if _, err := m.dev.Write([]byte(cmd + "\r")); err != nil {
		return "", fmt.Errorf("sending %s: %w", cmd, err)
	}
	resp, err := m.readUntil(timeout, matches...)
	m.logResp(resp)
	return resp, err
}

// readUntil reads lines until one contains a match string or timeout.
func (m *Modem) readUntil(timeout time.Duration, matches ...string) (string, error) {
	deadline := time.Now().Add(timeout)
	var accumulated strings.Builder
	buf := make([]byte, 1024)

	upperMatches := make([]string, len(matches))
	for i, match := range matches {
		upperMatches[i] = strings.ToUpper(match)
	}

	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		readStep := 500 * time.Millisecond
		if remaining < readStep {
			readStep = remaining
		}
		if readStep <= 0 {
			break
		}
		m.dev.SetReadDeadline(time.Now().Add(readStep))
		n, err := m.dev.Read(buf)
		if n > 0 {
			accumulated.Write(buf[:n])
			upperResp := strings.ToUpper(accumulated.String())

			for _, match := range upperMatches {
				if strings.Contains(upperResp, match) {
					m.dev.SetReadDeadline(time.Time{})
					return accumulated.String(), nil
				}
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
	s = strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == '\t' {
			return ' '
		}
		if !unicode.IsPrint(r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}
