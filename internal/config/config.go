package config

import (
	"os"
	"strconv"
)

// AppConfig holds application configuration loaded from environment variables.
type AppConfig struct {
	SSHPort           int
	ModemCount        int
	ModemDevicePrefix string
	SitesPath         string
	UserDataDir       string
	LogDir            string
	HostKeyDir        string
}

// LoadFromEnv loads configuration from environment variables with defaults.
func LoadFromEnv() AppConfig {
	return AppConfig{
		SSHPort:           envInt("SSH_PORT", 2222),
		ModemCount:        envInt("MODEM_COUNT", 8),
		ModemDevicePrefix: envStr("MODEM_DEVICE_PREFIX", "/dev/ttyIAX"),
		SitesPath:         envStr("SITES_PATH", "/etc/oob-sites.conf"),
		UserDataDir:       envStr("USER_DATA_DIR", "/data/users"),
		LogDir:            envStr("LOG_DIR", "/var/log/oob-sessions"),
		HostKeyDir:        envStr("HOST_KEY_DIR", "/data/users/ssh_host_keys"),
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
