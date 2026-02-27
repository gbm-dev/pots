package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gbm-dev/pots/internal/auth"
)

const minPasswordLen = 8

// PasswordModel is the first-login password change form.
type PasswordModel struct {
	newPassword     textinput.Model
	confirmPassword textinput.Model
	focusIndex      int
	err             string
	username        string
	store           auth.UserStore
}

// NewPasswordModel creates a password change form.
func NewPasswordModel(username string, store auth.UserStore) PasswordModel {
	np := textinput.New()
	np.Placeholder = "New password (min 8 chars)"
	np.EchoMode = textinput.EchoPassword
	np.EchoCharacter = '*'
	np.Focus()

	cp := textinput.New()
	cp.Placeholder = "Confirm password"
	cp.EchoMode = textinput.EchoPassword
	cp.EchoCharacter = '*'

	return PasswordModel{
		newPassword:     np,
		confirmPassword: cp,
		username:        username,
		store:           store,
	}
}

func (m PasswordModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m PasswordModel) Update(msg tea.Msg) (PasswordModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab", "down", "up":
			if m.focusIndex == 0 {
				m.focusIndex = 1
				m.newPassword.Blur()
				m.confirmPassword.Focus()
			} else {
				m.focusIndex = 0
				m.confirmPassword.Blur()
				m.newPassword.Focus()
			}
			return m, nil

		case "enter":
			if m.focusIndex == 0 {
				m.focusIndex = 1
				m.newPassword.Blur()
				m.confirmPassword.Focus()
				return m, nil
			}
			return m, m.submit()

		case "ctrl+c":
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	if m.focusIndex == 0 {
		m.newPassword, cmd = m.newPassword.Update(msg)
	} else {
		m.confirmPassword, cmd = m.confirmPassword.Update(msg)
	}
	return m, cmd
}

func (m PasswordModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Password Change Required"))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("  You must set a new password before continuing."))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  New password:     %s\n", m.newPassword.View()))
	b.WriteString(fmt.Sprintf("  Confirm password: %s\n", m.confirmPassword.View()))

	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("  " + m.err))
	}

	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("  Tab to switch fields | Enter to submit"))

	return boxStyle.Render(b.String())
}

func (m PasswordModel) submit() tea.Cmd {
	return func() tea.Msg {
		pw := m.newPassword.Value()
		confirm := m.confirmPassword.Value()

		if len(pw) < minPasswordLen {
			return ErrorMsg{Err: fmt.Errorf("password must be at least %d characters", minPasswordLen)}
		}
		if pw != confirm {
			return ErrorMsg{Err: fmt.Errorf("passwords do not match")}
		}
		if err := m.store.SetPassword(m.username, pw); err != nil {
			return ErrorMsg{Err: fmt.Errorf("setting password: %w", err)}
		}
		return PasswordChangedMsg{}
	}
}
