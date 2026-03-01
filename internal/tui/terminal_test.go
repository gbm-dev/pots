package tui

import (
	"bytes"
	"io"
	"strings"
	"testing"
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
