package bot

import (
	"fmt"
	"strings"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/amongus"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
)

// SessionControl answers the /au session commands.
//
// It returns finished Discord replies rather than values for the /au service to
// render, because what to say about a session is Discord knowledge and this is
// where that lives. The service stays a router.
type SessionControl struct {
	bot *Bot
}

// NewSessionControl wires the session commands to a bot.
func NewSessionControl(bot *Bot) *SessionControl {
	return &SessionControl{bot: bot}
}

// Start begins applying the voice policy.
func (s *SessionControl) Start(guildID string) (string, error) {
	if _, ready := s.bot.voicePolicyConfig(guildID); !ready {
		return "❌ AUVC is not set up for this server yet. Run `/au setup channels` first.", nil
	}

	previous := s.bot.CaptureSessions.Mode(guildID)
	if err := s.bot.StartSession(guildID); err != nil {
		return "", err
	}

	if previous == Running {
		return "✅ Already managing voice. Nothing changed.", nil
	}
	return "✅ Now managing voice for this server.", nil
}

// Stop stops managing voice and releases everyone.
func (s *SessionControl) Stop(guildID string) (string, error) {
	previous := s.bot.CaptureSessions.Mode(guildID)
	if err := s.bot.StopSession(guildID); err != nil {
		return "", err
	}

	if previous == Stopped {
		return "✅ Not managing voice. Nothing changed.", nil
	}
	return "✅ Stopped managing voice. Everyone has been unmuted and returned to the main channel.", nil
}

// Pause stops applying voice changes and leaves Discord exactly as it is.
func (s *SessionControl) Pause(guildID string) (string, error) {
	previous := s.bot.PauseSession(guildID)

	switch previous {
	case Paused:
		return "✅ Already paused. Nothing changed.", nil
	case Stopped:
		// Saying "paused" would suggest something was interrupted, and resuming
		// later would then do something the administrator did not expect.
		return "⚠️ Nothing was being managed, so there was nothing to pause. " +
			"Run `/au session start` to begin.", nil
	default:
		return "⏸️ Paused. Players stay exactly where they are until " +
			"`/au session resume`, and the game is still being followed.", nil
	}
}

// Resume starts applying voice changes again.
func (s *SessionControl) Resume(guildID string) (string, error) {
	previous := s.bot.CaptureSessions.Mode(guildID)
	if previous == Running {
		return "✅ Already managing voice. Nothing changed.", nil
	}

	if _, ready := s.bot.voicePolicyConfig(guildID); !ready {
		return "❌ AUVC is not set up for this server yet. Run `/au setup channels` first.", nil
	}
	if err := s.bot.StartSession(guildID); err != nil {
		return "", err
	}

	// Resuming applies the round as it is now, not as it was when the pause
	// started, which is why the reply says so.
	return "▶️ Resumed. Voice has been brought into line with the game as it stands.", nil
}

// Status reports what the bot knows and what it is doing about it.
func (s *SessionControl) Status(guildID string) (string, error) {
	mode, phase, players := s.bot.CaptureSessions.Snapshot(guildID)

	var lines []string
	switch mode {
	case Running:
		lines = append(lines, "✅ Managing voice.")
	case Paused:
		lines = append(lines, "⏸️ Paused. The game is being followed, but Discord is left alone.")
	default:
		lines = append(lines, "⏹️ Not managing voice. Run `/au session start` to begin.")
	}

	if len(players) == 0 {
		lines = append(lines, "No game data yet. Connect the capture app with `/au capture pair`.")
		return strings.Join(lines, "\n"), nil
	}

	lines = append(lines, fmt.Sprintf("Phase: **%s**", describePhase(phase)))

	alive, dead := 0, 0
	for _, player := range players {
		if player.Disconnected {
			continue
		}
		if player.Alive {
			alive++
		} else {
			dead++
		}
	}
	lines = append(lines, fmt.Sprintf("Players: %d alive, %d dead.", alive, dead))

	if _, ready := s.bot.voicePolicyConfig(guildID); !ready {
		lines = append(lines, "⚠️ Voice channels are not configured. Run `/au setup channels`.")
	}

	return strings.Join(lines, "\n"), nil
}

// describePhase names a phase for a person.
//
// game.PhaseNames has no entry for the states that mean "no round is running",
// and an empty string in a status message reads like a bug.
func describePhase(phase game.Phase) string {
	switch phase {
	case game.MENU:
		return "Menu"
	case game.GAMEOVER:
		return "Between rounds"
	case game.UNINITIALIZED:
		return "Unknown"
	default:
		if name := amongus.ToLocale(phase); name != nil && name.Other != "" {
			return name.Other
		}
		return string(game.PhaseNames[phase])
	}
}
