package bot

import (
	"path/filepath"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
	"github.com/bwmarrin/discordgo"
)

// What everybody reads follows the server language set in Discord, until the
// server picks another with /au settings language.
func TestTheServerLanguageFollowsDiscordUntilTheServerPicksOne(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "amongus.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	state := discordgo.NewState()
	for id, locale := range map[string]string{"german": "de", "american": "en-US", "brazilian": "pt-BR"} {
		if err := state.GuildAdd(&discordgo.Guild{ID: id, PreferredLocale: locale}); err != nil {
			t.Fatalf("add guild %s: %v", id, err)
		}
	}
	controller := &Bot{AUVC: au.NewService(db, "test", "test"), PrimarySession: &discordgo.Session{State: state}}

	for guildID, want := range map[string]text.Language{
		"german":    text.German,
		"american":  text.English,
		"brazilian": text.English,
		// The bot has never seen this server.
		"unknown": text.English,
	} {
		if got := controller.guildLanguage(guildID); got != want {
			t.Errorf("%s: got %s, want %s", guildID, got, want)
		}
	}

	for guildID, picked := range map[string]text.Language{"german": text.English, "american": text.German} {
		config, err := db.EnsureGuildConfig(guildID)
		if err != nil {
			t.Fatalf("read %s: %v", guildID, err)
		}
		config.Language = string(picked)
		if err := db.SaveGuildConfig(config); err != nil {
			t.Fatalf("save %s: %v", guildID, err)
		}
		if got := controller.guildLanguage(guildID); got != picked {
			t.Errorf("%s picked %s, but the server language is %s", guildID, picked, got)
		}
	}
}
