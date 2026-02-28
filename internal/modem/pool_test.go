package modem

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func testPool(t *testing.T, count int) (*Pool, []string) {
	t.Helper()
	dir := t.TempDir()
	devices := make(map[string]string)
	var paths []string
	for i := 0; i < count; i++ {
		path := filepath.Join(dir, "ttyIAX")
		// Each needs a unique name
		path = filepath.Join(dir, fmt.Sprintf("ttyIAX%d", i))
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		devices[path] = ""
		paths = append(paths, path)
	}
	return &Pool{devices: devices}, paths
}

func TestPoolAcquireRelease(t *testing.T) {
	p, _ := testPool(t, 2)

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

func TestPoolAcquireRemovesMissing(t *testing.T) {
	dir := t.TempDir()
	missingPath := filepath.Join(dir, "ttyGONE")
	realPath := filepath.Join(dir, "ttyREAL")
	os.Create(realPath)

	p := &Pool{devices: map[string]string{
		missingPath: "",
		realPath:    "",
	}}

	dev, err := p.Acquire("test")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if dev != realPath {
		t.Errorf("expected %q, got %q", realPath, dev)
	}

	// Missing device should have been pruned during Acquire
	// Try to acquire again — only the one we got should be in pool (and it's in use)
	_, err = p.Acquire("test2")
	if err == nil {
		t.Error("expected error — missing device should be pruned and real one is in use")
	}
}

func TestPoolAvailable(t *testing.T) {
	p, paths := testPool(t, 3)
	// Mark one as in-use
	p.devices[paths[2]] = "some-site"

	free, total := p.Available()
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if free != 2 {
		t.Errorf("free = %d, want 2", free)
	}
}

func TestPoolActiveSites(t *testing.T) {
	p, paths := testPool(t, 3)
	p.devices[paths[0]] = "site-a"
	p.devices[paths[2]] = "site-b"

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
	p := NewPool(4, "/dev/ttyIAX")
	_, total := p.Available()
	if total != 0 {
		t.Logf("found %d real devices (expected on dev machine)", total)
	}
}
