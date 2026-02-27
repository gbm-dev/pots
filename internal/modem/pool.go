package modem

import (
	"fmt"
	"os"
	"sync"
)

// Pool manages a set of IAXmodem PTY devices.
type Pool struct {
	mu      sync.Mutex
	devices map[string]bool // device path → in-use
}

// NewPool creates a pool with modemCount devices (/dev/ttyIAX0 through ttyIAX{n-1}).
// Only devices that exist on the filesystem are added to the pool.
func NewPool(modemCount int) *Pool {
	devices := make(map[string]bool)
	for i := 0; i < modemCount; i++ {
		dev := fmt.Sprintf("/dev/ttyIAX%d", i)
		if _, err := os.Stat(dev); err == nil {
			devices[dev] = false
		}
	}
	return &Pool{devices: devices}
}

// Acquire returns the first available device, or an error if none are free.
func (p *Pool) Acquire() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for dev, inUse := range p.devices {
		if !inUse {
			p.devices[dev] = true
			return dev, nil
		}
	}
	return "", fmt.Errorf("no free modem devices")
}

// Release returns a device to the pool.
func (p *Pool) Release(dev string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.devices[dev] = false
}

// Available returns the count of free and total devices.
func (p *Pool) Available() (free, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	total = len(p.devices)
	for _, inUse := range p.devices {
		if !inUse {
			free++
		}
	}
	return
}
