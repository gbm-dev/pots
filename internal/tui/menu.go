package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gbm-dev/pots/internal/config"
	"github.com/gbm-dev/pots/internal/modem"
)

// siteItem implements list.Item for the site selector.
type siteItem struct {
	site   config.Site
	index  int
	active bool // currently connected by someone
}

func (i siteItem) Title() string       { return i.site.Name }
func (i siteItem) Description() string { return i.site.Description }
func (i siteItem) FilterValue() string { return i.site.Name + " " + i.site.Description }

// siteDelegate renders site items in the list.
type siteDelegate struct{}

func (d siteDelegate) Height() int                             { return 1 }
func (d siteDelegate) Spacing() int                            { return 0 }
func (d siteDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d siteDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	si, ok := item.(siteItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	// Status indicator
	status := "  "
	if si.active {
		status = lipgloss.NewStyle().Foreground(colorSuccess).Render("● ")
	}

	// Cursor
	cursor := "  "
	if isSelected {
		cursor = lipgloss.NewStyle().Foreground(colorPrimary).Render("> ")
	}

	// Name
	var nameStyle lipgloss.Style
	if isSelected {
		nameStyle = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	} else {
		nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	}

	// Description + baud — dimmed, separated
	detail := lipgloss.NewStyle().Foreground(colorMuted).Render(
		fmt.Sprintf(" — %s (%d baud)", si.site.Description, si.site.BaudRate))

	fmt.Fprintf(w, "%s%s%s%s", cursor, status, nameStyle.Render(si.site.Name), detail)
}

// MenuModel is the site selection view.
type MenuModel struct {
	list       list.Model
	sites      []config.Site
	pool       *modem.Pool
	username   string
	freePorts  int
	totalPorts int
	sipStatus  SIPStatus
}

// NewMenuModel creates the site selection menu.
func NewMenuModel(sites []config.Site, username string, pool *modem.Pool, width, height int) MenuModel {
	free, total := pool.Available()
	active := pool.ActiveSites()

	items := make([]list.Item, len(sites))
	for i, s := range sites {
		items[i] = siteItem{site: s, index: i, active: active[s.Name]}
	}

	l := list.New(items, siteDelegate{}, width, height-4)
	l.Title = "OOB Console Hub"
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	return MenuModel{
		list:       l,
		sites:      sites,
		pool:       pool,
		username:   username,
		freePorts:  free,
		totalPorts: total,
	}
}

func (m MenuModel) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg { return checkSIPStatus() },
		sipTick(),
	)
}

func (m MenuModel) Update(msg tea.Msg) (MenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept keys while filtering
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "enter":
			if i, ok := m.list.SelectedItem().(siteItem); ok {
				return m, func() tea.Msg { return DialRequestMsg{SiteIndex: i.index} }
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-4)
	case sipStatusMsg:
		m.sipStatus = SIPStatus(msg)
		return m, nil
	case sipTickMsg:
		return m, tea.Batch(
			func() tea.Msg { return checkSIPStatus() },
			sipTick(),
		)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m MenuModel) View() string {
	// Status bar
	var parts []string

	// SIP status with color
	switch m.sipStatus {
	case SIPRegistered:
		parts = append(parts, successStyle.Render("● SIP registered"))
	case SIPUnregistered:
		parts = append(parts, errorStyle.Render("● SIP unregistered"))
	default:
		parts = append(parts, labelStyle.Render("○ SIP checking..."))
	}

	parts = append(parts, labelStyle.Render(fmt.Sprintf("%d/%d ports", m.freePorts, m.totalPorts)))
	parts = append(parts, labelStyle.Render(m.username))
	parts = append(parts, labelStyle.Render("enter connect · q quit"))

	footer := statusBarStyle.Render("  " + strings.Join(parts, "  │  "))
	return m.list.View() + "\n" + footer
}

// refreshItems updates the list items with current active status.
func (m *MenuModel) refreshItems() {
	active := m.pool.ActiveSites()
	m.freePorts, m.totalPorts = m.pool.Available()

	items := make([]list.Item, len(m.sites))
	for i, s := range m.sites {
		items[i] = siteItem{site: s, index: i, active: active[s.Name]}
	}
	m.list.SetItems(items)
}
