package voice

import (
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
)

// The point of the whole rule. A channel change is visible to everyone in
// Discord, so moving the victim the moment they die announces the kill to the
// server while the survivors are still meant to be guessing.
func TestAFreshKillIsNotAnnouncedByAMove(t *testing.T) {
	got := Desired(stateWith(game.TASKS, secretlyDead("u")), ghostChat())["u"]

	if got.TargetChannelID != "" {
		t.Errorf("a secret death produced a move to %q", got.TargetChannelID)
	}
}

// Silencing them is not about what the living can hear right now, since they
// are deafened during tasks. It is about what happens if that deafen fails.
func TestAFreshlyKilledPlayerIsSilenced(t *testing.T) {
	got := Desired(stateWith(game.TASKS, secretlyDead("u")), ghostChat())["u"]

	if !got.Muted {
		t.Errorf("a secretly dead player must not be able to talk, got %+v", got)
	}
	if got.Deafened {
		t.Errorf("a dead player must never be deafened, got %+v", got)
	}
}

// The meeting is where the game tells everyone. From then on the move is free.
func TestTheMeetingIsWhereTheGhostChannelFills(t *testing.T) {
	secret := Desired(stateWith(game.TASKS, secretlyDead("u")), ghostChat())["u"]
	if secret.TargetChannelID == ghost {
		t.Fatal("a secret death reached the ghost channel during tasks")
	}

	meeting := Desired(stateWith(game.DISCUSS, secretlyDead("u")), ghostChat())["u"]
	if meeting.TargetChannelID != ghost {
		t.Errorf("a meeting must move the dead to the ghost channel, got %+v", meeting)
	}
}

// After the meeting the death is public, so the ghost keeps the ghost channel
// through the following task phase. Sending them back to main would be the same
// leak in reverse and would cost them ghost chat for the rest of the game.
func TestAnAnnouncedDeathKeepsTheGhostChannelDuringTasks(t *testing.T) {
	got := Desired(stateWith(game.TASKS, dead("u")), ghostChat())["u"]

	if got.TargetChannelID != ghost {
		t.Errorf("an announced death should stay in the ghost channel, got %+v", got)
	}
	if got.Muted {
		t.Errorf("ghosts talk among themselves, got %+v", got)
	}
}

// Two players die before anyone calls a meeting. Neither may be moved, or the
// ghost channel fills up in plain sight.
func TestSeveralSecretDeathsStayHidden(t *testing.T) {
	state := stateWith(game.TASKS,
		living("survivor"),
		secretlyDead("first"),
		secretlyDead("second"),
	)

	desired := Desired(state, ghostChat())
	for _, userID := range []string{"first", "second"} {
		if desired[userID].TargetChannelID != "" {
			t.Errorf("%s was moved before the meeting: %+v", userID, desired[userID])
		}
	}
	if desired["survivor"].TargetChannelID != main {
		t.Errorf("the survivor should be in main, got %+v", desired["survivor"])
	}
}

// With no ghost channel there is nothing to move anyone into, so secrecy costs
// nothing and the dead are silenced either way.
func TestSecrecyChangesNothingWithoutAGhostChannel(t *testing.T) {
	config := Config{MainChannelID: main, AutoMoveGhosts: false, EnforceChannels: true}

	secret := Desired(stateWith(game.TASKS, secretlyDead("u")), config)["u"]
	announced := Desired(stateWith(game.TASKS, dead("u")), config)["u"]

	if !secret.Muted || !announced.Muted {
		t.Errorf("both should be silenced: secret=%+v announced=%+v", secret, announced)
	}
}

// A round ends and everybody is alive again, so the question does not arise.
func TestSecrecyDoesNotOutliveTheRound(t *testing.T) {
	for _, phase := range []game.Phase{game.LOBBY, game.MENU, game.GAMEOVER} {
		got := Desired(stateWith(phase, secretlyDead("u")), ghostChat())["u"]

		if got.TargetChannelID != main {
			t.Errorf("phase %v: everyone comes back to main, got %+v", phase, got)
		}
		if got.Muted || got.Deafened {
			t.Errorf("phase %v: nobody stays silenced between rounds, got %+v", phase, got)
		}
	}
}
