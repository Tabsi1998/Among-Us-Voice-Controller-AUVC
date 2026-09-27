package bot

import (
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
)

// The same round as the required scenarios, with the setting the app now
// defaults to: the living lose the microphone and keep their headphones, and
// the round still gives nothing away.
func TestScenarioWithoutDeafeningTheLivingStillHearNothingOfTheDead(t *testing.T) {
	lobby := newLobby(t, "Red", "Blue", "Green")
	lobby.config.DeafenDuringTasks = false

	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseLobby,
		Players: []protocol.Player{alive("Red"), alive("Blue"), alive("Green")}})
	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseTasks})

	for _, name := range []string{"Red", "Blue", "Green"} {
		lobby.expect(name, silenced(mainChannel))
	}

	// A kill during the tasks is still silent and still does not move anybody.
	lobby.send(&protocol.PlayerDied{Player: protocol.Player{Name: "Green", Dead: true}})
	lobby.expect("Green", silenced(mainChannel))
	lobby.expect("Red", silenced(mainChannel))

	// The meeting announces the death: the ghost goes to the ghost channel and
	// the living can talk again.
	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseDiscussion})
	lobby.expect("Green", openIn(ghostChannel))
	lobby.expect("Red", openIn(mainChannel))

	// Back to the tasks: the living are muted again, never deafened.
	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseTasks})
	lobby.expect("Red", silenced(mainChannel))
	lobby.expect("Blue", silenced(mainChannel))
	lobby.expect("Green", openIn(ghostChannel))

	// The round ends with everybody back together and audible.
	lobby.send(&protocol.GameEnded{})
	for _, name := range []string{"Red", "Blue", "Green"} {
		lobby.expect(name, openIn(mainChannel))
	}
}
