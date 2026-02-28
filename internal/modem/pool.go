package modem

import (
	"fmt"
	"os"
	"sync"
)

// Pool manages a set of IAXmodem PTY devices.
type Pool struct {
	mu      sync.Mutex
	devices map[string]string // device path → site name ("" = free)
}

// NewPool creates a pool with modemCount devices (/dev/ttyIAX0 through ttyIAX{n-1}).
// Only devices that exist on the filesystem are added to the pool.
func NewPool(modemCount int) *Pool {
	devices := make(map[string]string)
	for i := 0; i < modemCount; i++ {
		dev := fmt.Sprintf("/dev/ttyIAX%d", i)
		if _, err := os.Stat(dev); err == nil {
			devices[dev] = ""
		}
	}
	return &Pool{devices: devices}
}

// Acquire returns the first available device that exists on the filesystem,
// or an error if none are free.
func (p *Pool) Acquire(siteName string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for dev, site := range p.devices {
		if site == "" {
			// Verify device still exists before handing it out
			if _, err := os.Stat(dev); err != nil {
				delete(p.devices, dev)
				continue
			}
			p.devices[dev] = siteName
			return dev, nil
		}
	}
	return "", fmt.Errorf("no free modem devices")
}

// Release returns a device to the pool.
func (p *Pool) Release(dev string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.devices[dev] = ""
}

// Available returns the count of free and total devices.
func (p *Pool) Available() (free, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	total = len(p.devices)
	for _, site := range p.devices {
		if site == "" {
			free++
		}
	}
	return
}

// ActiveSites returns the set of site names currently connected.
func (p *Pool) ActiveSites() map[string]bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	active := make(map[string]bool)
	for _, site := range p.devices {
		if site != "" {
			active[site] = true
		}
	}
	return active
}
