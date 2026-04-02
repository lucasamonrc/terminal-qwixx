package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/lucasacastro/qwixx/lobby"
	"github.com/lucasacastro/qwixx/tui"
)

var playerCounter atomic.Int64

// Server is the SSH game server.
type Server struct {
	host  string
	port  int
	lobby *lobby.Lobby
}

// New creates a new game server.
func New(host string, port int) *Server {
	return &Server{
		host:  host,
		port:  port,
		lobby: lobby.NewLobby(),
	}
}

// Start starts the SSH server and blocks until shutdown.
func (s *Server) Start() error {
	srv, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%d", s.host, s.port)),
		wish.WithHostKeyPath(".ssh/host_key"),
		wish.WithMiddleware(
			s.cleanupMiddleware(),
			bm.Middleware(s.teaHandler),
		),
	)
	if err != nil {
		return fmt.Errorf("could not create server: %w", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Starting Qwixx server on %s:%d", s.host, s.port)
	log.Printf("Players can connect with: ssh -p %d %s", s.port, s.host)

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return srv.Shutdown(ctx)
}

func (s *Server) teaHandler(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
	playerID := sess.Context().Value(playerIDKey).(string)

	model := tui.NewAppModel(playerID, s.lobby)

	return model, []tea.ProgramOption{tea.WithAltScreen()}
}

// cleanupMiddleware returns a middleware that cleans up player state on disconnect.
func (s *Server) cleanupMiddleware() wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			playerID := fmt.Sprintf("player_%d_%d", time.Now().UnixNano(), playerCounter.Add(1))
			sess.Context().SetValue(playerIDKey, playerID)

			// Run the inner handler (bubbletea)
			next(sess)

			// Session ended -- clean up
			// We need to figure out the room code. Since we stored the playerID
			// in context, we can search the lobby for this player.
			s.cleanupPlayer(playerID)
		}
	}
}

// cleanupPlayer removes a player from any room they're in.
func (s *Server) cleanupPlayer(playerID string) {
	// We need to find which room this player is in.
	// This is a linear scan but rooms are few.
	s.lobby.RemovePlayerByID(playerID)
}

// contextKey is a custom type for context keys.
type contextKey string

const playerIDKey contextKey = "playerID"
