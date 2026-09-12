// Package transport carries the capture protocol over a WebSocket.
//
// The rules it enforces are not defined here: bot/pkg/protocol decides what a
// message means and what order messages may arrive in, and this package only
// moves bytes and closes connections that break those rules. Keeping the two
// apart is what let the protocol be tested without a socket, and it is what
// lets this package be tested without a game.
//
// Nothing here ever logs a credential. The requirement is absolute, and the one
// place a secret passes through is the authentication message, which is handed
// straight to the Authenticator and never printed.
package transport

import (
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/credential"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
	"github.com/gorilla/websocket"
)

// Authenticator decides whether a capture install may speak for a guild.
type Authenticator interface {
	// Authenticate resolves a presented credential token to a guild.
	Authenticate(token string) (string, error)
	// RedeemCode exchanges a typed pairing code for a credential and reports
	// the guild it belonged to.
	RedeemCode(typed string) (string, credential.Credential, error)
}

// Handler receives the messages a capture session sent and that the protocol
// accepted. It is an interface because applying them to a running game is the
// next phase; the transport is finished once a message arrives here.
type Handler interface {
	HandleCapture(guildID string, message protocol.Message) error
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(guildID string, message protocol.Message) error

func (f HandlerFunc) HandleCapture(guildID string, message protocol.Message) error {
	return f(guildID, message)
}

const (
	// maxMessageBytes caps one frame. A snapshot of a full lobby is a few
	// kilobytes; anything approaching this is a mistake or an attempt to make
	// the bot allocate on demand.
	maxMessageBytes = 256 * 1024

	// idleTimeout closes a connection that has gone quiet. Capture sends
	// heartbeats, so silence for this long means the other end is gone even
	// though the socket still looks open.
	idleTimeout = 90 * time.Second

	// writeTimeout bounds a single write, so one unresponsive peer cannot hold
	// a goroutine forever.
	writeTimeout = 10 * time.Second
)

// Server accepts capture connections.
type Server struct {
	auth     Authenticator
	handler  Handler
	logger   *log.Logger
	upgrader websocket.Upgrader

	mu       sync.Mutex
	sessions map[string]int
}

// NewServer builds a server. A nil logger writes to the standard logger.
func NewServer(auth Authenticator, handler Handler, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}

	return &Server{
		auth:    auth,
		handler: handler,
		logger:  logger,
		upgrader: websocket.Upgrader{
			HandshakeTimeout: 10 * time.Second,
			ReadBufferSize:   4096,
			WriteBufferSize:  4096,
			// Capture is a desktop application and sends no Origin header, which
			// this accepts. A browser always sends one, so a web page cannot
			// open this socket even if it learns the address: the credential
			// would still be required, but refusing here means a page cannot
			// even reach the handshake.
			CheckOrigin: func(r *http.Request) bool {
				return r.Header.Get("Origin") == ""
			},
		},
		sessions: map[string]int{},
	}
}

// Routes returns the HTTP surface: the capture socket and the pairing endpoint.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/capture/link", s.handleLink)
	mux.HandleFunc("/capture/pair", s.handlePair)
	return mux
}

// Connections reports how many capture sessions are connected for a guild. It
// is what /au capture status and /au doctor read.
func (s *Server) Connections(guildID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[guildID]
}

// handlePair exchanges a typed pairing code for a credential.
//
// This is deliberately outside the WebSocket protocol. Pairing happens once,
// before there is a session to speak in, and folding it into the protocol would
// have meant a message that exists only for the first connection of a capture
// install's life.
func (s *Server) handlePair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeJSONError(w, http.StatusBadRequest, "malformed", "the request is not a pairing request")
		return
	}

	guildID, issued, err := s.auth.RedeemCode(request.Code)
	switch {
	case errors.Is(err, credential.ErrExpired):
		// The reason is given because it changes what the person does next: ask
		// for a new code rather than check what they typed.
		writeJSONError(w, http.StatusForbidden, "expired",
			"that pairing code has expired; ask an administrator for a new one with /au capture pair")
		return
	case errors.Is(err, credential.ErrUsed), errors.Is(err, credential.ErrMismatch):
		writeJSONError(w, http.StatusForbidden, "rejected",
			"that pairing code is not valid; ask an administrator for a new one with /au capture pair")
		return
	case err != nil:
		s.logger.Println("capture pairing failed:", err)
		writeJSONError(w, http.StatusInternalServerError, "unavailable", "pairing is temporarily unavailable")
		return
	}

	s.logger.Printf("capture paired for guild %s", guildID)

	// The credential is returned here and nowhere else, ever. It is the one
	// moment it exists outside capture.
	writeJSON(w, http.StatusOK, map[string]string{
		"guild":      guildID,
		"credential": issued.Token().Reveal(),
	})
}

// handleLink upgrades a capture connection and runs it to completion.
func (s *Server) handleLink(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade has already written a response by the time it fails.
		return
	}

	// Run the session on this request's goroutine rather than starting another.
	// The HTTP server already gave the request one, and keeping the connection
	// tied to it means a shutting-down server waits for the session instead of
	// leaving it orphaned.
	s.serve(conn)
}

// serve runs one capture session until it ends or breaks a rule.
func (s *Server) serve(conn *websocket.Conn) {
	defer conn.Close()

	conn.SetReadLimit(maxMessageBytes)

	var receiver protocol.Receiver
	guildID := ""

	defer func() {
		if guildID != "" {
			s.leave(guildID)
		}
	}()

	for {
		if err := conn.SetReadDeadline(time.Now().Add(idleTimeout)); err != nil {
			return
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		message, err := protocol.Decode(data)
		if err != nil {
			s.refuse(conn, protocol.NewError("", 0, protocol.CodeMalformed, err.Error()))
			return
		}

		action, refusal := receiver.Accept(message)
		if refusal != nil {
			s.refuse(conn, refusal)
			// A refusal that leaves the session unusable ends the connection;
			// one the session can recover from does not.
			if fatal(refusal.Code) {
				return
			}
			continue
		}
		if action == protocol.ActionIgnore {
			continue
		}

		// The handshake and capture's own error reports are not game data and
		// never reach the handler: hello carries no state, authentication
		// carries a secret, and an error is a report about a problem.
		switch typed := message.(type) {
		case *protocol.Hello:
			continue

		case *protocol.Authentication:
			resolved, authErr := s.auth.Authenticate(typed.Credential)
			if authErr != nil {
				// The connection has to close here. The protocol receiver has
				// already recorded that an authentication message arrived in
				// the right order, so letting the loop continue would leave a
				// session that believes it is authenticated and is not.
				s.refuse(conn, authenticationRefusal(typed, authErr))
				return
			}
			guildID = resolved
			s.join(guildID)
			continue

		case *protocol.Error:
			// Only the code and the message are logged. Capture writes both,
			// and neither is a place a credential belongs.
			s.logger.Printf("capture reported %s: %s", typed.Code, typed.Message)
			continue
		}

		if guildID == "" {
			// Unreachable while the protocol requires authentication first, and
			// cheap insurance against that ever changing without this being
			// reconsidered.
			s.refuse(conn, protocol.NewError(protocol.Envelope(message).Session,
				protocol.Envelope(message).Seq, protocol.CodeUnauthenticated,
				"capture must authenticate before sending game data"))
			return
		}

		if err := s.handler.HandleCapture(guildID, message); err != nil {
			s.logger.Printf("capture message for guild %s could not be applied: %v", guildID, err)
		}
	}
}

// authenticationRefusal turns a credential failure into a protocol error the
// person running capture can act on.
//
// The presented credential is never part of the message, and never reaches a
// log: the only thing said out loud is what the holder should do next.
func authenticationRefusal(message *protocol.Authentication, err error) *protocol.Error {
	envelope := protocol.Envelope(message)

	switch {
	case errors.Is(err, credential.ErrRevoked):
		return protocol.NewError(envelope.Session, envelope.Seq, protocol.CodeUnauthenticated,
			"this capture credential was revoked; ask an administrator to run /au capture pair again")
	default:
		return protocol.NewError(envelope.Session, envelope.Seq, protocol.CodeUnauthenticated,
			"this capture credential is not valid for any guild; pair again with /au capture pair")
	}
}

// fatal reports whether a refusal ends the connection.
//
// Only the refusals a session cannot recover from do. A missing snapshot is
// recoverable by sending one, and closing the connection for it would turn an
// ordinary reconnect into a loop.
func fatal(code string) bool {
	switch code {
	case protocol.CodeIncompatibleProtocol, protocol.CodeExpectedHello,
		protocol.CodeUnauthenticated, protocol.CodeMalformed:
		return true
	default:
		return false
	}
}

// refuse sends one protocol error, best effort. A peer that has already gone
// away cannot be told anything, and failing to tell it is not worth a log line.
func (s *Server) refuse(conn *websocket.Conn, refusal *protocol.Error) {
	encoded, err := protocol.Encode(refusal)
	if err != nil {
		return
	}
	if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return
	}
	_ = conn.WriteMessage(websocket.TextMessage, encoded)
}

func (s *Server) join(guildID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[guildID]++
}

func (s *Server) leave(guildID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sessions[guildID] <= 1 {
		delete(s.sessions, guildID)
		return
	}
	s.sessions[guildID]--
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
