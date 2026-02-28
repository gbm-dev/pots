package config

import (
	"strings"
	"testing"
)

func TestParseSites(t *testing.T) {
	input := `# Header comment
# Format: name|phone|description|baud

2broadway|14105551234|2 Broadway Terminal Server|19200
router1|13125559876|Chicago Core Router|9600

# Another comment

nyc-switch|12125551111|NYC Core Switch Stack|38400
`
	sites, err := ParseSites(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sites) != 3 {
		t.Fatalf("expected 3 sites, got %d", len(sites))
	}

	tests := []struct {
		name, phone, desc string
		baud              int
	}{
		{"2broadway", "14105551234", "2 Broadway Terminal Server", 19200},
		{"router1", "13125559876", "Chicago Core Router", 9600},
		{"nyc-switch", "12125551111", "NYC Core Switch Stack", 38400},
	}
	for i, tt := range tests {
		s := sites[i]
		if s.Name != tt.name {
			t.Errorf("site %d: name = %q, want %q", i, s.Name, tt.name)
		}
		if s.Phone != tt.phone {
			t.Errorf("site %d: phone = %q, want %q", i, s.Phone, tt.phone)
		}
		if s.Description != tt.desc {
			t.Errorf("site %d: desc = %q, want %q", i, s.Description, tt.desc)
		}
		if s.BaudRate != tt.baud {
			t.Errorf("site %d: baud = %d, want %d", i, s.BaudRate, tt.baud)
		}
	}
}

func TestParseSitesFile(t *testing.T) {
	sites, err := ParseSitesFile("../../tests/fixtures/oob-sites.conf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sites) != 3 {
		t.Fatalf("expected 3 sites, got %d", len(sites))
	}
	if sites[0].Name != "2broadway" {
		t.Errorf("first site name = %q, want %q", sites[0].Name, "2broadway")
	}
}

func TestParseSitesEmpty(t *testing.T) {
	sites, err := ParseSites(strings.NewReader("# only comments\n\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sites) != 0 {
		t.Errorf("expected 0 sites, got %d", len(sites))
	}
}

func TestParseSitesInvalidFields(t *testing.T) {
	_, err := ParseSites(strings.NewReader("bad|line\n"))
	if err == nil {
		t.Fatal("expected error for invalid field count")
	}
}

func TestParseSitesInvalidBaud(t *testing.T) {
	_, err := ParseSites(strings.NewReader("name|phone|desc|notanumber\n"))
	if err == nil {
		t.Fatal("expected error for invalid baud rate")
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Test defaults
	cfg := LoadFromEnv()
	if cfg.SSHPort != 2222 {
		t.Errorf("default SSHPort = %d, want 2222", cfg.SSHPort)
	}
	if cfg.ModemCount != 8 {
		t.Errorf("default ModemCount = %d, want 8", cfg.ModemCount)
	}
	if cfg.ModemDevicePrefix != "/dev/ttyIAX" {
		t.Errorf("default ModemDevicePrefix = %q, want /dev/ttyIAX", cfg.ModemDevicePrefix)
	}

	// Test override
	t.Setenv("SSH_PORT", "3333")
	t.Setenv("MODEM_COUNT", "4")
	t.Setenv("MODEM_DEVICE_PREFIX", "/tmp/ttySL")
	cfg = LoadFromEnv()
	if cfg.SSHPort != 3333 {
		t.Errorf("SSHPort = %d, want 3333", cfg.SSHPort)
	}
	if cfg.ModemCount != 4 {
		t.Errorf("ModemCount = %d, want 4", cfg.ModemCount)
	}
	if cfg.ModemDevicePrefix != "/tmp/ttySL" {
		t.Errorf("ModemDevicePrefix = %q, want /tmp/ttySL", cfg.ModemDevicePrefix)
	}
}
