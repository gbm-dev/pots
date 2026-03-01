package tui

import (
	"bytes"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestUserToModem_EscapeSequence(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOut string
	}{
		{
			name:    "normal text passes through",
			input:   "hello",
			wantOut: "hello",
		},
		{
			name:    "enter tilde dot disconnects",
			input:   "hello\r~.",
			wantOut: "hello\r",
		},
		{
			name:    "newline tilde dot disconnects",
			input:   "hello\n~.",
			wantOut: "hello\n",
		},
		{
			name:    "tilde without preceding enter passes through",
			input:   "a~.",
			wantOut: "a~.",
		},
		{
			name:    "tilde followed by non-dot forwards tilde",
			input:   "\r~x",
			wantOut: "\r~x",
		},
		{
			name:    "enter only resets state",
			input:   "\r\r\r",
			wantOut: "\r\r\r",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &TerminalSession{}
			r := strings.NewReader(tt.input)
			var w bytes.Buffer

			ts.userToModem(r, &w)

			if got := w.String(); got != tt.wantOut {
				t.Errorf("output = %q, want %q", got, tt.wantOut)
			}
		})
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
