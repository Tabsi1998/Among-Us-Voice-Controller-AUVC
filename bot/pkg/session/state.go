// Package session holds the authoritative, Discord-free view of a running
// Among Us session: the phase, which Discord users are linked to which player,
// and whether those players are alive.
//
// Nothing here imports discordgo, Redis or the storage layer. The voice policy
// added in phase 8 consumes this projection and produces a DesiredVoiceState;
// keeping the projection free of transport and API types is what allows that
// policy to be a pure function with exhaustive tests.
package session

import "github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"

// PlayerState is one managed Discord user and, when linked, the Among Us player
// they control.
type PlayerState struct {
	// UserID is the Discord user snowflake.
	UserID string
	// InGameName is the Among Us player name this user is linked to, or an
	// empty string when the user is not linked to any player.
	InGameName string
	// Alive reflects the linked player. It is meaningless for unlinked users
	// and is reported as true for them so that no caller can accidentally treat
	// an unlinked user as a corpse.
	Alive bool
	// Bot marks Discord bot accounts. The voice policy must never manage them,
	// and neither must channel enforcement.
	Bot bool
}

// Linked reports whether this user controls an Among Us player.
func (p PlayerState) Linked() bool {
	return p.InGameName != ""
}

// Managed reports whether the voice policy may act on this user at all.
// Unlinked users and bots are left untouched.
func (p PlayerState) Managed() bool {
	return p.Linked() && !p.Bot
}

// State is the session projection the voice policy consumes.
type State struct {
	// Phase is the current Among Us phase.
	Phase game.Phase
	// Players is the set of Discord users the bot knows about in this session,
	// linked or not. Order is not significant.
	Players []PlayerState
}

// Managed returns the players the voice policy may act on: linked, non-bot.
func (s State) Managed() []PlayerState {
	managed := make([]PlayerState, 0, len(s.Players))
	for _, player := range s.Players {
		if player.Managed() {
			managed = append(managed, player)
		}
	}
	return managed
}

// Player returns the state of one Discord user.
func (s State) Player(userID string) (PlayerState, bool) {
	for _, player := range s.Players {
		if player.UserID == userID {
			return player, true
		}
	}
	return PlayerState{}, false
}

// CountManaged returns how many players the policy may act on.
func (s State) CountManaged() int {
	count := 0
	for _, player := range s.Players {
		if player.Managed() {
			count++
		}
	}
	return count
}

// CountAlive returns how many managed players are still alive.
func (s State) CountAlive() int {
	count := 0
	for _, player := range s.Players {
		if player.Managed() && player.Alive {
			count++
		}
	}
	return count
}

// CountDead returns how many managed players are dead.
func (s State) CountDead() int {
	return s.CountManaged() - s.CountAlive()
}
