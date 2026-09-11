package voice

import (
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/session"
)

// unenforced is a guild that still wants ghost chat but has handed the choice
// of channel back to its players.
func unenforced() Config {
	config := ghostChat()
	config.EnforceChannels = false
	return config
}

// plan is what actually reaches Discord: the policy's answer for one player,
// compared against where that player is observed to be.
func plan(t *testing.T, phase game.Phase, config Config, player session.PlayerState, observed Observed) []Change {
	t.Helper()

	return Diff(
		map[string]Observed{player.UserID: observed},
		Desired(stateWith(phase, player), config),
	)
}

func onlyChange(t *testing.T, changes []Change) Change {
	t.Helper()

	if len(changes) != 1 {
		t.Fatalf("expected exactly one change, got %+v", changes)
	}
	return changes[0]
}

// The headline case from docs/requirements.md: a living player who walks into
// the ghost channel is sent back to main. Without this they sit and listen to
// the dead discussing who killed them.
func TestALivingPlayerWhoWalksIntoTheGhostChannelIsReturnedToMain(t *testing.T) {
	for _, phase := range []game.Phase{game.TASKS, game.DISCUSS} {
		changes := plan(t, phase, ghostChat(), living("u"), Observed{ChannelID: ghost})

		change := onlyChange(t, changes)
		if change.MoveTo == nil {
			t.Fatalf("phase %v: expected a move back to main, got %+v", phase, change)
		}
		if *change.MoveTo != main {
			t.Errorf("phase %v: moved to %q, want %q", phase, *change.MoveTo, main)
		}
	}
}

// The other half of the same requirement. This one is also what a death looks
// like, which is why it holds whether the player wandered or just died.
func TestADeadPlayerWhoWalksIntoMainIsReturnedToGhost(t *testing.T) {
	for _, phase := range []game.Phase{game.TASKS, game.DISCUSS} {
		changes := plan(t, phase, ghostChat(), dead("u"), Observed{ChannelID: main})

		change := onlyChange(t, changes)
		if change.MoveTo == nil {
			t.Fatalf("phase %v: expected a move to the ghost channel, got %+v", phase, change)
		}
		if *change.MoveTo != ghost {
			t.Errorf("phase %v: moved to %q, want %q", phase, *change.MoveTo, ghost)
		}
	}
}

// The admin override. With enforcement off the bot stops deciding where a
// living player sits, and still decides what they can say and hear: a player
// who wandered off during tasks stays deafened, so wandering is not a way to
// listen in.
func TestEnforcementOffLeavesALivingPlayerWhereTheyChose(t *testing.T) {
	changes := plan(t, game.TASKS, unenforced(), living("u"), Observed{ChannelID: ghost})

	for _, change := range changes {
		if change.MoveTo != nil {
			t.Errorf("enforcement is off, nobody may be moved: %+v", change)
		}
	}

	change := onlyChange(t, changes)
	if change.Muted == nil || !*change.Muted {
		t.Errorf("a living player during tasks must still be muted, got %+v", change)
	}
	if change.Deafened == nil || !*change.Deafened {
		t.Errorf("a living player during tasks must still be deafened, got %+v", change)
	}
}

// Enforcement off must not quietly switch ghost chat off with it. Moving the
// dead into the ghost channel is the feature auto_move_ghosts governs, not a
// correction of somebody who wandered.
func TestEnforcementOffStillMovesTheDeadIntoTheGhostChannel(t *testing.T) {
	changes := plan(t, game.TASKS, unenforced(), dead("u"), Observed{ChannelID: main})

	change := onlyChange(t, changes)
	if change.MoveTo == nil || *change.MoveTo != ghost {
		t.Errorf("ghost chat must survive enforcement being off, got %+v", change)
	}
}

// Whatever the guild configured, a round has to end with everyone back
// together. Leaving enforcement to decide this would strand ghosts in the ghost
// channel after the match.
func TestEnforcementOffStillReturnsGhostsAtTheEndOfTheRound(t *testing.T) {
	for _, phase := range []game.Phase{game.LOBBY, game.MENU, game.GAMEOVER} {
		changes := plan(t, phase, unenforced(), dead("u"), Observed{ChannelID: ghost})

		change := onlyChange(t, changes)
		if change.MoveTo == nil || *change.MoveTo != main {
			t.Errorf("phase %v: a ghost must be brought back to main, got %+v", phase, change)
		}
	}
}

// A music bot parked in the ghost channel is the exact accident enforcement
// could cause. Unmanaged players are absent from the policy's answer, so there
// is nothing for the reconciler to act on.
func TestEnforcementNeverTouchesBotsOrUnlinkedUsers(t *testing.T) {
	state := stateWith(game.TASKS,
		session.PlayerState{UserID: "musicbot", InGameName: "Red", Alive: true, Bot: true},
		session.PlayerState{UserID: "spectator", Alive: true},
	)
	observed := map[string]Observed{
		"musicbot":  {ChannelID: ghost},
		"spectator": {ChannelID: ghost},
	}

	if changes := Diff(observed, Desired(state, ghostChat())); len(changes) != 0 {
		t.Errorf("enforcement must not reach unmanaged players, got %+v", changes)
	}
}

// A move produces a voice state update, which produces another observation. If
// that second pass still wanted a move the bot would chase the player around
// the server forever.
func TestEnforcingAChannelTwiceIsANoOp(t *testing.T) {
	player := living("u")
	first := plan(t, game.TASKS, ghostChat(), player, Observed{ChannelID: ghost})

	change := onlyChange(t, first)
	settled := Observed{ChannelID: *change.MoveTo, Muted: *change.Muted, Deafened: *change.Deafened}

	if again := plan(t, game.TASKS, ghostChat(), player, settled); len(again) != 0 {
		t.Errorf("enforcement looped: %+v", again)
	}
}

// Someone who left voice entirely cannot be enforced anywhere. Sending a move
// for them would fail, and sending an empty channel id would be read as a
// disconnect.
func TestAPlayerWhoLeftVoiceIsNotEnforced(t *testing.T) {
	if changes := plan(t, game.TASKS, ghostChat(), living("u"), Observed{}); len(changes) != 0 {
		t.Errorf("a player who is not in voice must be left alone, got %+v", changes)
	}
}
