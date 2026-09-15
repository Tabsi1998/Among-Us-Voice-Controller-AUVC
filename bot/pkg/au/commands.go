// Package au defines the /au slash command tree, who may invoke each part of
// it, and what a valid guild configuration looks like.
//
// The command tree is data and the authorization and validation rules are pure
// functions, so all three can be tested without a Discord connection. Handlers
// live outside this package.
package au

import (
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
	"github.com/bwmarrin/discordgo"
)

// Name is the single top-level command. Everything else is a subcommand or a
// subcommand inside a group, which is as deep as Discord allows.
const Name = "au"

// Subcommand group and subcommand names, so handlers and tests refer to the
// same strings the registration uses.
const (
	GroupSetup    = "setup"
	GroupSettings = "settings"
	GroupCapture  = "capture"
	GroupSession  = "session"

	SetupChannels    = "channels"
	SetupPermissions = "permissions"
	SetupReset       = "reset"

	SettingsShow   = "show"
	SettingsPreset = "preset"
	SettingsVoice  = "voice"
	SettingsGhosts = "ghosts"
	SettingsSafety = "safety"
	SettingsExport = "export"

	SettingsLanguage = "language"

	CapturePair   = "pair"
	CaptureStatus = "status"
	CaptureRevoke = "revoke"

	SessionStart  = "start"
	SessionStop   = "stop"
	SessionPause  = "pause"
	SessionResume = "resume"
	SessionStatus = "status"

	Link    = "link"
	Unlink  = "unlink"
	Doctor  = "doctor"
	Version = "version"
)

// Option names used by more than one subcommand.
const (
	OptionMainChannel    = "main_channel"
	OptionGhostChannel   = "ghost_channel"
	OptionControlChannel = "control_channel"
	OptionAdminRole      = "admin_role"
	OptionEnabled        = "enabled"
	OptionPolicy         = "policy"
	OptionAutoMoveGhosts = "auto_move_ghosts"
	OptionEnforce        = "enforce_channels"
	OptionTimeout        = "capture_timeout_seconds"
	OptionTimeoutAction  = "capture_timeout_action"
	OptionAutoStart      = "auto_start"
	OptionPlayer         = "player"
	OptionUser           = "user"
	OptionConfirm        = "confirm"
	OptionLanguage       = "language"
)

// LanguageFromDiscord is the choice for following the server language set in
// Discord. It is stored as an empty language.
const LanguageFromDiscord = "discord"

// Capture timeout bounds. Below the minimum a brief network hiccup would trip
// the fail-safe; above the maximum a dead capture would leave players muted for
// minutes before anything happens.
const (
	MinCaptureTimeoutSeconds = 10
	MaxCaptureTimeoutSeconds = 600
)

// VoicePolicies are the accepted values for voice_policy.
var VoicePolicies = []string{"ghost-chat"}

// What to do when capture stops responding.
const (
	// CaptureTimeoutFailOpen releases every managed player: unmuted,
	// undeafened and back in the main channel. It is the default because
	// nothing in Discord expires a server mute, so a round left muted stays
	// muted until somebody notices.
	CaptureTimeoutFailOpen = "fail-open"
	// CaptureTimeoutPause suspends the session and leaves players exactly as
	// they are.
	CaptureTimeoutPause = "pause"

	// DefaultCaptureTimeoutSeconds is what a guild gets when its
	// configuration cannot be read. A fail-safe that switches itself off
	// when a database blinks is not one.
	DefaultCaptureTimeoutSeconds = 60
)

// CaptureTimeoutActions are the accepted values for capture_timeout_action.
var CaptureTimeoutActions = []string{CaptureTimeoutFailOpen, CaptureTimeoutPause}

// DiscordLocales are the Discord locales AUVC describes its commands in besides
// English, which Discord shows everybody else.
var DiscordLocales = map[discordgo.Locale]text.Language{
	discordgo.German: text.German,
}

// describe returns a description in English and in every other language AUVC
// speaks, for Discord to show each member in their own.
func describe(key text.Key) (string, map[discordgo.Locale]string) {
	translations := make(map[discordgo.Locale]string, len(DiscordLocales))
	for locale, language := range DiscordLocales {
		translations[locale] = language.Say(key)
	}
	return text.English.Say(key), translations
}

func choices(values ...string) []*discordgo.ApplicationCommandOptionChoice {
	list := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(values))
	for _, value := range values {
		list = append(list, &discordgo.ApplicationCommandOptionChoice{Name: value, Value: value})
	}
	return list
}

func option(kind discordgo.ApplicationCommandOptionType, name string, key text.Key) *discordgo.ApplicationCommandOption {
	description, translations := describe(key)
	return &discordgo.ApplicationCommandOption{
		Type:                     kind,
		Name:                     name,
		Description:              description,
		DescriptionLocalizations: translations,
	}
}

func sub(name string, key text.Key, options ...*discordgo.ApplicationCommandOption) *discordgo.ApplicationCommandOption {
	command := option(discordgo.ApplicationCommandOptionSubCommand, name, key)
	command.Options = options
	return command
}

func group(name string, key text.Key, subcommands ...*discordgo.ApplicationCommandOption) *discordgo.ApplicationCommandOption {
	command := option(discordgo.ApplicationCommandOptionSubCommandGroup, name, key)
	command.Options = subcommands
	return command
}

// voiceChannel builds a channel option restricted to guild voice channels, so
// Discord itself prevents someone from picking a text channel.
func voiceChannel(name string, key text.Key, required bool) *discordgo.ApplicationCommandOption {
	channel := option(discordgo.ApplicationCommandOptionChannel, name, key)
	channel.Required = required
	channel.ChannelTypes = []discordgo.ChannelType{discordgo.ChannelTypeGuildVoice}
	return channel
}

func textChannel(name string, key text.Key, required bool) *discordgo.ApplicationCommandOption {
	channel := option(discordgo.ApplicationCommandOptionChannel, name, key)
	channel.Required = required
	channel.ChannelTypes = []discordgo.ChannelType{discordgo.ChannelTypeGuildText}
	return channel
}

func boolean(name string, key text.Key, required bool) *discordgo.ApplicationCommandOption {
	flag := option(discordgo.ApplicationCommandOptionBoolean, name, key)
	flag.Required = required
	return flag
}

func role(name string, key text.Key, required bool) *discordgo.ApplicationCommandOption {
	picked := option(discordgo.ApplicationCommandOptionRole, name, key)
	picked.Required = required
	return picked
}

func user(name string, key text.Key, required bool) *discordgo.ApplicationCommandOption {
	picked := option(discordgo.ApplicationCommandOptionUser, name, key)
	picked.Required = required
	return picked
}

func freeText(name string, key text.Key, required bool, allowed ...string) *discordgo.ApplicationCommandOption {
	value := option(discordgo.ApplicationCommandOptionString, name, key)
	value.Required = required
	if len(allowed) > 0 {
		value.Choices = choices(allowed...)
	}
	return value
}

// languageOption offers following Discord and every language AUVC speaks. Each
// language is named in itself, so it can be found from any other.
func languageOption() *discordgo.ApplicationCommandOption {
	choice := option(discordgo.ApplicationCommandOptionString, OptionLanguage, text.DescribeLanguage)
	choice.Required = true

	sameAsDiscord, translations := describe(text.LanguageSameAsDiscord)
	choice.Choices = []*discordgo.ApplicationCommandOptionChoice{
		{Name: sameAsDiscord, NameLocalizations: translations, Value: LanguageFromDiscord},
	}
	for _, language := range text.Languages {
		choice.Choices = append(choice.Choices, &discordgo.ApplicationCommandOptionChoice{
			Name: language.Name(), Value: string(language),
		})
	}
	return choice
}

// Command returns the complete /au tree.
//
// Options use the real Discord types: channel options are restricted to the
// channel kind they accept, roles are role options, flags are booleans and the
// timeout is an integer with server-side bounds. Nothing is flattened to a
// string, so Discord rejects most bad input before the bot ever sees it.
//
// Every description comes in each language AUVC speaks. Discord picks the one
// matching the member's own Discord language and falls back to English.
func Command() *discordgo.ApplicationCommand {
	minTimeout := float64(MinCaptureTimeoutSeconds)
	maxTimeout := float64(MaxCaptureTimeoutSeconds)

	timeout := option(discordgo.ApplicationCommandOptionInteger, OptionTimeout, text.DescribeTimeout)
	timeout.MinValue = &minTimeout
	timeout.MaxValue = maxTimeout

	description, translations := describe(text.DescribeAU)
	return &discordgo.ApplicationCommand{
		Name:                     Name,
		Description:              description,
		DescriptionLocalizations: &translations,
		Options: []*discordgo.ApplicationCommandOption{
			group(GroupSetup, text.DescribeSetup,
				sub(SetupChannels, text.DescribeSetupChannels,
					voiceChannel(OptionMainChannel, text.DescribeMainChannel, true),
					voiceChannel(OptionGhostChannel, text.DescribeGhostChannel, true),
					textChannel(OptionControlChannel, text.DescribeControlChannel, false),
				),
				sub(SetupPermissions, text.DescribeSetupPermissions,
					role(OptionAdminRole, text.DescribeAdminRole, true),
				),
				sub(SetupReset, text.DescribeSetupReset,
					boolean(OptionConfirm, text.DescribeConfirmReset, true),
				),
			),

			group(GroupSettings, text.DescribeSettings,
				sub(SettingsShow, text.DescribeSettingsShow),
				sub(SettingsPreset, text.DescribeSettingsPreset,
					freeText(OptionPolicy, text.DescribePolicyPreset, true, VoicePolicies...),
				),
				sub(SettingsVoice, text.DescribeSettingsVoice,
					boolean(OptionEnabled, text.DescribeEnabled, false),
					freeText(OptionPolicy, text.DescribePolicy, false, VoicePolicies...),
				),
				sub(SettingsGhosts, text.DescribeSettingsGhosts,
					boolean(OptionAutoMoveGhosts, text.DescribeAutoMoveGhosts, false),
					boolean(OptionEnforce, text.DescribeEnforce, false),
				),
				sub(SettingsSafety, text.DescribeSettingsSafety,
					timeout,
					freeText(OptionTimeoutAction, text.DescribeTimeoutAction, false, CaptureTimeoutActions...),
					boolean(OptionAutoStart, text.DescribeAutoStart, false),
				),
				sub(SettingsLanguage, text.DescribeSettingsLanguage,
					languageOption(),
				),
				sub(SettingsExport, text.DescribeSettingsExport),
			),

			group(GroupCapture, text.DescribeCapture,
				sub(CapturePair, text.DescribeCapturePair),
				sub(CaptureStatus, text.DescribeCaptureStatus),
				sub(CaptureRevoke, text.DescribeCaptureRevoke,
					boolean(OptionConfirm, text.DescribeConfirmRevoke, true),
				),
			),

			group(GroupSession, text.DescribeSession,
				sub(SessionStart, text.DescribeSessionStart),
				sub(SessionStop, text.DescribeSessionStop),
				sub(SessionPause, text.DescribeSessionPause),
				sub(SessionResume, text.DescribeSessionResume),
				sub(SessionStatus, text.DescribeSessionStatus),
			),

			sub(Link, text.DescribeLink,
				freeText(OptionPlayer, text.DescribePlayer, false),
				user(OptionUser, text.DescribeUser, false),
			),
			sub(Unlink, text.DescribeUnlink,
				user(OptionUser, text.DescribeUser, false),
			),
			sub(Doctor, text.DescribeDoctor),
			sub(Version, text.DescribeVersion),
		},
	}
}
