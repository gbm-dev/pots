package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gbm-dev/pots/internal/config"
	"github.com/gbm-dev/pots/internal/modem"
)

const dialTimeout = 60 * time.Second
const resetTimeout = 5 * time.Second

// DialingModel shows connection progress with a spinner.
type DialingModel struct {
	spinner spinner.Model
	site    config.Site
	status  string
	err     error
	done    bool
	pool    *modem.Pool
}

// NewDialingModel creates a dialing view for the given site.
func NewDialingModel(site config.Site, pool *modem.Pool) DialingModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = warningStyle
	return DialingModel{
		spinner: s,
		site:    site,
		status:  "Acquiring modem...",
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
			m.status = "CONNECTED!"
			return m, nil
		}
		m.done = true
		m.err = fmt.Errorf("dial failed: %s", msg.Result)
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
	if m.err != nil {
		return boxStyle.Render(
			errorStyle.Render("Connection Failed") + "\n\n" +
				fmt.Sprintf("  Site:  %s\n  Phone: %s\n  Error: %s\n\n",
					m.site.Name, m.site.Phone, m.err) +
				labelStyle.Render("  Press Enter to return to menu"),
		)
	}
	return boxStyle.Render(
		fmt.Sprintf("%s Connecting to %s\n\n  Phone: %s\n  Baud:  %d\n  %s",
			m.spinner.View(), m.site.Name, m.site.Phone, m.site.BaudRate, m.status),
	)
}

// acquireAndDial runs the modem acquire → reset → dial sequence.
func (m DialingModel) acquireAndDial() tea.Cmd {
	return func() tea.Msg {
		dev, err := m.pool.Acquire()
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("no free modems: %w", err), Context: "acquire"}
		}

		mdm, err := modem.Open(dev)
		if err != nil {
			m.pool.Release(dev)
			return ErrorMsg{Err: fmt.Errorf("opening %s: %w", dev, err), Context: "open"}
		}

		// Send status updates via a goroutine-safe channel approach
		// (status updates arrive as tea.Msg through the return)

		if err := mdm.Reset(resetTimeout); err != nil {
			mdm.Close()
			m.pool.Release(dev)
			return ErrorMsg{Err: fmt.Errorf("modem reset: %w", err), Context: "reset"}
		}

		result, err := mdm.Dial(m.site.Phone, dialTimeout)
		if err != nil {
			mdm.Close()
			m.pool.Release(dev)
			return ErrorMsg{Err: fmt.Errorf("dial error: %w", err), Context: "dial"}
		}

		if result != modem.ResultConnect {
			mdm.Close()
			m.pool.Release(dev)
		}

		return DialResultMsg{Result: result, Modem: mdm, Device: dev}
	}
}
