package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// SIPStatus represents the Telnyx trunk registration state.
type SIPStatus int

const (
	SIPUnknown SIPStatus = iota
	SIPRegistered
	SIPUnregistered
)

// SIPInfo holds the parsed SIP registration details.
type SIPInfo struct {
	Status SIPStatus
	Trunk  string // e.g. "telnyx-out"
	Server string // e.g. "sip.telnyx.com"
	Expiry string // e.g. "3434s"
}

func (s SIPInfo) String() string {
	switch s.Status {
	case SIPRegistered:
		if s.Trunk != "" {
			return "SIP: " + s.Trunk + " registered"
		}
		return "SIP: registered"
	case SIPUnregistered:
		return "SIP: not registered"
	default:
		if s.Trunk == "dmodem" {
			return "SIP: managed by dmodem"
		}
		return "SIP: checking..."
	}
}

// sipStatusMsg carries the result of a SIP registration check.
type sipStatusMsg SIPInfo

// checkSIPStatus reports SIP state for the current backend.
func checkSIPStatus() tea.Msg {
	// d-modem handles SIP directly and does not expose Asterisk registration state.
	return sipStatusMsg(SIPInfo{Status: SIPUnknown, Trunk: "dmodem"})
}

// sipTickMsg triggers periodic SIP status checks.
type sipTickMsg struct{}

// sipTick returns a command that ticks every 30 seconds.
func sipTick() tea.Cmd {
	return tea.Tick(30*time.Second, func(time.Time) tea.Msg {
		return sipTickMsg{}
	})
}
