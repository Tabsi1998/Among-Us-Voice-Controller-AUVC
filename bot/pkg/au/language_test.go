package au

import (
	"strings"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
)

// /au settings language picks what everybody in the server reads, and "Same as
// Discord" goes back to following the server language set in Discord.
func TestTheServerLanguageIsSavedAndShown(t *testing.T) {
	service, db := testService(t)

	for _, step := range []struct {
		choice, stored, shown string
	}{
		{"de", "de", "Deutsch"},
		{"en", "en", "English"},
		{LanguageFromDiscord, "", text.English.Say(text.LanguageSameAsDiscord)},
	} {
		set := request(GroupSettings, SettingsLanguage)
		set.Values.Strings[OptionLanguage] = step.choice
		reply, err := service.Handle(set)
		if err != nil {
			t.Fatalf("%s: %v", step.choice, err)
		}

		config, err := db.GuildConfig("guild")
		if err != nil {
			t.Fatalf("read back: %v", err)
		}
		if config.Language != step.stored {
			t.Errorf("%s stored %q, want %q", step.choice, config.Language, step.stored)
		}
		if !strings.Contains(reply, "Language: "+step.shown) {
			t.Errorf("%s: the settings do not show the language: %s", step.choice, reply)
		}
	}
}

// A language AUVC does not speak, from an import or an older build, would
// quietly come out in English.
func TestALanguageAUVCDoesNotSpeakIsRefused(t *testing.T) {
	config := configured()
	config.Language = "fr"

	if !mentions(Validate(config), OptionLanguage) {
		t.Errorf("expected a problem for %s, got %v", OptionLanguage, Validate(config))
	}

	config.Language = ""
	if mentions(Validate(config), OptionLanguage) {
		t.Errorf("following Discord is not a problem, got %v", Validate(config))
	}
}
