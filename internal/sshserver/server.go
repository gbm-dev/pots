package sshserver

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/gbm-dev/pots/internal/auth"
	"github.com/gbm-dev/pots/internal/config"
	"github.com/gbm-dev/pots/internal/modem"
	"github.com/gbm-dev/pots/internal/tui"
)

// Server wraps the Wish SSH server.
type Server struct {
	srv    *ssh.Server
	store  auth.UserStore
	pool   *modem.Pool
	sites  []config.Site
	logDir string
}

// New creates a new SSH server.
func New(cfg config.AppConfig, store auth.UserStore, pool *modem.Pool, sites []config.Site) (*Server, error) {
	s := &Server{
		store:  store,
		pool:   pool,
		sites:  sites,
		logDir: cfg.LogDir,
	}

	// Ensure host key directory exists
	if err := os.MkdirAll(cfg.HostKeyDir, 0700); err != nil {
		return nil, fmt.Errorf("creating host key dir: %w", err)
	}

	hostKeyPath := filepath.Join(cfg.HostKeyDir, "ssh_host_ed25519_key")

	srv, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf(":%d", cfg.SSHPort)),
		wish.WithHostKeyPath(hostKeyPath),
		wish.WithPasswordAuth(s.passwordAuth),
		wish.WithMiddleware(
			bubbletea.Middleware(s.teaHandler),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating SSH server: %w", err)
	}

	s.srv = srv
	return s, nil
}

// ListenAndServe starts the SSH server.
func (s *Server) ListenAndServe() error {
	log.Printf("SSH server listening on %s", s.srv.Addr)
	return s.srv.ListenAndServe()
}

// Shutdown gracefully stops the SSH server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// passwordAuth validates credentials against the user store.
func (s *Server) passwordAuth(ctx ssh.Context, password string) bool {
	username := ctx.User()
	ok, err := s.store.Authenticate(username, password)
	if err != nil {
		log.Printf("auth error for %q: %v", username, err)
		return false
	}
	if ok {
		s.store.UpdateLastLogin(username)
		log.Printf("user %q authenticated from %s", username, ctx.RemoteAddr())
	}
	return ok
}

// teaHandler creates a Bubble Tea program for each SSH session.
func (s *Server) teaHandler(sshSession ssh.Session) (tea.Model, []tea.ProgramOption) {
	username := sshSession.User()

	forceChange, err := s.store.MustChangePassword(username)
	if err != nil {
		log.Printf("error checking password change for %q: %v", username, err)
		forceChange = false
	}

	model := tui.New(username, s.sites, s.pool, s.store, s.logDir, forceChange)

	return model, []tea.ProgramOption{tea.WithAltScreen()}
}
