package transport

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/credential"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/pairing"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/gorilla/websocket"
)

const guild = "guild-1"

// mustNotAppear is the secret half of a rejected token. The log is searched
// for it, because the failure path is where a credential is most tempting to
// write down.
const mustNotAppear = "value-that-must-never-be-logged"

// received collects what the handler was given, so a test can assert on what
// actually made it past the protocol and the credential check.
type received struct {
	mu       sync.Mutex
	messages []protocol.Message
	guilds   []string
}

func (r *received) HandleCapture(guildID string, message protocol.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.guilds = append(r.guilds, guildID)
	r.messages = append(r.messages, message)
	return nil
}

func (r *received) types() []protocol.Type {
	r.mu.Lock()
	defer r.mu.Unlock()

	types := make([]protocol.Type, 0, len(r.messages))
	for _, message := range r.messages {
		types = append(types, protocol.Envelope(message).Type)
	}
	return types
}

type harness struct {
	server   *httptest.Server
	pairing  *pairing.Service
	handler  *received
	logs     *bytes.Buffer
	capture  *Server
	clock    func() time.Time
	advance  func(time.Duration)
	database *sqlite.DB
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "auvc.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	var mu sync.Mutex
	now := time.Unix(1_700_000_000, 0).UTC()
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	advance := func(d time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		now = now.Add(d)
	}

	pairingService := pairing.NewServiceWithClock(db, clock)
	handler := &received{}
	logs := &bytes.Buffer{}
	capture := NewServer(pairingService, handler, log.New(logs, "", 0))

	server := httptest.NewServer(capture.Routes())
	t.Cleanup(server.Close)

	return &harness{
		server:   server,
		pairing:  pairingService,
		handler:  handler,
		logs:     logs,
		capture:  capture,
		clock:    clock,
		advance:  advance,
		database: db,
	}
}

// pairOverHTTP walks the real pairing endpoint, the way capture will.
func (h *harness) pairOverHTTP(t *testing.T, code string) (int, map[string]string) {
	t.Helper()

	body, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		t.Fatalf("encode pairing request: %v", err)
	}

	response, err := http.Post(h.server.URL+"/capture/pair", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("pairing request: %v", err)
	}
	defer response.Body.Close()

	var decoded map[string]string
	_ = json.NewDecoder(response.Body).Decode(&decoded)
	return response.StatusCode, decoded
}

func (h *harness) dial(t *testing.T) *websocket.Conn {
	t.Helper()

	url := "ws" + strings.TrimPrefix(h.server.URL, "http") + "/capture/link"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func send(t *testing.T, conn *websocket.Conn, message protocol.Message) {
	t.Helper()

	encoded, err := protocol.Encode(message)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, encoded); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// readRefusal expects the server to answer with a protocol error.
func readRefusal(t *testing.T, conn *websocket.Conn) *protocol.Error {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("expected a refusal, got %v", err)
	}

	message, err := protocol.Decode(data)
	if err != nil {
		t.Fatalf("the server sent something that is not a message: %v", err)
	}
	refusal, ok := message.(*protocol.Error)
	if !ok {
		t.Fatalf("expected an error message, got %s", protocol.Envelope(message).Type)
	}
	return refusal
}

// expectClosed asserts that the server hung up.
func expectClosed(t *testing.T, conn *websocket.Conn) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func header(messageType protocol.Type, session string, seq uint64) protocol.Header {
	return protocol.Header{Protocol: protocol.Version, Type: messageType, Session: session, Seq: seq}
}

// issueCredential pairs a guild through Discord and then through the HTTP
// endpoint, which is exactly the path a real install takes.
func (h *harness) issueCredential(t *testing.T) string {
	t.Helper()

	code, _, err := h.pairing.Pair(guild, "admin")
	if err != nil {
		t.Fatalf("pair: %v", err)
	}

	status, body := h.pairOverHTTP(t, code.Display())
	if status != http.StatusOK {
		t.Fatalf("pairing returned %d: %v", status, body)
	}
	if body["guild"] != guild {
		t.Fatalf("paired for guild %q, want %q", body["guild"], guild)
	}
	if body["credential"] == "" {
		t.Fatal("pairing returned no credential")
	}
	return body["credential"]
}

// notACredential builds a token that is shaped like a credential and is not
// one: no stored credential carries this identifier.
//
// It is assembled rather than written out as a literal so that a secret scanner
// reading this file sees a construction, not a key. A test fixture that trips
// the scanner would either fail every build or earn a standing allowlist entry,
// and an allowlist over this file could later hide a real mistake in it.
func notACredential(secret string) string {
	return "00112233445566aa" + "." + secret
}

func TestACapturePairsConnectsAndSendsAGame(t *testing.T) {
	h := newHarness(t)
	token := h.issueCredential(t)

	conn := h.dial(t)
	send(t, conn, &protocol.Hello{Header: header(protocol.TypeHello, "s1", 1), Capture: "1.0.0"})
	send(t, conn, &protocol.Authentication{
		Header:     header(protocol.TypeAuthentication, "s1", 2),
		Credential: token,
	})
	send(t, conn, &protocol.Snapshot{
		Header:  header(protocol.TypeSnapshot, "s1", 3),
		Phase:   protocol.PhaseTasks,
		Players: []protocol.Player{{Name: "Red"}},
	})
	send(t, conn, &protocol.PlayerDied{
		Header: header(protocol.TypePlayerDied, "s1", 4),
		Player: protocol.Player{Name: "Red", Dead: true},
	})

	waitFor(t, func() bool { return len(h.handler.types()) >= 2 })

	got := h.handler.types()
	if len(got) != 2 || got[0] != protocol.TypeSnapshot || got[1] != protocol.TypePlayerDied {
		t.Errorf("the handler received %v, want snapshot then player_died", got)
	}

	h.handler.mu.Lock()
	defer h.handler.mu.Unlock()
	for _, seen := range h.handler.guilds {
		if seen != guild {
			t.Errorf("a message arrived for guild %q, want %q", seen, guild)
		}
	}
}

// The hello and the authentication are handshake, not game data, so they must
// not reach the part of the bot that drives voice.
func TestTheHandshakeDoesNotReachTheHandler(t *testing.T) {
	h := newHarness(t)
	token := h.issueCredential(t)

	conn := h.dial(t)
	send(t, conn, &protocol.Hello{Header: header(protocol.TypeHello, "s1", 1), Capture: "1.0.0"})
	send(t, conn, &protocol.Authentication{
		Header:     header(protocol.TypeAuthentication, "s1", 2),
		Credential: token,
	})
	send(t, conn, &protocol.Snapshot{
		Header:  header(protocol.TypeSnapshot, "s1", 3),
		Phase:   protocol.PhaseLobby,
		Players: []protocol.Player{{Name: "Red"}},
	})

	waitFor(t, func() bool { return len(h.handler.types()) >= 1 })

	for _, seen := range h.handler.types() {
		if seen == protocol.TypeHello || seen == protocol.TypeAuthentication {
			t.Errorf("%s reached the handler", seen)
		}
	}
}

// The protocol receiver records that an authentication arrived in the right
// order before the credential is checked. If a failed check did not close the
// connection, the session would believe it is authenticated and is not.
func TestAWrongCredentialClosesTheConnection(t *testing.T) {
	h := newHarness(t)
	h.issueCredential(t)

	conn := h.dial(t)
	send(t, conn, &protocol.Hello{Header: header(protocol.TypeHello, "s1", 1), Capture: "1.0.0"})
	send(t, conn, &protocol.Authentication{
		Header:     header(protocol.TypeAuthentication, "s1", 2),
		Credential: notACredential("not-the-secret"),
	})

	refusal := readRefusal(t, conn)
	if refusal.Code != protocol.CodeUnauthenticated {
		t.Errorf("got code %q, want %q", refusal.Code, protocol.CodeUnauthenticated)
	}
	expectClosed(t, conn)

	if got := h.handler.types(); len(got) != 0 {
		t.Errorf("an unauthenticated session reached the handler: %v", got)
	}
}

func TestARevokedCredentialIsTurnedAwayWithAnExplanation(t *testing.T) {
	h := newHarness(t)
	token := h.issueCredential(t)

	if _, err := h.pairing.Revoke(guild); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	conn := h.dial(t)
	send(t, conn, &protocol.Hello{Header: header(protocol.TypeHello, "s1", 1), Capture: "1.0.0"})
	send(t, conn, &protocol.Authentication{
		Header:     header(protocol.TypeAuthentication, "s1", 2),
		Credential: token,
	})

	refusal := readRefusal(t, conn)
	if refusal.Code != protocol.CodeUnauthenticated {
		t.Errorf("got code %q, want %q", refusal.Code, protocol.CodeUnauthenticated)
	}
	// The holder has to learn that this is not a typo they can fix.
	if !strings.Contains(refusal.Message, "revoked") {
		t.Errorf("a revoked credential should say so: %q", refusal.Message)
	}
	expectClosed(t, conn)
}

func TestAnIncompatibleProtocolIsRefusedAtTheHandshake(t *testing.T) {
	h := newHarness(t)
	h.issueCredential(t)

	conn := h.dial(t)
	stranger := &protocol.Hello{Header: header(protocol.TypeHello, "s1", 1), Capture: "1.0.0"}
	stranger.Protocol = protocol.Version + 1
	send(t, conn, stranger)

	refusal := readRefusal(t, conn)
	if refusal.Code != protocol.CodeIncompatibleProtocol {
		t.Errorf("got code %q, want %q", refusal.Code, protocol.CodeIncompatibleProtocol)
	}
	expectClosed(t, conn)
}

func TestGameDataBeforeAuthenticationIsRefused(t *testing.T) {
	h := newHarness(t)

	conn := h.dial(t)
	send(t, conn, &protocol.Hello{Header: header(protocol.TypeHello, "s1", 1), Capture: "1.0.0"})
	send(t, conn, &protocol.Snapshot{
		Header:  header(protocol.TypeSnapshot, "s1", 2),
		Phase:   protocol.PhaseTasks,
		Players: []protocol.Player{{Name: "Red"}},
	})

	refusal := readRefusal(t, conn)
	if refusal.Code != protocol.CodeUnauthenticated {
		t.Errorf("got code %q, want %q", refusal.Code, protocol.CodeUnauthenticated)
	}
	expectClosed(t, conn)
}

// A missing snapshot is something the session can fix by sending one. Closing
// the connection for it would turn an ordinary recovery into a reconnect loop.
func TestAMissingSnapshotDoesNotEndTheConnection(t *testing.T) {
	h := newHarness(t)
	token := h.issueCredential(t)

	conn := h.dial(t)
	send(t, conn, &protocol.Hello{Header: header(protocol.TypeHello, "s1", 1), Capture: "1.0.0"})
	send(t, conn, &protocol.Authentication{
		Header:     header(protocol.TypeAuthentication, "s1", 2),
		Credential: token,
	})
	send(t, conn, &protocol.PlayerDied{
		Header: header(protocol.TypePlayerDied, "s1", 3),
		Player: protocol.Player{Name: "Red", Dead: true},
	})

	refusal := readRefusal(t, conn)
	if refusal.Code != protocol.CodeSnapshotRequired {
		t.Fatalf("got code %q, want %q", refusal.Code, protocol.CodeSnapshotRequired)
	}

	// The same connection recovers by sending what was asked for.
	send(t, conn, &protocol.Snapshot{
		Header:  header(protocol.TypeSnapshot, "s1", 4),
		Phase:   protocol.PhaseTasks,
		Players: []protocol.Player{{Name: "Red"}},
	})
	waitFor(t, func() bool { return len(h.handler.types()) >= 1 })

	if got := h.handler.types(); got[0] != protocol.TypeSnapshot {
		t.Errorf("the recovery snapshot did not arrive: %v", got)
	}
}

func TestAPairingCodeWorksOnlyOnce(t *testing.T) {
	h := newHarness(t)

	code, _, err := h.pairing.Pair(guild, "admin")
	if err != nil {
		t.Fatalf("pair: %v", err)
	}

	if status, _ := h.pairOverHTTP(t, code.Display()); status != http.StatusOK {
		t.Fatalf("first redemption returned %d", status)
	}
	status, body := h.pairOverHTTP(t, code.Display())
	if status != http.StatusForbidden {
		t.Errorf("replaying a code returned %d, want %d", status, http.StatusForbidden)
	}
	if body["credential"] != "" {
		t.Error("a replayed code still produced a credential")
	}
}

func TestAnExpiredPairingCodeSaysSo(t *testing.T) {
	h := newHarness(t)

	code, _, err := h.pairing.Pair(guild, "admin")
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	h.advance(credential.PairingCodeLifetime + time.Second)

	status, body := h.pairOverHTTP(t, code.Display())
	if status != http.StatusForbidden {
		t.Errorf("an expired code returned %d, want %d", status, http.StatusForbidden)
	}
	// Expired and wrong lead to different next steps, so they read differently.
	if body["error"] != "expired" {
		t.Errorf("an expired code reported %q", body["error"])
	}
}

func TestAWrongPairingCodeIsRefused(t *testing.T) {
	h := newHarness(t)
	if _, _, err := h.pairing.Pair(guild, "admin"); err != nil {
		t.Fatalf("pair: %v", err)
	}

	status, body := h.pairOverHTTP(t, "AUVC-0000-0000")
	if status != http.StatusForbidden {
		t.Errorf("a wrong code returned %d, want %d", status, http.StatusForbidden)
	}
	if body["credential"] != "" {
		t.Error("a wrong code produced a credential")
	}
}

func TestThePairingEndpointRefusesAnythingButPost(t *testing.T) {
	h := newHarness(t)

	response, err := http.Get(h.server.URL + "/capture/pair")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET returned %d, want %d", response.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestAMalformedPairingRequestIsRefused(t *testing.T) {
	h := newHarness(t)

	for name, body := range map[string]string{
		"not json":      "{",
		"unknown field": `{"code":"AUVC-0000-0000","guild":"sneaky"}`,
		"two documents": `{"code":"a"}{"code":"b"}`,
		"not an object": `"AUVC-0000-0000"`,
	} {
		t.Run(name, func(t *testing.T) {
			response, err := http.Post(h.server.URL+"/capture/pair", "application/json", strings.NewReader(body))
			if err != nil {
				t.Fatalf("post: %v", err)
			}
			defer response.Body.Close()

			if response.StatusCode != http.StatusBadRequest {
				t.Errorf("returned %d, want %d", response.StatusCode, http.StatusBadRequest)
			}
		})
	}
}

// A web page must not be able to open this socket. Capture is a desktop
// application and sends no Origin header; a browser always sends one.
func TestABrowserCannotOpenTheCaptureSocket(t *testing.T) {
	h := newHarness(t)

	url := "ws" + strings.TrimPrefix(h.server.URL, "http") + "/capture/link"
	conn, response, err := websocket.DefaultDialer.Dial(url, http.Header{
		"Origin": []string{"https://example.invalid"},
	})
	if err == nil {
		conn.Close()
		t.Fatal("a request with an Origin header was upgraded")
	}
	if response != nil && response.StatusCode != http.StatusForbidden {
		t.Errorf("returned %d, want %d", response.StatusCode, http.StatusForbidden)
	}
}

// The requirement is absolute: no credential in plaintext, anywhere in a log.
func TestNoCredentialEverReachesTheLog(t *testing.T) {
	h := newHarness(t)
	token := h.issueCredential(t)

	conn := h.dial(t)
	send(t, conn, &protocol.Hello{Header: header(protocol.TypeHello, "s1", 1), Capture: "1.0.0"})
	send(t, conn, &protocol.Authentication{
		Header:     header(protocol.TypeAuthentication, "s1", 2),
		Credential: token,
	})
	send(t, conn, &protocol.Snapshot{
		Header:  header(protocol.TypeSnapshot, "s1", 3),
		Phase:   protocol.PhaseTasks,
		Players: []protocol.Player{{Name: "Red"}},
	})
	waitFor(t, func() bool { return len(h.handler.types()) >= 1 })

	// And once more with a bad one, because the failure path is where a
	// credential is most tempting to log.
	bad := h.dial(t)
	send(t, bad, &protocol.Hello{Header: header(protocol.TypeHello, "s2", 1), Capture: "1.0.0"})
	send(t, bad, &protocol.Authentication{
		Header:     header(protocol.TypeAuthentication, "s2", 2),
		Credential: notACredential(mustNotAppear),
	})
	expectClosed(t, bad)

	logged := h.logs.String()
	for _, secret := range []string{token, mustNotAppear} {
		if strings.Contains(logged, secret) {
			t.Errorf("a credential reached the log:\n%s", logged)
		}
	}
}

func TestConnectionsAreCountedPerGuild(t *testing.T) {
	h := newHarness(t)
	token := h.issueCredential(t)

	if got := h.capture.Connections(guild); got != 0 {
		t.Fatalf("a fresh server reported %d connections", got)
	}

	conn := h.dial(t)
	send(t, conn, &protocol.Hello{Header: header(protocol.TypeHello, "s1", 1), Capture: "1.0.0"})
	send(t, conn, &protocol.Authentication{
		Header:     header(protocol.TypeAuthentication, "s1", 2),
		Credential: token,
	})
	waitFor(t, func() bool { return h.capture.Connections(guild) == 1 })

	conn.Close()
	waitFor(t, func() bool { return h.capture.Connections(guild) == 0 })
}

func TestAFrameThatIsNotAMessageClosesTheConnection(t *testing.T) {
	h := newHarness(t)

	conn := h.dial(t)
	if err := conn.WriteMessage(websocket.TextMessage, []byte("{not json")); err != nil {
		t.Fatalf("write: %v", err)
	}

	refusal := readRefusal(t, conn)
	if refusal.Code != protocol.CodeMalformed {
		t.Errorf("got code %q, want %q", refusal.Code, protocol.CodeMalformed)
	}
	expectClosed(t, conn)
}

// waitFor polls a condition, because the server applies messages on its own
// goroutine and the test has no other way to know it got there.
func waitFor(t *testing.T, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the server")
}
