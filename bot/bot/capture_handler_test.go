package bot

import (
	"strings"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/session"
)

func apply(t *testing.T, live *session.Live, message protocol.Message) bool {
	t.Helper()

	changed, err := applyCaptureMessage(live, message)
	if err != nil {
		t.Fatalf("%s: %v", protocol.Envelope(message).Type, err)
	}
	return changed
}

// Every phase the protocol defines has to map onto one the bot knows, or
// capture can report a state the bot silently discards.
func TestEveryProtocolPhaseIsUnderstood(t *testing.T) {
	for _, phase := range protocol.Phases {
		t.Run(string(phase), func(t *testing.T) {
			mapped, ok := phaseFromProtocol(phase)
			if !ok {
				t.Fatalf("%s has no mapping", phase)
			}
			if mapped == game.UNINITIALIZED {
				t.Errorf("%s maps to UNINITIALIZED", phase)
			}
		})
	}
}

// The game reads the lobby only after the phase changed, so capture reports
// joining a lobby as a change into the same phase. That has to count as a
// change, because it is what redraws the crewmate board.
func TestALobbyAloneIsAChange(t *testing.T) {
	live := session.NewLive()
	header := protocol.Header{Protocol: protocol.Version, Type: protocol.TypeGameStateChanged, Session: "s", Seq: 1}
	polus := &protocol.Lobby{Code: "ABCDEF", Map: protocol.MapPolus}

	if !apply(t, live, &protocol.GameStateChanged{Header: header, Phase: protocol.PhaseTasks}) {
		t.Fatal("entering tasks changed nothing")
	}
	if !apply(t, live, &protocol.GameStateChanged{Header: header, Phase: protocol.PhaseTasks, Lobby: polus}) {
		t.Error("a new lobby in the same phase counted as no change")
	}
	if got := live.Lobby(); got != (session.Lobby{Code: "ABCDEF", Map: protocol.MapPolus}) {
		t.Errorf("the lobby is %+v", got)
	}
	if apply(t, live, &protocol.GameStateChanged{Header: header, Phase: protocol.PhaseTasks, Lobby: polus}) {
		t.Error("the same lobby again counted as a change")
	}

	// Leaving for the menu carries no lobby, and none is kept.
	if !apply(t, live, &protocol.GameStateChanged{Header: header, Phase: protocol.PhaseMenu}) || live.Lobby() != (session.Lobby{}) {
		t.Errorf("the menu kept the lobby %+v", live.Lobby())
	}
}

func TestASnapshotBringsItsLobby(t *testing.T) {
	live := session.NewLive()

	apply(t, live, &protocol.Snapshot{
		Header:  protocol.Header{Protocol: protocol.Version, Type: protocol.TypeSnapshot, Session: "s", Seq: 1},
		Phase:   protocol.PhaseLobby,
		Players: []protocol.Player{{Name: "Red"}},
		Lobby:   &protocol.Lobby{Code: "ABCD", Map: protocol.MapFungle},
	})

	if got := live.Lobby(); got != (session.Lobby{Code: "ABCD", Map: protocol.MapFungle}) {
		t.Errorf("the lobby is %+v", got)
	}
}

func TestAnUnknownPhaseIsRefusedRatherThanGuessed(t *testing.T) {
	if _, ok := phaseFromProtocol("voting"); ok {
		t.Error("an unknown phase must not be mapped")
	}

	live := session.NewLive()
	_, err := applyCaptureMessage(live, &protocol.GameStateChanged{Phase: "voting"})
	if err == nil {
		t.Error("an unknown phase must be an error, not a silent no-op")
	}
}

// The protocol reports death and the session tracks life, so the inversion has
// to be right or every player is the opposite of what capture said.
func TestDeadOnTheWireBecomesNotAliveInTheSession(t *testing.T) {
	alive := playerFromProtocol(protocol.Player{Name: "Red", Dead: false})
	dead := playerFromProtocol(protocol.Player{Name: "Blue", Dead: true})

	if !alive.Alive {
		t.Error("a living player arrived as dead")
	}
	if dead.Alive {
		t.Error("a dead player arrived as alive")
	}
}

func TestASnapshotSetsThePhaseAndThePlayers(t *testing.T) {
	live := session.NewLive()

	if !apply(t, live, &protocol.Snapshot{
		Phase: protocol.PhaseTasks,
		Players: []protocol.Player{
			{Name: "Red"},
			{Name: "Blue", Dead: true},
		},
	}) {
		t.Fatal("a snapshot always changes the picture")
	}

	if live.Phase() != game.TASKS {
		t.Errorf("phase is %v, want TASKS", live.Phase())
	}

	players := live.Players()
	if len(players) != 2 {
		t.Fatalf("expected two players, got %+v", players)
	}
	if players[0].Name != "Blue" || players[0].Alive {
		t.Errorf("Blue should be dead: %+v", players[0])
	}
}

// A reconnect sends a fresh snapshot, and the bot must not keep players who are
// no longer in the game.
func TestASnapshotReplacesWhatWasThereBefore(t *testing.T) {
	live := session.NewLive()
	apply(t, live, &protocol.Snapshot{
		Phase:   protocol.PhaseTasks,
		Players: []protocol.Player{{Name: "Red"}, {Name: "Blue"}},
	})

	apply(t, live, &protocol.Snapshot{
		Phase:   protocol.PhaseLobby,
		Players: []protocol.Player{{Name: "Red"}},
	})

	if got := len(live.Players()); got != 1 {
		t.Errorf("expected the snapshot to replace the lobby, got %d players", got)
	}
}

// Reconciling costs a Discord round trip, so a message that changes nothing
// must say so.
func TestOnlyRealChangesAskForAReconciliation(t *testing.T) {
	live := session.NewLive()
	apply(t, live, &protocol.Snapshot{
		Phase:   protocol.PhaseTasks,
		Players: []protocol.Player{{Name: "Red"}},
	})

	if apply(t, live, &protocol.GameStateChanged{Phase: protocol.PhaseTasks}) {
		t.Error("a phase that did not change must not trigger a reconciliation")
	}
	if !apply(t, live, &protocol.GameStateChanged{Phase: protocol.PhaseDiscussion}) {
		t.Error("a real phase change must trigger a reconciliation")
	}
}

// A heartbeat says nothing about the round. Reconciling on one would mean
// talking to Discord every few seconds for no reason.
func TestAHeartbeatChangesNothing(t *testing.T) {
	live := session.NewLive()
	apply(t, live, &protocol.Snapshot{
		Phase:   protocol.PhaseTasks,
		Players: []protocol.Player{{Name: "Red"}},
	})

	if apply(t, live, &protocol.Heartbeat{}) {
		t.Error("a heartbeat must not trigger a reconciliation")
	}
	if live.Phase() != game.TASKS {
		t.Error("a heartbeat must not change the session")
	}
}

func TestADeathIsRecorded(t *testing.T) {
	live := session.NewLive()
	apply(t, live, &protocol.Snapshot{
		Phase:   protocol.PhaseTasks,
		Players: []protocol.Player{{Name: "Red"}},
	})

	apply(t, live, &protocol.PlayerDied{Player: protocol.Player{Name: "Red", Dead: true}})

	if live.Players()[0].Alive {
		t.Error("Red died and is still alive in the session")
	}
}

func TestAPlayerWhoLeavesIsDropped(t *testing.T) {
	live := session.NewLive()
	apply(t, live, &protocol.Snapshot{
		Phase:   protocol.PhaseTasks,
		Players: []protocol.Player{{Name: "Red"}, {Name: "Blue"}},
	})

	apply(t, live, &protocol.PlayerLeft{Player: protocol.Player{Name: "Blue"}})

	if got := len(live.Players()); got != 1 {
		t.Errorf("expected Blue to be gone, got %+v", live.Players())
	}
}

// The end of a round releases everyone: the phase changes and the dead come
// back, which is what lets the voice policy bring the ghosts home.
func TestTheEndOfARoundRevivesEveryone(t *testing.T) {
	live := session.NewLive()
	apply(t, live, &protocol.Snapshot{
		Phase:   protocol.PhaseTasks,
		Players: []protocol.Player{{Name: "Red", Dead: true}, {Name: "Blue"}},
	})

	apply(t, live, &protocol.GameEnded{})

	if live.Phase() != game.GAMEOVER {
		t.Errorf("phase is %v, want GAMEOVER", live.Phase())
	}
	for _, player := range live.Players() {
		if !player.Alive {
			t.Errorf("%s is still dead after the round ended", player.Name)
		}
	}
}

// A full round, driven exactly as capture would drive it.
func TestARoundPlaysThrough(t *testing.T) {
	live := session.NewLive()

	apply(t, live, &protocol.Snapshot{
		Phase:   protocol.PhaseLobby,
		Players: []protocol.Player{{Name: "Red"}, {Name: "Blue"}},
	})
	apply(t, live, &protocol.PlayerJoined{Player: protocol.Player{Name: "Green"}})
	apply(t, live, &protocol.GameStateChanged{Phase: protocol.PhaseTasks})
	apply(t, live, &protocol.PlayerDied{Player: protocol.Player{Name: "Green", Dead: true}})
	apply(t, live, &protocol.GameStateChanged{Phase: protocol.PhaseDiscussion})
	apply(t, live, &protocol.GameEnded{})

	if live.Phase() != game.GAMEOVER {
		t.Errorf("phase is %v, want GAMEOVER", live.Phase())
	}
	if got := len(live.Players()); got != 3 {
		t.Errorf("expected three players at the end, got %+v", live.Players())
	}
}

// A message with no guild cannot be applied to anything, and guessing would
// mean driving the voice of a server nobody named.
func TestAMessageWithoutAGuildIsRefused(t *testing.T) {
	controller := &Bot{CaptureSessions: NewCaptureSessions()}

	err := controller.HandleCapture("", &protocol.Heartbeat{})
	if err == nil {
		t.Fatal("a message with no guild must be refused")
	}
	if !strings.Contains(err.Error(), "guild") {
		t.Errorf("the error should say what is missing: %v", err)
	}
}

// Two guilds must not share a session. A death in one server would otherwise
// mute somebody in another.
func TestGuildsKeepSeparateSessions(t *testing.T) {
	sessions := NewCaptureSessions()

	first := sessions.forGuild("guild-1")
	second := sessions.forGuild("guild-2")

	apply(t, first.live, &protocol.Snapshot{
		Phase:   protocol.PhaseTasks,
		Players: []protocol.Player{{Name: "Red"}},
	})

	if _, phase, players := sessions.Snapshot("guild-2"); len(players) != 0 || phase == game.TASKS {
		t.Errorf("guild-2 saw guild-1's session: phase=%v players=%+v", phase, players)
	}
	if second.live.Phase() == game.TASKS {
		t.Error("the second guild inherited the first guild's phase")
	}
}

func TestTheSameGuildKeepsTheSameSession(t *testing.T) {
	sessions := NewCaptureSessions()

	first := sessions.forGuild("guild-1")
	again := sessions.forGuild("guild-1")

	if first != again {
		t.Error("a guild got a different session on the second lookup")
	}
}
