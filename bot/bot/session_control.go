package bot

import (
	"strings"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
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
func (s *SessionControl) Start(guildID string, language text.Language) (string, error) {
	if _, ready := s.bot.voicePolicyConfig(guildID); !ready {
		return language.Say(text.NotSetUp), nil
	}

	previous := s.bot.CaptureSessions.Mode(guildID)
	if err := s.bot.StartSession(guildID); err != nil {
		return "", err
	}

	if previous == Running {
		return language.Say(text.AlreadyManaging), nil
	}
	return language.Say(text.NowManaging), nil
}

// Stop stops managing voice and releases everyone.
func (s *SessionControl) Stop(guildID string, language text.Language) (string, error) {
	previous := s.bot.CaptureSessions.Mode(guildID)
	if err := s.bot.StopSession(guildID); err != nil {
		return "", err
	}

	if previous == Stopped {
		return language.Say(text.NotManaging), nil
	}
	return language.Say(text.StoppedManaging), nil
}

// Pause stops applying voice changes and leaves Discord exactly as it is.
func (s *SessionControl) Pause(guildID string, language text.Language) (string, error) {
	previous := s.bot.PauseSession(guildID)

	switch previous {
	case Paused:
		return language.Say(text.AlreadyPaused), nil
	case Stopped:
		// Saying "paused" would suggest something was interrupted, and resuming
		// later would then do something the administrator did not expect.
		return language.Say(text.NothingToPause), nil
	default:
		return language.Say(text.PausedNow), nil
	}
}

// Resume starts applying voice changes again.
func (s *SessionControl) Resume(guildID string, language text.Language) (string, error) {
	previous := s.bot.CaptureSessions.Mode(guildID)
	if previous == Running {
		return language.Say(text.AlreadyManaging), nil
	}

	if _, ready := s.bot.voicePolicyConfig(guildID); !ready {
		return language.Say(text.NotSetUp), nil
	}
	if err := s.bot.StartSession(guildID); err != nil {
		return "", err
	}

	// Resuming applies the round as it is now, not as it was when the pause
	// started, which is why the reply says so.
	return language.Say(text.Resumed), nil
}

// Status reports what the bot knows and what it is doing about it.
func (s *SessionControl) Status(guildID string, language text.Language) (string, error) {
	mode, phase, players := s.bot.CaptureSessions.Snapshot(guildID)

	var lines []string
	switch mode {
	case Running:
		lines = append(lines, language.Say(text.StatusManaging))
	case Paused:
		lines = append(lines, language.Say(text.StatusPaused))
	default:
		lines = append(lines, language.Say(text.StatusStopped))
	}

	if len(players) == 0 {
		lines = append(lines, language.Say(text.StatusNoGame))
		return strings.Join(lines, "\n"), nil
	}

	lines = append(lines, language.Say(text.StatusPhase, describePhase(phase, language)))

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
	lines = append(lines, language.Say(text.StatusPlayers, alive, dead))

	if _, ready := s.bot.voicePolicyConfig(guildID); !ready {
		lines = append(lines, language.Say(text.StatusNoChannels))
	}

	return strings.Join(lines, "\n"), nil
}

// describePhase names a phase for a person.
//
// game.PhaseNames has no entry for the states that mean "no round is running",
// and an empty string in a status message reads like a bug.
func describePhase(phase game.Phase, language text.Language) string {
	switch phase {
	case game.MENU:
		return language.Say(text.PhaseMenu)
	case game.LOBBY:
		return language.Say(text.PhaseLobby)
	case game.TASKS:
		return language.Say(text.PhaseTasks)
	case game.DISCUSS:
		return language.Say(text.PhaseDiscussion)
	case game.GAMEOVER:
		return language.Say(text.PhaseBetweenRounds)
	default:
		return language.Say(text.PhaseUnknown)
	}
}

// describeMode names a session mode for a person.
func describeMode(mode Mode, language text.Language) string {
	switch mode {
	case Running:
		return language.Say(text.ModeRunning)
	case Paused:
		return language.Say(text.ModePaused)
	default:
		return language.Say(text.ModeStopped)
	}
}
