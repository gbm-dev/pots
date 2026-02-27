package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gbm-dev/pots/internal/modem"
)

// State represents the current TUI state.
type State int

const (
	StatePasswordChange State = iota
	StateMenu
	StateDialing
	StateConnected
)

// Messages passed between TUI components.

// DialRequestMsg is sent when the user selects a site.
type DialRequestMsg struct {
	SiteIndex int
}

// ModemAcquiredMsg is sent when a modem device is acquired from the pool.
type ModemAcquiredMsg struct {
	Device string
}

// ModemResetMsg is sent after ATZ succeeds.
type ModemResetMsg struct{}

// DialResultMsg is sent after the dial attempt completes.
type DialResultMsg struct {
	Result DialResult
	Modem  *modem.Modem
	Device string
}

// DialResult mirrors modem.DialResult for the TUI layer.
type DialResult = modem.DialResult

// DisconnectMsg is sent when the user disconnects from a session.
type DisconnectMsg struct{}

// PasswordChangedMsg is sent after a successful password change.
type PasswordChangedMsg struct{}

// ErrorMsg wraps an error for display.
type ErrorMsg struct {
	Err     error
	Context string
}

// TerminalDoneMsg is sent when tea.Exec returns from terminal mode.
type TerminalDoneMsg struct {
	Err error
}

// statusMsg is used internally to update the dialing status text.
type statusMsg string

func updateStatus(msg string) tea.Cmd {
	return func() tea.Msg {
		return statusMsg(msg)
	}
}
