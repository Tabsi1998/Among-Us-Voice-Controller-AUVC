package main

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/bot"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/pairing"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/gorilla/websocket"
)

// The transport and the pairing service each do their part of a revoke, and
// neither is any use if the listener does not connect them. This drives the
// wiring the running bot uses.
func TestRevokingEndsACaptureThatIsAlreadyConnected(t *testing.T) {
	const guild = "guild-1"

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "auvc.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	pairingService := pairing.NewService(db)
	controller := &bot.Bot{}
	capture := newCaptureServer(pairingService, controller)

	server := httptest.NewServer(routes(capture, controller))
	defer server.Close()

	code, _, err := pairingService.Pair(guild, "admin")
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	_, issued, err := pairingService.RedeemCode(code.Display())
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/capture/link", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	for _, message := range []protocol.Message{
		&protocol.Hello{
			Header:  protocol.Header{Protocol: protocol.Version, Type: protocol.TypeHello, Session: "s1", Seq: 1},
			Capture: "1.0.0",
		},
		&protocol.Authentication{
			Header:     protocol.Header{Protocol: protocol.Version, Type: protocol.TypeAuthentication, Session: "s1", Seq: 2},
			Credential: issued.Token().Reveal(),
		},
	} {
		encoded, err := protocol.Encode(message)
		if err != nil {
			t.Fatalf("encode: %v", err)
		}
		if err := conn.WriteMessage(websocket.TextMessage, encoded); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	waitUntil(t, func() bool { return capture.Connections(guild) == 1 })

	// This is the call /au capture revoke makes.
	if _, err := pairingService.Revoke(guild); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	closed := false
	for !closed {
		if _, _, err := conn.ReadMessage(); err != nil {
			closed = true
		}
	}
	waitUntil(t, func() bool { return capture.Connections(guild) == 0 })
}

func waitUntil(t *testing.T, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out")
}
