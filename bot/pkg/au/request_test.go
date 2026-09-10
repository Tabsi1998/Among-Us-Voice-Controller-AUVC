package au

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestParsePathAndValuesPreservesDiscordTypes(t *testing.T) {
	options := []*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name: GroupSettings,
			Type: discordgo.ApplicationCommandOptionSubCommandGroup,
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{
					Name: SettingsSafety,
					Type: discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandInteractionDataOption{
						{Name: OptionTimeout, Type: discordgo.ApplicationCommandOptionInteger, Value: float64(90)},
						{Name: OptionAutoStart, Type: discordgo.ApplicationCommandOptionBoolean, Value: true},
						{Name: OptionTimeoutAction, Type: discordgo.ApplicationCommandOptionString, Value: "pause"},
					},
				},
			},
		},
	}

	group, command, values, err := ParsePathAndValues(options)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if group != GroupSettings || command != SettingsSafety {
		t.Fatalf("path = %q/%q, want %q/%q", group, command, GroupSettings, SettingsSafety)
	}
	if timeout, ok := values.Integer(OptionTimeout); !ok || timeout != 90 {
		t.Errorf("timeout = %d, %t; want 90, true", timeout, ok)
	}
	if autoStart, ok := values.Bool(OptionAutoStart); !ok || !autoStart {
		t.Errorf("auto_start = %t, %t; want true, true", autoStart, ok)
	}
	if action, ok := values.String(OptionTimeoutAction); !ok || action != "pause" {
		t.Errorf("action = %q, %t; want pause, true", action, ok)
	}
}

func TestParsePathAndValuesReadsDiscordIDsAsStrings(t *testing.T) {
	options := []*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name: SetupChannels,
			Type: discordgo.ApplicationCommandOptionSubCommand,
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: OptionMainChannel, Type: discordgo.ApplicationCommandOptionChannel, Value: "main"},
			},
		},
	}

	_, _, values, err := ParsePathAndValues(options)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if channel, ok := values.String(OptionMainChannel); !ok || channel != "main" {
		t.Errorf("channel = %q, %t; want main, true", channel, ok)
	}
}

func TestParsePathAndValuesRejectsMalformedNesting(t *testing.T) {
	_, _, _, err := ParsePathAndValues([]*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name: GroupSettings,
			Type: discordgo.ApplicationCommandOptionSubCommandGroup,
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: SettingsShow, Type: discordgo.ApplicationCommandOptionSubCommand},
				{Name: SettingsExport, Type: discordgo.ApplicationCommandOptionSubCommand},
			},
		},
	})
	if err == nil {
		t.Fatal("expected a group containing two subcommands to be rejected")
	}
}

func TestParsePathAndValuesRejectsWrongRuntimeType(t *testing.T) {
	_, _, _, err := ParsePathAndValues([]*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name: SettingsVoice,
			Type: discordgo.ApplicationCommandOptionSubCommand,
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{Name: OptionEnabled, Type: discordgo.ApplicationCommandOptionBoolean, Value: "true"},
			},
		},
	})
	if err == nil {
		t.Fatal("expected a string in a boolean option to be rejected")
	}
}
