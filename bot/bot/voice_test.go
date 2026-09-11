package bot

import (
	"errors"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/voice"
)

// configStore serves one guild configuration, or one failure, to the /au
// service. Only EnsureGuildConfig is exercised here; the rest satisfies the
// interface.
type configStore struct {
	config sqlite.GuildConfig
	err    error
}

func (s configStore) EnsureGuildConfig(guildID string) (sqlite.GuildConfig, error) {
	if s.err != nil {
		return sqlite.GuildConfig{}, s.err
	}
	config := s.config
	config.GuildID = guildID
	return config, nil
}

func (s configStore) SaveGuildConfig(sqlite.GuildConfig) error { return nil }
func (s configStore) ReplaceLink(_, _, _ string) error         { return nil }
func (s configStore) DeleteLinksForUser(_, _ string) error     { return nil }
func (s configStore) SchemaVersion() (int, error)              { return 1, nil }

func configuredGuild() sqlite.GuildConfig {
	return sqlite.GuildConfig{
		Enabled:             true,
		MainVoiceChannelID:  "main",
		GhostVoiceChannelID: "ghost",
		AutoMoveGhosts:      true,
	}
}

func botWithStore(store au.Store) *Bot {
	return &Bot{
		AUVC:       au.NewService(store, "test", "test"),
		Reconciler: voice.NewReconciler(nil),
	}
}

func TestVoicePolicyTakesOverOnceTheGuildIsConfigured(t *testing.T) {
	config, ready := botWithStore(configStore{config: configuredGuild()}).voicePolicyConfig("g")

	if !ready {
		t.Fatal("a fully configured guild must be handled by the voice policy")
	}
	if want := (voice.Config{MainChannelID: "main", GhostChannelID: "ghost", AutoMoveGhosts: true}); config != want {
		t.Errorf("got %+v, want %+v", config, want)
	}
}

// A guild that has never run /au setup channels has no ghost channel to move
// anyone into. Taking over there would mean the bot does nothing at all, which
// looks like a broken bot rather than an unconfigured one.
func TestVoicePolicyLeavesAnUnconfiguredGuildToTheLegacyRules(t *testing.T) {
	cases := map[string]func(sqlite.GuildConfig) sqlite.GuildConfig{
		"disabled":              func(c sqlite.GuildConfig) sqlite.GuildConfig { c.Enabled = false; return c },
		"no main channel":       func(c sqlite.GuildConfig) sqlite.GuildConfig { c.MainVoiceChannelID = ""; return c },
		"no ghost channel":      func(c sqlite.GuildConfig) sqlite.GuildConfig { c.GhostVoiceChannelID = ""; return c },
		"nothing set up at all": func(sqlite.GuildConfig) sqlite.GuildConfig { return sqlite.GuildConfig{} },
	}

	for name, breakIt := range cases {
		t.Run(name, func(t *testing.T) {
			if _, ready := botWithStore(configStore{config: breakIt(configuredGuild())}).voicePolicyConfig("g"); ready {
				t.Error("the voice policy must not take over a guild that is not set up")
			}
		})
	}
}

// Voice is the visible half of a round. An unreachable configuration must fall
// back to the old behaviour rather than leave players muted with no way out.
func TestVoicePolicyFallsBackWhenTheConfigurationCannotBeRead(t *testing.T) {
	if _, ready := botWithStore(configStore{err: errors.New("database is gone")}).voicePolicyConfig("g"); ready {
		t.Error("an unreadable configuration must fall back to the legacy voice rules")
	}
}

// The bot is built with a reconciler, but a nil one would panic inside a voice
// handler and take the process down. Falling back is the safe reading.
func TestVoicePolicyFallsBackWithoutAReconciler(t *testing.T) {
	bot := &Bot{AUVC: au.NewService(configStore{config: configuredGuild()}, "test", "test")}

	if _, ready := bot.voicePolicyConfig("g"); ready {
		t.Error("without a reconciler the voice policy cannot apply anything")
	}
}

// AUVC is only wired up once the SQLite store opens. Until then the bot still
// has to run.
func TestVoicePolicyFallsBackWithoutTheService(t *testing.T) {
	bot := &Bot{Reconciler: voice.NewReconciler(nil)}

	if _, ready := bot.voicePolicyConfig("g"); ready {
		t.Error("without the /au service the voice policy has nothing to read")
	}
}
