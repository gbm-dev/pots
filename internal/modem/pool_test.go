package modem

import (
	"testing"
)

func TestPoolAcquireRelease(t *testing.T) {
	p := &Pool{devices: map[string]string{
		"/dev/ttyIAX0": "",
		"/dev/ttyIAX1": "",
	}}

	dev1, err := p.Acquire("site-a")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if dev1 == "" {
		t.Fatal("expected non-empty device")
	}

	dev2, err := p.Acquire("site-b")
	if err != nil {
		t.Fatalf("Acquire second: %v", err)
	}
	if dev2 == dev1 {
		t.Error("expected different device")
	}

	// All in use
	_, err = p.Acquire("site-c")
	if err == nil {
		t.Error("expected error when all devices in use")
	}

	// Release one
	p.Release(dev1)
	dev3, err := p.Acquire("site-a")
	if err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
	if dev3 != dev1 {
		t.Errorf("expected released device %q, got %q", dev1, dev3)
	}
}

func TestPoolAvailable(t *testing.T) {
	p := &Pool{devices: map[string]string{
		"/dev/ttyIAX0": "",
		"/dev/ttyIAX1": "",
		"/dev/ttyIAX2": "some-site",
	}}

	free, total := p.Available()
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if free != 2 {
		t.Errorf("free = %d, want 2", free)
	}
}

func TestPoolActiveSites(t *testing.T) {
	p := &Pool{devices: map[string]string{
		"/dev/ttyIAX0": "site-a",
		"/dev/ttyIAX1": "",
		"/dev/ttyIAX2": "site-b",
	}}

	active := p.ActiveSites()
	if len(active) != 2 {
		t.Errorf("expected 2 active sites, got %d", len(active))
	}
	if !active["site-a"] || !active["site-b"] {
		t.Errorf("unexpected active sites: %v", active)
	}
}

func TestNewPoolWithRealFiles(t *testing.T) {
	// NewPool checks /dev/ttyIAX* which won't exist in tests
	p := NewPool(4)
	_, total := p.Available()
	if total != 0 {
		t.Logf("found %d real devices (expected on dev machine)", total)
	}
}
