// Package voice turns a game session into the voice state each player should
// be in. It is a pure mapping: no Discord calls, no storage, no clock.
//
// Game event handlers never mutate Discord voice state directly. They update
// the session, ask this package what the result should look like, and hand that
// to the reconciler, which applies only the differences. Keeping the decision
// separate from the application is what makes the ghost-chat table in
// docs/requirements.md testable in full.
package voice

import (
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/session"
)

// DesiredVoiceState is where a player should be and how they should be muted.
type DesiredVoiceState struct {
	// TargetChannelID is the voice channel the player belongs in. Empty means
	// the policy has no opinion and the player must be left where they are.
	TargetChannelID string
	// Muted is the server mute the player should carry.
	Muted bool
	// Deafened is the server deafen the player should carry.
	Deafened bool
}

// Open is the state every managed player returns to between rounds: audible,
// able to hear, in the main channel.
func Open(mainChannelID string) DesiredVoiceState {
	return DesiredVoiceState{TargetChannelID: mainChannelID}
}

// Config is the part of a guild's configuration the policy reads. It is a plain
// struct rather than the stored configuration so this package stays independent
// of the storage layer.
type Config struct {
	MainChannelID  string
	GhostChannelID string
	// AutoMoveGhosts moves dead players into the ghost channel so they can talk
	// to each other. With it off, or with no ghost channel configured, dead
	// players stay in the main channel and are silenced instead.
	AutoMoveGhosts bool
	// EnforceChannels returns players who switch channels themselves. It is on
	// by default. With it off the bot stops deciding where a living player
	// sits during a round and only manages what they can say and hear.
	//
	// It does not switch off ghost moves: moving the dead into the ghost
	// channel is the feature AutoMoveGhosts governs, not a correction of a
	// player who wandered off.
	EnforceChannels bool
}

// usesGhostChannel reports whether dead players can actually be moved.
//
// A ghost channel that was never configured is treated exactly like the feature
// being switched off. Producing a move to an empty channel id would make the
// reconciler either fail per player or, worse, move somebody nowhere.
func (c Config) usesGhostChannel() bool {
	return c.AutoMoveGhosts && c.GhostChannelID != ""
}

// enforcedMain is the main channel when the guild lets the bot decide where
// players sit during a round, and an empty string when it does not.
//
// An empty target is how the policy says "leave this player where they are":
// the reconciler never sends one, because Discord reads an empty channel id
// as a disconnect.
func (c Config) enforcedMain() string {
	if !c.EnforceChannels {
		return ""
	}
	return c.MainChannelID
}

// Desired returns the voice state every managed player should be in, keyed by
// Discord user id.
//
// Only managed players appear: unlinked users and bot accounts are absent from
// the result entirely, so nothing downstream can act on them by accident. That
// is stricter than filtering later, because a player missing from the map is
// impossible to move by mistake.
//
// The mapping follows the ghost-chat table in docs/requirements.md:
//
//	Lobby       living and dead   main, open
//	Tasks       living            main, muted and deafened
//	            dead              ghost, open
//	Discussion  living            main, open
//	            dead              ghost, open
//	Ended       living and dead   main, open
//
// Discussion covers meetings and voting, which the game state reports as one
// phase. Menu means no round is running and is handled like Ended.
//
// The channel half of that table is what Config.EnforceChannels governs. With
// enforcement off a living player keeps the mute and deafen their phase calls
// for but is left in whatever channel they chose, so an admin can hand that
// decision back to the players without giving up the round.
func Desired(state session.State, config Config) map[string]DesiredVoiceState {
	desired := make(map[string]DesiredVoiceState)

	for _, player := range state.Managed() {
		desired[player.UserID] = desiredFor(state.Phase, player.Alive, config)
	}
	return desired
}

func desiredFor(phase game.Phase, alive bool, config Config) DesiredVoiceState {
	switch phase {
	case game.TASKS:
		if alive {
			// Living players cannot talk and cannot hear anything while the
			// round is running.
			return DesiredVoiceState{
				TargetChannelID: config.enforcedMain(),
				Muted:           true,
				Deafened:        true,
			}
		}
		if config.usesGhostChannel() {
			// Ghosts talk among themselves. The living are deafened, so they
			// hear none of it.
			return Open(config.GhostChannelID)
		}
		// Without a ghost channel the dead stay put. They are silenced rather
		// than left audible, because the living stop being deafened the moment
		// the phase changes and a stray voice would leak the round.
		return DesiredVoiceState{TargetChannelID: config.enforcedMain(), Muted: true}

	case game.DISCUSS:
		if alive {
			return DesiredVoiceState{TargetChannelID: config.enforcedMain()}
		}
		if config.usesGhostChannel() {
			// Ghosts keep talking during meetings and voting.
			return Open(config.GhostChannelID)
		}
		// The living can hear now, so a dead player in the main channel would
		// reveal the round. Silence them.
		return DesiredVoiceState{TargetChannelID: config.enforcedMain(), Muted: true}

	default:
		// Lobby, Menu and Ended: everyone back in the main channel, audible.
		// Dead players are released here even when they were in the ghost
		// channel, which is what ends a round cleanly.
		//
		// This one ignores EnforceChannels on purpose. Ghosts have to come back
		// out of the ghost channel however the guild is configured, or a round
		// ends with half the players stranded somewhere the bot put them.
		return Open(config.MainChannelID)
	}
}
