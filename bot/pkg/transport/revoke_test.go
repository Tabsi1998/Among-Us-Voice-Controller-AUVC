package transport

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
	"github.com/gorilla/websocket"
)

// connect opens a session and waits until the server has registered it.
func (h *harness) connect(t *testing.T, guildID, token string) *websocket.Conn {
	t.Helper()

	conn := h.dial(t)
	send(t, conn, &protocol.Hello{Header: header(protocol.TypeHello, "s1", 1), Capture: "1.0.0"})
	send(t, conn, &protocol.Authentication{
		Header:     header(protocol.TypeAuthentication, "s1", 2),
		Credential: token,
	})
	waitFor(t, func() bool { return h.capture.Connections(guildID) == 1 })
	return conn
}

// The credential is checked at the handshake only, and heartbeats keep a
// connection alive. Without Disconnect, revoking would leave an already
// connected capture working for as long as it stayed connected.
func TestRevokingEndsAConnectionThatIsAlreadyOpen(t *testing.T) {
	h := newHarness(t)
	conn := h.connect(t, guild, h.issueCredential(t))

	if _, err := h.pairing.Revoke(guild); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if ended := h.capture.Disconnect(guild); ended != 1 {
		t.Errorf("Disconnect reported %d connections, want 1", ended)
	}

	// Capture has to learn that access was withdrawn, not guess at a network
	// failure and retry forever.
	refusal := readRefusal(t, conn)
	if refusal.Code != protocol.CodeUnauthenticated {
		t.Errorf("got code %q, want %q", refusal.Code, protocol.CodeUnauthenticated)
	}
	if !strings.Contains(refusal.Message, "revoked") {
		t.Errorf("the capture should be told it was revoked: %q", refusal.Message)
	}
	expectClosed(t, conn)

	waitFor(t, func() bool { return h.capture.Connections(guild) == 0 })
}

func TestRevokingOneGuildLeavesAnotherConnected(t *testing.T) {
	h := newHarness(t)
	const other = "guild-2"

	revoked := h.connect(t, guild, h.issueCredential(t))

	code, _, err := h.pairing.Pair(other, "admin")
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	status, body := h.pairOverHTTP(t, code.Display())
	if status != http.StatusOK {
		t.Fatalf("pairing returned %d: %v", status, body)
	}
	kept := h.connect(t, other, body["credential"])

	h.capture.Disconnect(guild)
	expectClosed(t, revoked)

	send(t, kept, &protocol.Snapshot{
		Header:  header(protocol.TypeSnapshot, "s1", 3),
		Phase:   protocol.PhaseLobby,
		Players: []protocol.Player{{Name: "Red"}},
	})
	waitFor(t, func() bool { return len(h.handler.types()) >= 1 })

	h.handler.mu.Lock()
	defer h.handler.mu.Unlock()
	if h.handler.guilds[0] != other {
		t.Errorf("a message arrived for guild %q, want %q", h.handler.guilds[0], other)
	}
	if got := h.capture.Connections(other); got != 1 {
		t.Errorf("the other guild has %d connections, want 1", got)
	}
}

// A revoke must not become a permanent ban on the guild: pairing again is the
// documented way back.
func TestAGuildCanConnectAgainAfterPairingAgain(t *testing.T) {
	h := newHarness(t)
	h.connect(t, guild, h.issueCredential(t))

	if _, err := h.pairing.Revoke(guild); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	h.capture.Disconnect(guild)
	waitFor(t, func() bool { return h.capture.Connections(guild) == 0 })

	h.connect(t, guild, h.issueCredential(t))
}

// gatedAuthenticator holds an authentication between the credential check and
// its result, which is exactly where a revoke can slip in unnoticed.
type gatedAuthenticator struct {
	Authenticator
	checked chan struct{}
	release chan struct{}
}

func (g *gatedAuthenticator) Authenticate(token string) (string, error) {
	guildID, err := g.Authenticator.Authenticate(token)
	close(g.checked)
	<-g.release
	return guildID, err
}

// The credential check reads the database before the revoke writes to it, and
// the revoke looks for open connections before this one registers. Each step is
// correct on its own, and together they would let one connection outlive the
// revoke.
func TestAConnectionCheckedJustBeforeARevokeIsStillRefused(t *testing.T) {
	h := newHarness(t)
	token := h.issueCredential(t)

	gated := &gatedAuthenticator{
		Authenticator: h.pairing,
		checked:       make(chan struct{}),
		release:       make(chan struct{}),
	}
	release := sync.OnceFunc(func() { close(gated.release) })
	t.Cleanup(release)

	capture := NewServer(gated, h.handler, log.New(io.Discard, "", 0))
	server := httptest.NewServer(capture.Routes())
	t.Cleanup(server.Close)
	raced := &harness{server: server, capture: capture, handler: h.handler}

	conn := raced.dial(t)
	send(t, conn, &protocol.Hello{Header: header(protocol.TypeHello, "s1", 1), Capture: "1.0.0"})
	send(t, conn, &protocol.Authentication{
		Header:     header(protocol.TypeAuthentication, "s1", 2),
		Credential: token,
	})

	select {
	case <-gated.checked:
	case <-time.After(5 * time.Second):
		t.Fatal("the credential was never checked")
	}

	if _, err := h.pairing.Revoke(guild); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if ended := capture.Disconnect(guild); ended != 0 {
		t.Fatalf("Disconnect found %d connections before the check finished", ended)
	}
	release()

	refusal := readRefusal(t, conn)
	if refusal.Code != protocol.CodeUnauthenticated || !strings.Contains(refusal.Message, "revoked") {
		t.Errorf("got %q %q, want an unauthenticated refusal that says revoked", refusal.Code, refusal.Message)
	}
	expectClosed(t, conn)

	if got := capture.Connections(guild); got != 0 {
		t.Errorf("a connection that raced the revoke was registered: %d", got)
	}
}
