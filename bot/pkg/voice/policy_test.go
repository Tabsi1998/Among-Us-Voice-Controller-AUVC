package voice

import (
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/session"
)

const (
	main  = "main-channel"
	ghost = "ghost-channel"
)

// ghostChat is a guild configured the way the requirements default: ghost chat
// on and channel enforcement on.
func ghostChat() Config {
	return Config{
		MainChannelID:   main,
		GhostChannelID:  ghost,
		AutoMoveGhosts:  true,
		EnforceChannels: true,
	}
}

func stateWith(phase game.Phase, players ...session.PlayerState) session.State {
	return session.State{Phase: phase, Players: players}
}

func living(userID string) session.PlayerState {
	return session.PlayerState{UserID: userID, InGameName: "Player-" + userID, Alive: true}
}

// dead is a player whose death a meeting has already announced. Most of the
// policy is about what happens after that.
func dead(userID string) session.PlayerState {
	return session.PlayerState{
		UserID: userID, InGameName: "Player-" + userID, Alive: false, Revealed: true,
	}
}

// secretlyDead is a player who has just been killed and whose death nobody
// has been told about yet.
func secretlyDead(userID string) session.PlayerState {
	return session.PlayerState{UserID: userID, InGameName: "Player-" + userID, Alive: false}
}

// This is the ghost-chat table from docs/requirements.md, asserted cell by cell.
// It is the contract the whole product exists to implement, so every combination
// of phase and aliveness is covered rather than a representative sample.
func TestGhostChatTable(t *testing.T) {
	cases := []struct {
		name  string
		phase game.Phase
		alive bool
		want  DesiredVoiceState
	}{
		{"lobby, living", game.LOBBY, true, DesiredVoiceState{TargetChannelID: main}},
		{"lobby, dead", game.LOBBY, false, DesiredVoiceState{TargetChannelID: main}},

		{"tasks, living", game.TASKS, true,
			DesiredVoiceState{TargetChannelID: main, Muted: true, Deafened: true}},
		// A death the meeting already announced.
		{"tasks, dead", game.TASKS, false, DesiredVoiceState{TargetChannelID: ghost}},

		// Discussion covers meetings and voting; the game state reports both as
		// one phase.
		{"discussion, living", game.DISCUSS, true, DesiredVoiceState{TargetChannelID: main}},
		{"discussion, dead", game.DISCUSS, false, DesiredVoiceState{TargetChannelID: ghost}},

		{"ended, living", game.GAMEOVER, true, DesiredVoiceState{TargetChannelID: main}},
		{"ended, dead", game.GAMEOVER, false, DesiredVoiceState{TargetChannelID: main}},

		// No round is running.
		{"menu, living", game.MENU, true, DesiredVoiceState{TargetChannelID: main}},
		{"menu, dead", game.MENU, false, DesiredVoiceState{TargetChannelID: main}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			player := living("u")
			if !c.alive {
				player = dead("u")
			}

			got := Desired(stateWith(c.phase, player), ghostChat())["u"]
			if got != c.want {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

// Only the living are ever muted or deafened by the policy. A ghost that stays
// silenced after a meeting starts is the complaint that ends up as a bug report.
func TestOnlyLivingPlayersAreEverDeafened(t *testing.T) {
	for _, phase := range []game.Phase{game.LOBBY, game.TASKS, game.DISCUSS, game.MENU, game.GAMEOVER} {
		got := Desired(stateWith(phase, dead("u")), ghostChat())["u"]
		if got.Deafened {
			t.Errorf("phase %v: a dead player must never be deafened, got %+v", phase, got)
		}
	}
}

// The round ends by releasing everyone, including ghosts sitting in the ghost
// channel. Leaving them there is how players end up stranded after a match.
func TestEndOfRoundReturnsGhostsToMain(t *testing.T) {
	state := stateWith(game.GAMEOVER, living("alive"), dead("ghost-player"))

	desired := Desired(state, ghostChat())
	for userID, got := range desired {
		if got != (DesiredVoiceState{TargetChannelID: main}) {
			t.Errorf("%s: expected to be released to main, got %+v", userID, got)
		}
	}
}

// Unlinked users and bot accounts must not merely be ignored downstream; they
// must not be in the result at all. A player who is absent from the map cannot
// be moved by mistake.
func TestUnmanagedPlayersAreAbsentFromTheResult(t *testing.T) {
	state := stateWith(game.TASKS,
		living("player"),
		session.PlayerState{UserID: "spectator", Alive: true},
		session.PlayerState{UserID: "musicbot", InGameName: "Red", Alive: true, Bot: true},
	)

	desired := Desired(state, ghostChat())

	if len(desired) != 1 {
		t.Fatalf("expected exactly the linked human, got %d entries: %+v", len(desired), desired)
	}
	if _, ok := desired["spectator"]; ok {
		t.Error("an unlinked user must not appear in the result")
	}
	if _, ok := desired["musicbot"]; ok {
		t.Error("a bot account must not appear in the result")
	}
}

// With ghost moves switched off the dead stay in the main channel. They are
// silenced rather than left audible: the living stop being deafened the moment
// the phase changes, and a stray voice would give the round away.
func TestWithoutGhostMovesTheDeadAreSilencedInMain(t *testing.T) {
	config := Config{MainChannelID: main, GhostChannelID: ghost, AutoMoveGhosts: false, EnforceChannels: true}

	for _, phase := range []game.Phase{game.TASKS, game.DISCUSS} {
		got := Desired(stateWith(phase, dead("u")), config)["u"]

		want := DesiredVoiceState{TargetChannelID: main, Muted: true}
		if got != want {
			t.Errorf("phase %v: got %+v, want %+v", phase, got, want)
		}
	}
}

// A ghost channel that was never configured has to behave exactly like the
// feature being switched off. Emitting a move to an empty channel id would make
// the reconciler either fail per player or move somebody nowhere.
func TestAnUnconfiguredGhostChannelBehavesLikeTheFeatureBeingOff(t *testing.T) {
	config := Config{MainChannelID: main, AutoMoveGhosts: true, EnforceChannels: true}

	got := Desired(stateWith(game.TASKS, dead("u")), config)["u"]

	if got.TargetChannelID != main {
		t.Errorf("expected the dead player to stay in main, got %q", got.TargetChannelID)
	}
	if !got.Muted {
		t.Error("expected the dead player to be silenced when there is nowhere to move them")
	}
}

// Living players are never sent to the ghost channel, whatever the phase.
func TestLivingPlayersNeverTargetTheGhostChannel(t *testing.T) {
	for _, phase := range []game.Phase{game.LOBBY, game.TASKS, game.DISCUSS, game.MENU, game.GAMEOVER} {
		got := Desired(stateWith(phase, living("u")), ghostChat())["u"]
		if got.TargetChannelID == ghost {
			t.Errorf("phase %v: a living player was sent to the ghost channel", phase)
		}
	}
}

// The policy has to hold for a full lobby, not just one player at a time.
func TestMixedLobbyDuringTasks(t *testing.T) {
	state := stateWith(game.TASKS,
		living("a"), living("b"), dead("c"), dead("d"),
		session.PlayerState{UserID: "spectator", Alive: true},
	)

	desired := Desired(state, ghostChat())

	if len(desired) != 4 {
		t.Fatalf("expected 4 managed players, got %d", len(desired))
	}
	for _, userID := range []string{"a", "b"} {
		want := DesiredVoiceState{TargetChannelID: main, Muted: true, Deafened: true}
		if desired[userID] != want {
			t.Errorf("%s: got %+v, want %+v", userID, desired[userID], want)
		}
	}
	for _, userID := range []string{"c", "d"} {
		want := DesiredVoiceState{TargetChannelID: ghost}
		if desired[userID] != want {
			t.Errorf("%s: got %+v, want %+v", userID, desired[userID], want)
		}
	}
}

// The policy is a pure function: the same inputs must always give the same
// answer, and it must not touch what it was given.
func TestDesiredIsPureAndRepeatable(t *testing.T) {
	state := stateWith(game.TASKS, living("a"), dead("b"))
	config := ghostChat()

	first := Desired(state, config)
	second := Desired(state, config)

	if len(first) != len(second) {
		t.Fatalf("result size changed between calls: %d then %d", len(first), len(second))
	}
	for userID, want := range first {
		if second[userID] != want {
			t.Errorf("%s changed between calls: %+v then %+v", userID, want, second[userID])
		}
	}

	if state.Phase != game.TASKS || len(state.Players) != 2 {
		t.Error("the session state was modified")
	}
	if config != ghostChat() {
		t.Error("the configuration was modified")
	}
}

func TestEmptySessionProducesNoWork(t *testing.T) {
	if got := Desired(session.State{Phase: game.TASKS}, ghostChat()); len(got) != 0 {
		t.Errorf("expected no desired states, got %+v", got)
	}
}
