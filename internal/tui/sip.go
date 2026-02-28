package tui

import (
	"os/exec"
	"strings"
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

func (s SIPStatus) String() string {
	switch s {
	case SIPRegistered:
		return "SIP: registered"
	case SIPUnregistered:
		return "SIP: unregistered"
	default:
		return "SIP: checking..."
	}
}

// sipStatusMsg carries the result of a SIP registration check.
type sipStatusMsg SIPStatus

// checkSIPStatus runs `asterisk -rx "pjsip show registrations"` and parses output.
func checkSIPStatus() tea.Msg {
	out, err := exec.Command("asterisk", "-rx", "pjsip show registrations").CombinedOutput()
	if err != nil {
		return sipStatusMsg(SIPUnregistered)
	}
	if strings.Contains(strings.ToLower(string(out)), "registered") {
		return sipStatusMsg(SIPRegistered)
	}
	return sipStatusMsg(SIPUnregistered)
}

// sipTickMsg triggers periodic SIP status checks.
type sipTickMsg struct{}

// sipTick returns a command that ticks every 30 seconds.
func sipTick() tea.Cmd {
	return tea.Tick(30*time.Second, func(time.Time) tea.Msg {
		return sipTickMsg{}
	})
}
