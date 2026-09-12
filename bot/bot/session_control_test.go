package bot

import (
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
)

func TestAGuildStartsStopped(t *testing.T) {
	sessions := NewCaptureSessions()

	if got := sessions.Mode("guild-1"); got != Stopped {
		t.Errorf("a fresh guild is %s, want stopped", got)
	}
}

func TestSettingTheModeReportsWhatItWas(t *testing.T) {
	sessions := NewCaptureSessions()

	if previous := sessions.SetMode("guild-1", Running); previous != Stopped {
		t.Errorf("got %s, want stopped", previous)
	}
	if previous := sessions.SetMode("guild-1", Paused); previous != Running {
		t.Errorf("got %s, want running", previous)
	}
	if got := sessions.Mode("guild-1"); got != Paused {
		t.Errorf("the guild is %s, want paused", got)
	}
}

func TestModesAreSeparatePerGuild(t *testing.T) {
	sessions := NewCaptureSessions()
	sessions.SetMode("guild-1", Running)

	if got := sessions.Mode("guild-2"); got != Stopped {
		t.Errorf("guild-2 is %s, want stopped", got)
	}
}

// Every mode has to print as something a person can read, because it reaches a
// Discord message and a log line.
func TestEveryModeHasAName(t *testing.T) {
	for mode, want := range map[Mode]string{
		Stopped: "stopped",
		Running: "running",
		Paused:  "paused",
	} {
		if got := mode.String(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

// A guild that has not asked AUVC to manage its voice must not have it taken
// over because a capture connected and sent a snapshot.
func TestAStoppedGuildIsNotTakenOverBySnapshot(t *testing.T) {
	controller := &Bot{CaptureSessions: NewCaptureSessions()}

	err := controller.HandleCapture("guild-1", &protocol.Snapshot{
		Phase:   protocol.PhaseTasks,
		Players: []protocol.Player{{Name: "Red"}},
	})
	if err != nil {
		t.Fatalf("handling a snapshot: %v", err)
	}

	if got := controller.CaptureSessions.Mode("guild-1"); got != Stopped {
		t.Errorf("the guild was taken over: %s", got)
	}
}

// The session keeps being tracked while paused, so resuming acts on the round
// as it is then rather than as it was when the pause started.
func TestAPausedSessionKeepsFollowingTheGame(t *testing.T) {
	controller := &Bot{CaptureSessions: NewCaptureSessions()}
	controller.CaptureSessions.SetMode("guild-1", Paused)

	if err := controller.HandleCapture("guild-1", &protocol.Snapshot{
		Phase:   protocol.PhaseTasks,
		Players: []protocol.Player{{Name: "Red"}, {Name: "Blue"}},
	}); err != nil {
		t.Fatalf("handling a snapshot: %v", err)
	}
	if err := controller.HandleCapture("guild-1", &protocol.PlayerDied{
		Player: protocol.Player{Name: "Red", Dead: true},
	}); err != nil {
		t.Fatalf("handling a death: %v", err)
	}

	mode, phase, players := controller.CaptureSessions.Snapshot("guild-1")
	if mode != Paused {
		t.Errorf("the mode changed to %s", mode)
	}
	if phase != game.TASKS {
		t.Errorf("phase is %v, want TASKS", phase)
	}
	if len(players) != 2 {
		t.Fatalf("expected two players, got %+v", players)
	}
	for _, player := range players {
		if player.Name == "Red" && player.Alive {
			t.Error("a death during a pause was not recorded")
		}
	}
}

// Pausing something that was never running would promise a resume that does
// something unexpected later.
func TestPausingReportsWhatItWasBefore(t *testing.T) {
	controller := &Bot{CaptureSessions: NewCaptureSessions()}

	if previous := controller.PauseSession("guild-1"); previous != Stopped {
		t.Errorf("got %s, want stopped", previous)
	}
	if previous := controller.PauseSession("guild-1"); previous != Paused {
		t.Errorf("got %s, want paused", previous)
	}
}

// A configuration that cannot be read must not cause a takeover. Grabbing a
// guild's voice channels because a database blinked is the wrong way to be
// wrong.
func TestAutoStartSaysNoWhenTheConfigurationIsUnreadable(t *testing.T) {
	controller := &Bot{CaptureSessions: NewCaptureSessions()}

	if controller.autoStart("guild-1") {
		t.Error("auto start must not be assumed when the configuration is unavailable")
	}
}
