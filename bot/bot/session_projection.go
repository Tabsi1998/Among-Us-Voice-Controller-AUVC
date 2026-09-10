package bot

import (
	"sort"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/amongus"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/session"
)

// SessionState projects the Discord-coupled game state onto the pure session
// domain in pkg/session.
//
// This is the seam the architecture calls for: game event handlers own the
// session, and everything past this projection works on plain values with no
// Discord, Redis or storage types in reach. The voice policy added in phase 8
// consumes the result, which is what lets it be a pure function.
//
// Players are ordered by Discord user id so the projection is reproducible;
// the underlying user data is a map and would otherwise iterate at random.
func (dgs *GameState) SessionState() session.State {
	state := session.State{
		Phase:   dgs.GameData.GetPhase(),
		Players: make([]session.PlayerState, 0, len(dgs.UserData)),
	}

	for userID, user := range dgs.UserData {
		player := session.PlayerState{
			UserID: userID,
			Bot:    user.User.IsBot,
			// Unlinked users have no player to be dead, and reporting them as
			// alive keeps callers from mistaking them for corpses.
			Alive: true,
		}

		if name := user.InGameName; name != "" && name != amongus.UnlinkedPlayerName {
			player.InGameName = name
			if data, ok := dgs.GameData.GetByName(name); ok {
				player.Alive = data.IsAlive
			}
		}

		state.Players = append(state.Players, player)
	}

	sort.Slice(state.Players, func(i, j int) bool {
		return state.Players[i].UserID < state.Players[j].UserID
	})
	return state
}
