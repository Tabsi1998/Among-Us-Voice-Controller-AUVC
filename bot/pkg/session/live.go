package session

import (
	"sort"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
)

// GamePlayer is a player exactly as the game reports them: an in-game name, a
// colour and whether they are still alive.
//
// There is no Discord identity here. Capture reads the game and knows nothing
// about Discord; joining the two is the bot's job and happens above this.
type GamePlayer struct {
	Name         string
	Color        int
	Alive        bool
	Disconnected bool
}

// Live is the mutable session a capture stream drives.
//
// It is the counterpart to State: events arrive here, and State is what comes
// out for the voice policy to act on. Keeping the mutation separate from the
// projection is what lets the policy stay a pure function over a value.
//
// Nothing in here is safe for concurrent use. One capture session produces one
// ordered stream of messages, and the caller serializes per guild.
type Live struct {
	phase   game.Phase
	players map[string]GamePlayer
}

// NewLive returns an empty session, before any snapshot has arrived.
func NewLive() *Live {
	return &Live{phase: game.LOBBY, players: map[string]GamePlayer{}}
}

// Phase returns the phase the session is in.
func (l *Live) Phase() game.Phase { return l.phase }

// Players returns every player the session knows, ordered by name so that a
// projection is reproducible and a log is readable.
func (l *Live) Players() []GamePlayer {
	players := make([]GamePlayer, 0, len(l.players))
	for _, player := range l.players {
		players = append(players, player)
	}

	sort.Slice(players, func(i, j int) bool { return players[i].Name < players[j].Name })
	return players
}

// Reset applies a complete snapshot.
//
// It replaces the player set rather than merging into it, which is the whole
// reason snapshots exist: after a reconnect the bot cannot know which of the
// players it remembers are still in the game, and keeping a stale one would
// mean managing the voice of somebody who left.
func (l *Live) Reset(phase game.Phase, players []GamePlayer) {
	l.phase = phase
	l.players = make(map[string]GamePlayer, len(players))

	for _, player := range players {
		if player.Name == "" {
			continue
		}
		l.players[player.Name] = player
	}
}

// SetPhase records a phase transition and reports whether it changed anything.
func (l *Live) SetPhase(phase game.Phase) bool {
	if l.phase == phase {
		return false
	}
	l.phase = phase
	return true
}

// Upsert adds or replaces one player.
//
// A player who is not in the session yet is added rather than ignored. Capture
// can report a change for somebody the bot missed, and refusing to learn about
// them would leave one player unmanaged for the rest of the round.
func (l *Live) Upsert(player GamePlayer) {
	if player.Name == "" {
		return
	}
	l.players[player.Name] = player
}

// Remove drops a player who left.
func (l *Live) Remove(name string) {
	delete(l.players, name)
}

// EndRound puts the session back to its between-rounds state.
//
// The players are kept. A round ending is not everybody leaving, and dropping
// them would mean the bot stops recognising the same lobby a moment later. Only
// the phase changes, which is what releases everyone in the voice policy.
func (l *Live) EndRound() {
	l.phase = game.GAMEOVER

	for name, player := range l.players {
		player.Alive = true
		l.players[name] = player
	}
}

// Project joins the session with Discord identities and produces the value the
// voice policy consumes.
//
// The resolver answers "which Discord user is this player, and is that user a
// bot account". A player it cannot resolve is left out entirely: an unmanaged
// player is one the policy cannot move by mistake, which is stricter and safer
// than carrying them through and filtering later.
//
// Disconnected players are left out for the same reason. They are not in the
// game any more, and a disconnect is exactly when somebody might legitimately
// be sitting in another channel.
func (l *Live) Project(resolve func(inGameName string) (userID string, isBot bool, ok bool)) State {
	state := State{Phase: l.phase, Players: make([]PlayerState, 0, len(l.players))}

	for _, player := range l.Players() {
		if player.Disconnected {
			continue
		}

		userID, isBot, ok := resolve(player.Name)
		if !ok || userID == "" {
			continue
		}

		state.Players = append(state.Players, PlayerState{
			UserID:     userID,
			InGameName: player.Name,
			Alive:      player.Alive,
			Bot:        isBot,
		})
	}

	// Players() is ordered by in-game name; the projection is keyed by Discord
	// user, so it is ordered that way to stay reproducible for its own callers.
	sort.Slice(state.Players, func(i, j int) bool {
		return state.Players[i].UserID < state.Players[j].UserID
	})
	return state
}
