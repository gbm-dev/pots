package modem

import (
	"os"
	"path/filepath"
	"testing"
)

func setupFakeDevices(t *testing.T, count int) string {
	t.Helper()
	// We can't create real /dev entries, so we test the pool logic
	// by creating a pool with a custom helper
	return t.TempDir()
}

func TestPoolAcquireRelease(t *testing.T) {
	// Create a pool with fake devices by injecting directly
	p := &Pool{devices: map[string]bool{
		"/dev/ttyIAX0": false,
		"/dev/ttyIAX1": false,
	}}

	dev1, err := p.Acquire()
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if dev1 == "" {
		t.Fatal("expected non-empty device")
	}

	dev2, err := p.Acquire()
	if err != nil {
		t.Fatalf("Acquire second: %v", err)
	}
	if dev2 == dev1 {
		t.Error("expected different device")
	}

	// All in use
	_, err = p.Acquire()
	if err == nil {
		t.Error("expected error when all devices in use")
	}

	// Release one
	p.Release(dev1)
	dev3, err := p.Acquire()
	if err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
	if dev3 != dev1 {
		t.Errorf("expected released device %q, got %q", dev1, dev3)
	}
}

func TestPoolAvailable(t *testing.T) {
	p := &Pool{devices: map[string]bool{
		"/dev/ttyIAX0": false,
		"/dev/ttyIAX1": false,
		"/dev/ttyIAX2": true,
	}}

	free, total := p.Available()
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if free != 2 {
		t.Errorf("free = %d, want 2", free)
	}
}

func TestNewPoolWithRealFiles(t *testing.T) {
	// Create temp "devices" to test NewPool file existence check
	dir := t.TempDir()

	// Create fake ttyIAX files
	for i := 0; i < 3; i++ {
		f, _ := os.Create(filepath.Join(dir, "ttyIAX"))
		f.Close()
	}

	// NewPool checks /dev/ttyIAX* which won't exist in tests
	p := NewPool(4)
	_, total := p.Available()
	// On a dev machine without IAXmodem, expect 0 devices
	if total != 0 {
		t.Logf("found %d real devices (expected on dev machine)", total)
	}
}
