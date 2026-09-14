package bot

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
	"github.com/bwmarrin/discordgo"
)

func TestSlashCommandHandlerRoutesAUAroundLegacyRedis(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "amongus.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	state := discordgo.NewState()
	if err := state.GuildAdd(&discordgo.Guild{ID: "guild", OwnerID: "owner"}); err != nil {
		t.Fatalf("add guild state: %v", err)
	}
	session := &discordgo.Session{State: state}
	controller := &Bot{AUVC: au.NewService(db, "v0.1.0-test", "abc123")}
	interaction := &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		Type:    discordgo.InteractionApplicationCommand,
		GuildID: "guild",
		Member:  &discordgo.Member{User: &discordgo.User{ID: "owner"}},
		Data: discordgo.ApplicationCommandInteractionData{
			Name: au.Name,
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: au.Version, Type: discordgo.ApplicationCommandOptionSubCommand},
			},
		},
	}}

	// The bot carries no game state, no session store and no reconciler here.
	// A panic would prove that /au reaches for something it should not need.
	response := controller.handleAUCommand(session, interaction)
	if response == nil || response.Data == nil {
		t.Fatal("expected an AUVC response")
	}
	if response.Data.Flags != discordgo.MessageFlagsEphemeral {
		t.Errorf("response flags = %d, want ephemeral", response.Data.Flags)
	}
	if !strings.Contains(response.Data.Content, "AUVC v0.1.0-test") {
		t.Errorf("unexpected response: %q", response.Data.Content)
	}
}

// Every /au reply is private, so it comes in the Discord language of whoever
// typed the command, and in English for a language AUVC does not speak.
func TestSlashCommandRepliesInTheLanguageOfWhoeverTypedIt(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "amongus.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	state := discordgo.NewState()
	if err := state.GuildAdd(&discordgo.Guild{ID: "guild", OwnerID: "owner"}); err != nil {
		t.Fatalf("add guild state: %v", err)
	}
	session := &discordgo.Session{State: state}
	// Without session control the command answers that it is unavailable, which
	// is a reply with no side effects to wait for.
	controller := &Bot{AUVC: au.NewService(db, "v0.1.0-test", "abc123")}

	for locale, language := range map[discordgo.Locale]text.Language{
		discordgo.German:    text.German,
		discordgo.EnglishUS: text.English,
		discordgo.French:    text.English,
	} {
		response := controller.handleAUCommand(session, &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
			Type:    discordgo.InteractionApplicationCommand,
			GuildID: "guild",
			Locale:  locale,
			Member:  &discordgo.Member{User: &discordgo.User{ID: "owner"}},
			Data: discordgo.ApplicationCommandInteractionData{
				Name: au.Name,
				Options: []*discordgo.ApplicationCommandInteractionDataOption{{
					Name: au.GroupSession,
					Type: discordgo.ApplicationCommandOptionSubCommandGroup,
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{Name: au.SessionStatus, Type: discordgo.ApplicationCommandOptionSubCommand},
					},
				}},
			},
		}})

		if want := language.Say(text.SessionUnavailable, au.SessionStatus); response.Data.Content != want {
			t.Errorf("%s: got %q, want %q", locale, response.Data.Content, want)
		}
	}
}
