package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gbm-dev/pots/internal/config"
	"github.com/gbm-dev/pots/internal/modem"
)

const dialTimeout = 60 * time.Second
const resetTimeout = 5 * time.Second

// DialingModel shows connection progress with a spinner.
type DialingModel struct {
	spinner    spinner.Model
	site       config.Site
	status     string
	device     string
	transcript string
	err        error
	done       bool
	pool       *modem.Pool
}

// NewDialingModel creates a dialing view for the given site.
func NewDialingModel(site config.Site, pool *modem.Pool) DialingModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = warningStyle
	return DialingModel{
		spinner: s,
		site:    site,
		status:  "Acquiring modem port...",
		pool:    pool,
	}
}

func (m DialingModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.acquireAndDial())
}

func (m DialingModel) Update(msg tea.Msg) (DialingModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case statusMsg:
		m.status = string(msg)
		return m, nil

	case DialResultMsg:
		if msg.Result == modem.ResultConnect {
			m.status = successStyle.Render("CONNECTED")
			m.device = msg.Device
			return m, nil
		}
		m.done = true
		m.device = msg.Device
		m.transcript = msg.Transcript
		m.err = fmt.Errorf("%s", msg.Result)
		return m, nil

	case ErrorMsg:
		m.done = true
		m.err = msg.Err
		return m, nil

	case tea.KeyMsg:
		if m.done && msg.String() == "enter" {
			return m, func() tea.Msg { return DisconnectMsg{} }
		}
		if msg.String() == "ctrl+c" {
			return m, func() tea.Msg { return DisconnectMsg{} }
		}
	}

	return m, nil
}

func (m DialingModel) View() string {
	header := titleStyle.Render(fmt.Sprintf("Connecting to %s", m.site.Name))

	details := fmt.Sprintf(
		"  Phone:  %s\n  Baud:   %d\n  Device: %s",
		m.site.Phone, m.site.BaudRate, m.deviceDisplay())

	if m.err != nil {
		view := header + "\n\n" + details + "\n\n" +
			errorStyle.Render(fmt.Sprintf("  Error: %s", m.err))
		if m.transcript != "" {
			view += "\n\n" + labelStyle.Render("  AT log:") + "\n" +
				lipgloss.NewStyle().Foreground(colorMuted).PaddingLeft(4).Render(m.transcript)
		}
		view += "\n\n" + labelStyle.Render("  Press Enter to return to menu")
		return boxStyle.Render(view)
	}

	return boxStyle.Render(
		header + "\n\n" + details + "\n\n" +
			fmt.Sprintf("  %s %s", m.spinner.View(), m.status),
	)
}

func (m DialingModel) deviceDisplay() string {
	if m.device == "" {
		return "—"
	}
	return m.device
}

// acquireAndDial runs the modem acquire → reset → dial sequence,
// sending status updates back to the TUI at each step.
func (m DialingModel) acquireAndDial() tea.Cmd {
	return func() tea.Msg {
		// Step 1: Acquire port
		dev, err := m.pool.Acquire(m.site.Name)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("no free modem ports available"), Context: "acquire"}
		}

		// Step 2: Open device
		mdm, err := modem.Open(dev)
		if err != nil {
			m.pool.Release(dev)
			return ErrorMsg{Err: fmt.Errorf("failed to open %s: %w", dev, err), Context: "open"}
		}

		// Step 3: Reset modem (ATZ)
		if err := mdm.Reset(resetTimeout); err != nil {
			mdm.Close()
			m.pool.Release(dev)
			return ErrorMsg{Err: fmt.Errorf("modem reset failed: %w", err), Context: "reset"}
		}

		// Step 4: Dial
		resp, err := mdm.Dial(m.site.Phone, dialTimeout)
		if err != nil {
			mdm.Close()
			m.pool.Release(dev)
			return ErrorMsg{Err: fmt.Errorf("dial error: %w", err), Context: "dial"}
		}

		if resp.Result != modem.ResultConnect {
			mdm.Close()
			m.pool.Release(dev)
		}

		return DialResultMsg{Result: resp.Result, Transcript: resp.Transcript, Modem: mdm, Device: dev}
	}
}
