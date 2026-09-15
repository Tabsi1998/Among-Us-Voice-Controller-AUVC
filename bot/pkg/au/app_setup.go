package au

import (
	"fmt"
	"strings"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
)

// LinkFromApp links a crewmate to a Discord member for the Windows app, or
// unlinks the crewmate when userID is empty.
//
// Whoever holds the local control secret started this bot with its token, so
// the app may link anybody, as an administrator may with /au link user:. It
// still goes through the same lock, and the same one link per member, as the
// command.
func (s *Service) LinkFromApp(guildID, player, userID string) error {
	player = strings.TrimSpace(player)
	if guildID == "" || player == "" {
		return fmt.Errorf("%w: a guild and a player are required", ErrInvalidInput)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if userID == "" {
		if err := s.store.DeleteLink(guildID, player); err != nil {
			return fmt.Errorf("remove player link: %w", err)
		}
		return nil
	}
	if err := s.store.ReplaceLink(guildID, player, userID); err != nil {
		return fmt.Errorf("save player link: %w", err)
	}
	return nil
}

// AppSetup is what the AUVC Windows app configures for a guild: the same
// channels /au setup channels takes, whether the dead move into the ghost
// channel, and whether a connecting capture starts a session on its own.
type AppSetup struct {
	MainVoiceChannelID   string
	GhostVoiceChannelID  string
	ControlTextChannelID string
	AutoStart            bool
	// AutoMoveGhosts is nil from an app that does not offer the choice, and then
	// what is stored stays. With it off no ghost channel is needed: everyone
	// stays in the main channel and the dead stay muted (#161).
	AutoMoveGhosts *bool
}

// ConfigureFromApp saves a setup chosen in the Windows app and returns the
// configuration as stored.
//
// Only the fields the app offers change. Everything else an administrator set
// in Discord, the admin role and the safety settings, is kept: the app
// finishing a setup must not quietly undo one.
//
// It goes through the same validation and the same lock as the commands, so
// the app cannot store anything /au setup channels would refuse.
func (s *Service) ConfigureFromApp(guildID string, setup AppSetup) (sqlite.GuildConfig, error) {
	if guildID == "" {
		return sqlite.GuildConfig{}, fmt.Errorf("%w: a guild is required", ErrInvalidInput)
	}
	if setup.MainVoiceChannelID == "" {
		return sqlite.GuildConfig{}, fmt.Errorf("%w: a main channel is required", ErrInvalidInput)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	config, err := s.store.EnsureGuildConfig(guildID)
	if err != nil {
		return sqlite.GuildConfig{}, fmt.Errorf("load guild configuration: %w", err)
	}

	if setup.AutoMoveGhosts != nil {
		config.AutoMoveGhosts = *setup.AutoMoveGhosts
	}
	if config.AutoMoveGhosts && setup.GhostVoiceChannelID == "" {
		return sqlite.GuildConfig{}, fmt.Errorf("%w: a ghost channel is required while the dead are moved into it", ErrInvalidInput)
	}

	config.MainVoiceChannelID = setup.MainVoiceChannelID
	config.GhostVoiceChannelID = setup.GhostVoiceChannelID
	config.ControlTextChannelID = setup.ControlTextChannelID
	config.AutoStart = setup.AutoStart

	// The app does not send its language yet (#112), so a refusal reads as it
	// always has.
	if err := s.save(config, text.English); err != nil {
		return sqlite.GuildConfig{}, err
	}

	// Read back rather than returning what was sent: the store sets updated_at
	// when it saves, so the values held here are already one timestamp behind.
	stored, err := s.store.EnsureGuildConfig(guildID)
	if err != nil {
		return sqlite.GuildConfig{}, fmt.Errorf("read back guild configuration: %w", err)
	}
	return stored, nil
}
