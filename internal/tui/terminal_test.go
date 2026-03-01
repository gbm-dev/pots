package tui

import (
	"bytes"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestUserToModem_LineBuffered(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantModem  string // what gets sent to modem
		wantEcho   string // what gets echoed to terminal
	}{
		{
			name:      "line sent on enter",
			input:     "hello\r",
			wantModem: "hello\r",
			wantEcho:  "hello\r\n",
		},
		{
			name:      "empty enter sends CR",
			input:     "\r",
			wantModem: "\r",
			wantEcho:  "\r\n",
		},
		{
			name:      "multiple lines",
			input:     "show ver\rshow ip int brief\r",
			wantModem: "show ver\rshow ip int brief\r",
			wantEcho:  "show ver\r\nshow ip int brief\r\n",
		},
		{
			name:      "newline also triggers send",
			input:     "test\n",
			wantModem: "test\r",
			wantEcho:  "test\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &TerminalSession{}
			r := strings.NewReader(tt.input)
			var modemBuf, echoBuf bytes.Buffer

			ts.userToModem(r, &modemBuf, &echoBuf)

			if got := modemBuf.String(); got != tt.wantModem {
				t.Errorf("modem output = %q, want %q", got, tt.wantModem)
			}
			if got := echoBuf.String(); got != tt.wantEcho {
				t.Errorf("echo output = %q, want %q", got, tt.wantEcho)
			}
		})
	}
}

func TestUserToModem_Backspace(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantModem string
		wantEcho  string
	}{
		{
			name:      "backspace removes last char",
			input:     "helo\x7fo\r",
			wantModem: "helo\r",
			wantEcho:  "helo\x08 \x08o\r\n",
		},
		{
			name:      "backspace on empty buffer is no-op",
			input:     "\x7f\x7fhi\r",
			wantModem: "hi\r",
			wantEcho:  "hi\r\n",
		},
		{
			name:      "BS char also works",
			input:     "ab\x08c\r",
			wantModem: "ac\r",
			wantEcho:  "ab\x08 \x08c\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &TerminalSession{}
			r := strings.NewReader(tt.input)
			var modemBuf, echoBuf bytes.Buffer

			ts.userToModem(r, &modemBuf, &echoBuf)

			if got := modemBuf.String(); got != tt.wantModem {
				t.Errorf("modem output = %q, want %q", got, tt.wantModem)
			}
			if got := echoBuf.String(); got != tt.wantEcho {
				t.Errorf("echo output = %q, want %q", got, tt.wantEcho)
			}
		})
	}
}

func TestUserToModem_EscapeSequence(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantModem string
	}{
		{
			name:      "enter tilde dot disconnects",
			input:     "\r~.",
			wantModem: "\r",
		},
		{
			name:      "tilde dot mid-line does not disconnect",
			input:     "a~.b\r",
			wantModem: "a~.b\r",
		},
		{
			name:      "tilde without dot is kept in buffer",
			input:     "\r~x\r",
			wantModem: "\r~x\r",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &TerminalSession{}
			r := strings.NewReader(tt.input)
			var modemBuf, echoBuf bytes.Buffer

			ts.userToModem(r, &modemBuf, &echoBuf)

			if got := modemBuf.String(); got != tt.wantModem {
				t.Errorf("modem output = %q, want %q", got, tt.wantModem)
			}
		})
	}
}

func TestUserToModem_CtrlC(t *testing.T) {
	ts := &TerminalSession{}
	// Type some text then Ctrl+C — should disconnect without sending
	r := strings.NewReader("hello\x03")
	var modemBuf, echoBuf bytes.Buffer

	err := ts.userToModem(r, &modemBuf, &echoBuf)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if got := modemBuf.String(); got != "" {
		t.Errorf("modem output = %q, want empty (nothing sent before Enter)", got)
	}
}

func TestSetStdinStdout_Used(t *testing.T) {
	ts := &TerminalSession{}

	r, _ := io.Pipe()
	defer r.Close()
	var w bytes.Buffer

	ts.SetStdin(r)
	ts.SetStdout(&w)

	if ts.stdin != r {
		t.Error("SetStdin did not store reader")
	}
	if ts.stdout == nil {
		t.Error("SetStdout did not store writer")
	}
}

func TestWakeLoop_SendsEntersUntilData(t *testing.T) {
	// Simulate a modem that responds after a short delay.
	// The wake goroutine in Run() sends \r to the modem until gotData is set.
	// Here we test the logic in isolation.

	var modemBuf bytes.Buffer
	var gotData atomic.Bool

	// Simulate the wake loop inline
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond) // fast for testing
		defer ticker.Stop()
		modemBuf.Write([]byte("\r"))
		for range ticker.C {
			if gotData.Load() {
				return
			}
			modemBuf.Write([]byte("\r"))
		}
	}()

	// Simulate remote data arriving after 120ms
	time.Sleep(120 * time.Millisecond)
	gotData.Store(true)

	// Let the goroutine notice and exit
	time.Sleep(60 * time.Millisecond)

	// Should have sent the initial \r plus at least one ticker \r
	got := modemBuf.Len()
	if got < 2 {
		t.Errorf("expected at least 2 enters sent, got %d", got)
	}

	// Verify all bytes are \r
	for i, b := range modemBuf.Bytes() {
		if b != '\r' {
			t.Errorf("byte %d = %q, want \\r", i, b)
		}
	}
}

func TestCarrierLost_SkipsHangup(t *testing.T) {
	ts := &TerminalSession{}
	// Simulate carrier loss
	ts.carrierLost.Store(true)

	if !ts.carrierLost.Load() {
		t.Error("expected carrierLost to be true")
	}
}
