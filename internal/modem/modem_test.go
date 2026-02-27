package modem

import (
	"testing"
)

func TestDialResultString(t *testing.T) {
	tests := []struct {
		r    DialResult
		want string
	}{
		{ResultConnect, "CONNECT"},
		{ResultBusy, "BUSY"},
		{ResultNoCarrier, "NO CARRIER"},
		{ResultNoDialtone, "NO DIALTONE"},
		{ResultError, "ERROR"},
		{ResultTimeout, "TIMEOUT"},
	}
	for _, tt := range tests {
		if got := tt.r.String(); got != tt.want {
			t.Errorf("DialResult(%d).String() = %q, want %q", tt.r, got, tt.want)
		}
	}
}
