package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gbm-dev/pots/internal/config"
)

// siteItem implements list.Item for the site selector.
type siteItem struct {
	site  config.Site
	index int
}

func (i siteItem) Title() string       { return i.site.Name }
func (i siteItem) Description() string { return fmt.Sprintf("%s (%d baud)", i.site.Description, i.site.BaudRate) }
func (i siteItem) FilterValue() string { return i.site.Name + " " + i.site.Description }

// siteDelegate renders site items in the list.
type siteDelegate struct{}

func (d siteDelegate) Height() int                             { return 2 }
func (d siteDelegate) Spacing() int                            { return 0 }
func (d siteDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d siteDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	si, ok := item.(siteItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	var nameStyle, descStyle lipgloss.Style
	if isSelected {
		nameStyle = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).PaddingLeft(2)
		descStyle = lipgloss.NewStyle().Foreground(colorSecondary).PaddingLeft(4)
	} else {
		nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC")).PaddingLeft(2)
		descStyle = lipgloss.NewStyle().Foreground(colorMuted).PaddingLeft(4)
	}

	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	fmt.Fprintf(w, "%s%s\n%s\n",
		cursor,
		nameStyle.Render(si.site.Name),
		descStyle.Render(si.Description()),
	)
}

// MenuModel is the site selection view.
type MenuModel struct {
	list     list.Model
	sites    []config.Site
	username string
	freePorts int
	totalPorts int
}

// NewMenuModel creates the site selection menu.
func NewMenuModel(sites []config.Site, username string, free, total int, width, height int) MenuModel {
	items := make([]list.Item, len(sites))
	for i, s := range sites {
		items[i] = siteItem{site: s, index: i}
	}

	l := list.New(items, siteDelegate{}, width, height-6)
	l.Title = "OOB Console Hub"
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	return MenuModel{
		list:       l,
		sites:      sites,
		username:   username,
		freePorts:  free,
		totalPorts: total,
	}
}

func (m MenuModel) Init() tea.Cmd { return nil }

func (m MenuModel) Update(msg tea.Msg) (MenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if i, ok := m.list.SelectedItem().(siteItem); ok {
				return m, func() tea.Msg { return DialRequestMsg{SiteIndex: i.index} }
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-6)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m MenuModel) View() string {
	footer := statusBarStyle.Render(
		fmt.Sprintf("  %s | %d/%d ports available | q quit | enter connect",
			m.username, m.freePorts, m.totalPorts))
	return m.list.View() + "\n" + footer
}

// UpdatePorts refreshes the port availability counts.
func (m *MenuModel) UpdatePorts(free, total int) {
	m.freePorts = free
	m.totalPorts = total
}
