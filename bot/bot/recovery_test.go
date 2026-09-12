package bot

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/pairing"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/session"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/transport"
	"github.com/gorilla/websocket"
)

// These are the recovery scenarios from docs/requirements.md, driven through
// the real WebSocket server, the real protocol receiver and the real bot
// handler. Only the final write to Discord is absent, and that has its own
// tests in pkg/voice and pkg/transport.
//
// The bot is built without a reconciler, so reconciliation is skipped and
// nothing reaches for a Discord connection that is not there.

type recovery struct {
	server     *httptest.Server
	controller *Bot
	credential string
}

func newRecovery(t *testing.T) *recovery {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "auvc.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	pairingService := pairing.NewService(db)
	code, _, err := pairingService.Pair(guild, "admin")
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	_, issued, err := pairingService.RedeemCode(code.Display())
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}

	controller := &Bot{CaptureSessions: NewCaptureSessions(), AUVCLinks: db}
	server := httptest.NewServer(transport.NewServer(pairingService, controller, nil).Routes())
	t.Cleanup(server.Close)

	return &recovery{server: server, controller: controller, credential: issued.Token().Reveal()}
}

// capture is one capture connection, numbering its own messages the way the
// real client does.
type capture struct {
	conn    *websocket.Conn
	session string
	seq     uint64
}

func (r *recovery) connect(t *testing.T, sessionID string) *capture {
	t.Helper()

	url := "ws" + strings.TrimPrefix(r.server.URL, "http") + "/capture/link"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return &capture{conn: conn, session: sessionID}
}

func (c *capture) header(messageType protocol.Type) protocol.Header {
	c.seq++
	return protocol.Header{
		Protocol: protocol.Version, Type: messageType, Session: c.session, Seq: c.seq,
	}
}

func (c *capture) send(t *testing.T, message protocol.Message) {
	t.Helper()

	encoded, err := protocol.Encode(message)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, encoded); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// handshake opens the session and authenticates, which is what every
// connection has to do, including a reconnect.
func (c *capture) handshake(t *testing.T, credential string) {
	t.Helper()

	c.send(t, &protocol.Hello{Header: c.header(protocol.TypeHello), Capture: "1.0.0"})
	c.send(t, &protocol.Authentication{
		Header: c.header(protocol.TypeAuthentication), Credential: credential,
	})
}

func (c *capture) snapshot(t *testing.T, phase protocol.Phase, players ...protocol.Player) {
	t.Helper()

	c.send(t, &protocol.Snapshot{
		Header: c.header(protocol.TypeSnapshot), Phase: phase, Players: players,
	})
}

// waitForPlayers polls until the bot has applied what capture sent, because the
// server applies messages on its own goroutine.
func (r *recovery) waitForPlayers(t *testing.T, count int) []session.GamePlayer {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, players := r.controller.CaptureSessions.Snapshot(guild); len(players) == count {
			return players
		}
		time.Sleep(5 * time.Millisecond)
	}

	_, _, players := r.controller.CaptureSessions.Snapshot(guild)
	t.Fatalf("timed out waiting for %d players, have %+v", count, players)
	return nil
}

func (r *recovery) phase(t *testing.T) game.Phase {
	t.Helper()

	_, phase, _ := r.controller.CaptureSessions.Snapshot(guild)
	return phase
}

// A capture that drops and comes back sends a new session and a complete
// snapshot, and the bot rebuilds the round from it.
func TestACaptureReconnectRebuildsTheRound(t *testing.T) {
	r := newRecovery(t)

	first := r.connect(t, "session-a")
	first.handshake(t, r.credential)
	first.snapshot(t, protocol.PhaseTasks,
		protocol.Player{Name: "Red"},
		protocol.Player{Name: "Blue"},
		protocol.Player{Name: "Green"},
	)
	first.send(t, &protocol.PlayerDied{
		Header: first.header(protocol.TypePlayerDied),
		Player: protocol.Player{Name: "Green", Dead: true},
	})
	r.waitForPlayers(t, 3)

	// The capture process dies and starts again. Green left the game while the
	// bot could not see, and a meeting has happened.
	first.conn.Close()

	second := r.connect(t, "session-b")
	second.handshake(t, r.credential)
	second.snapshot(t, protocol.PhaseDiscussion,
		protocol.Player{Name: "Red"},
		protocol.Player{Name: "Blue", Dead: true},
	)

	players := r.waitForPlayers(t, 2)
	if r.phase(t) != game.DISCUSS {
		t.Errorf("phase is %v, want DISCUSS", r.phase(t))
	}

	byName := map[string]session.GamePlayer{}
	for _, player := range players {
		byName[player.Name] = player
	}
	if _, stale := byName["Green"]; stale {
		t.Error("a player who is gone survived the reconnect")
	}
	if byName["Blue"].Alive {
		t.Error("the snapshot said Blue is dead and the bot still has them alive")
	}
	// The snapshot arrived during a meeting, so the death is public.
	if !byName["Blue"].Revealed {
		t.Error("a death in a meeting snapshot should be public")
	}
}

// Messages from the connection that was replaced are late by definition. Acting
// on them would undo the snapshot that just rebuilt the round.
func TestStaleEventsFromTheReplacedSessionAreIgnored(t *testing.T) {
	r := newRecovery(t)

	first := r.connect(t, "session-a")
	first.handshake(t, r.credential)
	first.snapshot(t, protocol.PhaseTasks, protocol.Player{Name: "Red"}, protocol.Player{Name: "Blue"})
	r.waitForPlayers(t, 2)

	second := r.connect(t, "session-b")
	second.handshake(t, r.credential)
	second.snapshot(t, protocol.PhaseTasks, protocol.Player{Name: "Red"})
	r.waitForPlayers(t, 1)

	// The old connection had not noticed it was replaced.
	first.send(t, &protocol.PlayerDied{
		Header: first.header(protocol.TypePlayerDied),
		Player: protocol.Player{Name: "Red", Dead: true},
	})

	// Give the server a moment to do the wrong thing, if it is going to.
	time.Sleep(100 * time.Millisecond)

	players := r.waitForPlayers(t, 1)
	if !players[0].Alive {
		t.Error("a stale event from the replaced session killed a player")
	}
}

// A duplicate is ordinary on a reconnecting transport. It must not be applied
// twice, and it must not be answered with an error either.
func TestADuplicateEventChangesNothing(t *testing.T) {
	r := newRecovery(t)

	capture := r.connect(t, "session-a")
	capture.handshake(t, r.credential)
	capture.snapshot(t, protocol.PhaseTasks, protocol.Player{Name: "Red"}, protocol.Player{Name: "Blue"})
	r.waitForPlayers(t, 2)

	death := &protocol.PlayerDied{
		Header: capture.header(protocol.TypePlayerDied),
		Player: protocol.Player{Name: "Blue", Dead: true},
	}
	capture.send(t, death)
	capture.send(t, death)
	time.Sleep(100 * time.Millisecond)

	players := r.waitForPlayers(t, 2)
	for _, player := range players {
		if player.Name == "Red" && !player.Alive {
			t.Error("a duplicate death reached the wrong player")
		}
	}
}

// The bot restarts mid-round and remembers nothing: the session lives in the
// process. Capture reconnects, and the snapshot is what puts the round back.
func TestABotRestartIsRecoveredByTheNextSnapshot(t *testing.T) {
	r := newRecovery(t)

	capture := r.connect(t, "session-a")
	capture.handshake(t, r.credential)
	capture.snapshot(t, protocol.PhaseTasks, protocol.Player{Name: "Red"}, protocol.Player{Name: "Blue"})
	r.waitForPlayers(t, 2)

	// A restart is a bot with nothing in it.
	restarted := &Bot{CaptureSessions: NewCaptureSessions(), AUVCLinks: r.controller.AUVCLinks}
	if _, _, players := restarted.CaptureSessions.Snapshot(guild); len(players) != 0 {
		t.Fatal("a restarted bot should know nothing")
	}

	replay := []protocol.Message{
		&protocol.Snapshot{
			Header: protocol.Header{Protocol: protocol.Version, Type: protocol.TypeSnapshot,
				Session: "session-b", Seq: 3},
			Phase:   protocol.PhaseDiscussion,
			Players: []protocol.Player{{Name: "Red"}, {Name: "Blue", Dead: true}},
		},
	}
	for _, message := range replay {
		if err := restarted.HandleCapture(guild, message); err != nil {
			t.Fatalf("replaying %s: %v", protocol.Envelope(message).Type, err)
		}
	}

	_, phase, players := restarted.CaptureSessions.Snapshot(guild)
	if phase != game.DISCUSS || len(players) != 2 {
		t.Errorf("the round was not rebuilt: phase=%v players=%+v", phase, players)
	}
}

// The whole fail-safe, end to end: a capture that stops talking, the session
// pausing so nobody is left muted, and the session resuming by itself when
// capture comes back.
func TestACaptureCrashFailsOpenAndRecoversOnReconnect(t *testing.T) {
	r := newRecovery(t)

	capture := r.connect(t, "session-a")
	capture.handshake(t, r.credential)
	capture.snapshot(t, protocol.PhaseTasks, protocol.Player{Name: "Red"})
	r.waitForPlayers(t, 1)

	r.controller.CaptureSessions.SetMode(guild, Running)

	// Capture dies. Nothing arrives for longer than the timeout.
	capture.conn.Close()
	r.controller.checkCaptureTimeouts(time.Now().Add(2 * time.Minute))

	if got := r.controller.CaptureSessions.Mode(guild); got != Paused {
		t.Fatalf("after a crash the session is %s, want paused", got)
	}

	// Capture comes back and the session picks up where it left off.
	returning := r.connect(t, "session-b")
	returning.handshake(t, r.credential)
	returning.snapshot(t, protocol.PhaseDiscussion, protocol.Player{Name: "Red"})

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if r.controller.CaptureSessions.Mode(guild) == Running {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Errorf("capture came back and the session is still %s",
		r.controller.CaptureSessions.Mode(guild))
}

// A reconnect that never sends a snapshot must not be allowed to drive the
// round with incremental events.
func TestAReconnectWithoutASnapshotCannotChangeTheRound(t *testing.T) {
	r := newRecovery(t)

	first := r.connect(t, "session-a")
	first.handshake(t, r.credential)
	first.snapshot(t, protocol.PhaseTasks, protocol.Player{Name: "Red"}, protocol.Player{Name: "Blue"})
	r.waitForPlayers(t, 2)
	first.conn.Close()

	second := r.connect(t, "session-b")
	second.handshake(t, r.credential)
	second.send(t, &protocol.PlayerDied{
		Header: second.header(protocol.TypePlayerDied),
		Player: protocol.Player{Name: "Red", Dead: true},
	})
	time.Sleep(100 * time.Millisecond)

	players := r.waitForPlayers(t, 2)
	for _, player := range players {
		if !player.Alive {
			t.Errorf("%s was killed by an event the bot should have refused", player.Name)
		}
	}
}
