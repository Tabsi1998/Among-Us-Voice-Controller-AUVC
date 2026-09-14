package bot

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/voice"
	"github.com/gorilla/websocket"
)

// recordedRound is a whole fifteen-player round exactly as capture sends it,
// after the hello and the authentication. The C# tests check that capture
// produces these messages from the memory reader's events; this test plays them
// into the bot over the real connection. Capture and bot therefore cannot come
// to disagree about a round without one side failing.
const recordedRound = "../../protocol/fixtures/rounds/fifteen_players.jsonl"

var roundPlayers = []string{"Ava", "Ben", "Cleo", "Dario", "Elif", "Finn", "Greta", "Hugo",
	"Ines", "Jonas", "Kira", "Luca", "Mia", "Noah", "Omar"}

var roundConfig = voice.Config{
	MainChannelID:   mainChannel,
	GhostChannelID:  ghostChannel,
	AutoMoveGhosts:  true,
	EnforceChannels: true,
}

// notManaged marks a player the voice policy must leave entirely alone.
var notManaged = voice.Observed{ChannelID: "(not managed)"}

// silencedWhereTheyAre is a death nobody has been told about: muted, with no
// channel to move to, because a move would announce the kill.
var silencedWhereTheyAre = voice.Observed{Muted: true}

// roundCheck is what the round has to look like once a message is applied.
type roundCheck struct {
	phase game.Phase
	voice map[string]voice.Observed
}

func everyone(where voice.Observed) map[string]voice.Observed {
	all := make(map[string]voice.Observed, len(roundPlayers))
	for _, name := range roundPlayers {
		all[name] = where
	}
	return all
}

func TestARecordedFifteenPlayerRoundPlaysThroughTheRealConnection(t *testing.T) {
	r := newRecovery(t)
	db, ok := r.controller.AUVCLinks.(*sqlite.DB)
	if !ok {
		t.Fatal("the test bot has no database for the links")
	}
	for _, name := range roundPlayers {
		if err := db.SaveLink(guild, name, userFor(name)); err != nil {
			t.Fatalf("link %s: %v", name, err)
		}
	}

	afterTheRound := map[string]voice.Observed{
		"Ava": openIn(mainChannel), "Cleo": openIn(mainChannel), "Elif": openIn(mainChannel),
		"Kira": openIn(mainChannel), "Luca": notManaged, "Omar": notManaged,
	}
	checks := map[uint64]roundCheck{
		// The lobby: everyone open in main.
		3: {game.LOBBY, everyone(openIn(mainChannel))},
		// Tasks: the living cannot talk or hear.
		4: {game.TASKS, everyone(shutOut(mainChannel))},
		// A kill: Cleo is silenced wherever she is, so nothing announces it.
		5: {game.TASKS, map[string]voice.Observed{"Cleo": silencedWhereTheyAre, "Ava": shutOut(mainChannel)}},
		// Omar quits mid-round and is left alone from then on.
		6: {game.TASKS, map[string]voice.Observed{"Omar": notManaged, "Cleo": silencedWhereTheyAre}},
		// The meeting announces Cleo's death.
		7: {game.DISCUSS, map[string]voice.Observed{"Cleo": openIn(ghostChannel), "Ava": openIn(mainChannel)}},
		// Elif is voted out in front of everyone: straight to the ghost channel.
		8: {game.DISCUSS, map[string]voice.Observed{"Elif": openIn(ghostChannel), "Ava": openIn(mainChannel)}},
		9: {game.TASKS, map[string]voice.Observed{
			"Elif": openIn(ghostChannel), "Cleo": openIn(ghostChannel), "Ava": shutOut(mainChannel)}},
		// The game reports the exile again once it marks Elif dead. She stays a ghost.
		10: {game.TASKS, map[string]voice.Observed{"Elif": openIn(ghostChannel)}},
		// A second kill, again unannounced.
		11: {game.TASKS, map[string]voice.Observed{"Kira": silencedWhereTheyAre, "Elif": openIn(ghostChannel)}},
		// Luca's connection drops: he is not in the round any more.
		12: {game.TASKS, map[string]voice.Observed{"Luca": notManaged, "Kira": silencedWhereTheyAre}},
		// The round ends: everyone still playing is back together, open.
		13: {game.GAMEOVER, afterTheRound},
		15: {game.LOBBY, afterTheRound},
	}

	capture := r.connect(t, "session-a")
	capture.handshake(t, r.credential)

	checked := 0
	for index, line := range recordedLines(t) {
		message, err := protocol.Decode(line)
		if err != nil {
			t.Fatalf("line %d of the recording is not a valid message: %v", index+1, err)
		}
		header := protocol.Envelope(message)
		if want := uint64(index) + 3; header.Session != capture.session || header.Seq != want {
			t.Fatalf("line %d carries session %q seq %d, want %q seq %d",
				index+1, header.Session, header.Seq, capture.session, want)
		}

		if err := capture.conn.WriteMessage(websocket.TextMessage, line); err != nil {
			t.Fatalf("send line %d: %v", index+1, err)
		}
		capture.seq = header.Seq

		if want, ok := checks[header.Seq]; ok {
			r.expectRound(t, header, want)
			checked++
		}
	}

	if checked != len(checks) {
		t.Errorf("checked %d points of the round, want %d: the recording is shorter than the checks", checked, len(checks))
	}
}

func recordedLines(t *testing.T) [][]byte {
	t.Helper()

	data, err := os.ReadFile(recordedRound)
	if err != nil {
		t.Fatalf("read the recording: %v", err)
	}
	var lines [][]byte
	for _, line := range bytes.Split(data, []byte("\n")) {
		if line = bytes.TrimSpace(line); len(line) > 0 {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		t.Fatal("the recording is empty")
	}
	return lines
}

// expectRound waits until the bot has applied a message and the voice policy
// decides what want says. The server applies messages on its own goroutine.
func (r *recovery) expectRound(t *testing.T, header protocol.Header, want roundCheck) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		problems := r.roundProblems(t, want)
		if len(problems) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("after message %d (%s):\n  %s", header.Seq, header.Type, strings.Join(problems, "\n  "))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (r *recovery) roundProblems(t *testing.T, want roundCheck) []string {
	t.Helper()

	resolve, err := r.controller.linkResolver(guild)
	if err != nil {
		t.Fatalf("resolve links: %v", err)
	}
	session := r.controller.CaptureSessions.forGuild(guild)
	session.mu.Lock()
	state := session.live.Project(resolve)
	phase := session.live.Phase()
	session.mu.Unlock()
	desired := voice.Desired(state, roundConfig)

	var problems []string
	if phase != want.phase {
		problems = append(problems, fmt.Sprintf("the phase is %v, want %v", phase, want.phase))
	}

	names := make([]string, 0, len(want.voice))
	for name := range want.voice {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		got := notManaged
		if decided, managed := desired[userFor(name)]; managed {
			got = voice.Observed{ChannelID: decided.TargetChannelID, Muted: decided.Muted, Deafened: decided.Deafened}
		}
		if got != want.voice[name] {
			problems = append(problems, fmt.Sprintf("%s is %+v, want %+v", name, got, want.voice[name]))
		}
	}
	return problems
}
