package bot

import (
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
)

// These follow the order capture's memory reader really reports in, which the
// numbered scenarios do not: an exile arrives before the phase leaves the
// meeting, and the same death arrives again once the game marks the player dead.

// An exile happens in front of everyone, so the ghost channel is no secret.
func TestScenarioAnExiledPlayerGoesToTheGhostChannel(t *testing.T) {
	lobby := newLobby(t, "Red", "Blue", "Green")
	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseTasks,
		Players: []protocol.Player{alive("Red"), alive("Blue"), alive("Green")}})
	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseDiscussion})

	lobby.send(&protocol.PlayerDied{Player: protocol.Player{Name: "Blue", Dead: true}})
	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseTasks})
	lobby.send(&protocol.PlayerDied{Player: protocol.Player{Name: "Blue", Dead: true}})

	lobby.expect("Blue", openIn(ghostChannel))
	lobby.expect("Red", shutOut(mainChannel))
}

// A ghost whose details change is still a ghost everybody knows about. Treating
// the update as a fresh death would silence them in the ghost channel.
func TestScenarioAnUpdateDoesNotSilenceAnAnnouncedGhost(t *testing.T) {
	lobby := newLobby(t, "Red", "Blue")
	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseTasks,
		Players: []protocol.Player{alive("Red"), alive("Blue")}})
	lobby.send(&protocol.PlayerDied{Player: protocol.Player{Name: "Blue", Dead: true}})
	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseDiscussion})
	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseTasks})

	lobby.send(&protocol.PlayerChanged{Player: protocol.Player{Name: "Blue", Color: 4, Dead: true}})

	lobby.expect("Blue", openIn(ghostChannel))
}
