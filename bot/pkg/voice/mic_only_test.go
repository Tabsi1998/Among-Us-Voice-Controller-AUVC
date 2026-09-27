package voice

import (
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/session"
)

// What the living lose during the tasks is the guild's choice since #171: the
// microphone always, the headphones only when the guild asks for it.
func TestTheLivingKeepTheirHeadphonesUnlessTheGuildAsks(t *testing.T) {
	state := stateWith(game.TASKS,
		session.PlayerState{UserID: "living", InGameName: "Red", Alive: true})

	for _, tc := range []struct {
		name     string
		config   Config
		deafened bool
	}{
		{"mic only, the default", micOnly(), false},
		{"deafening as well", ghostChat(), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := DesiredVoiceState{TargetChannelID: main, Muted: true, Deafened: tc.deafened}
			if got := Desired(state, tc.config)["living"]; got != want {
				t.Errorf("a living player is %+v, want %+v", got, want)
			}
		})
	}
}

// Keeping the headphones must not give the round away: a death nobody has been
// told about is silenced exactly as before, and the living hear nothing of it.
func TestKeepingTheHeadphonesStillSilencesASecretDeath(t *testing.T) {
	state := stateWith(game.TASKS,
		session.PlayerState{UserID: "living", InGameName: "Red", Alive: true},
		session.PlayerState{UserID: "secret", InGameName: "Blue"},
		session.PlayerState{UserID: "ghost", InGameName: "Green", Revealed: true})

	desired := Desired(state, micOnly())

	if got := desired["secret"]; got.TargetChannelID != "" || !got.Muted || got.Deafened {
		t.Errorf("a secret death is %+v, want left where they are and muted", got)
	}
	if got := desired["ghost"]; got.TargetChannelID != ghost || got.Muted || got.Deafened {
		t.Errorf("an announced death is %+v, want open in the ghost channel", got)
	}
}

// A meeting is unaffected either way: everybody alive can talk and hear.
func TestAMeetingIsTheSameWithAndWithoutDeafening(t *testing.T) {
	state := stateWith(game.DISCUSS,
		session.PlayerState{UserID: "living", InGameName: "Red", Alive: true})

	want := DesiredVoiceState{TargetChannelID: main}
	if got := Desired(state, micOnly())["living"]; got != want {
		t.Errorf("mic only: %+v, want %+v", got, want)
	}
	if got := Desired(state, ghostChat())["living"]; got != want {
		t.Errorf("with deafening: %+v, want %+v", got, want)
	}
}
