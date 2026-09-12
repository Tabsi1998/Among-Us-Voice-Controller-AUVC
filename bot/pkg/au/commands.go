// Package au defines the /au slash command tree, who may invoke each part of
// it, and what a valid guild configuration looks like.
//
// The command tree is data and the authorization and validation rules are pure
// functions, so all three can be tested without a Discord connection. Handlers
// live outside this package.
package au

import "github.com/bwmarrin/discordgo"

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
)

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

func choices(values ...string) []*discordgo.ApplicationCommandOptionChoice {
	list := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(values))
	for _, value := range values {
		list = append(list, &discordgo.ApplicationCommandOptionChoice{Name: value, Value: value})
	}
	return list
}

func sub(name, description string, options ...*discordgo.ApplicationCommandOption) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        name,
		Description: description,
		Options:     options,
	}
}

func group(name, description string, subcommands ...*discordgo.ApplicationCommandOption) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
		Name:        name,
		Description: description,
		Options:     subcommands,
	}
}

// voiceChannel builds a channel option restricted to guild voice channels, so
// Discord itself prevents someone from picking a text channel.
func voiceChannel(name, description string, required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:         discordgo.ApplicationCommandOptionChannel,
		Name:         name,
		Description:  description,
		Required:     required,
		ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildVoice},
	}
}

func textChannel(name, description string, required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:         discordgo.ApplicationCommandOptionChannel,
		Name:         name,
		Description:  description,
		Required:     required,
		ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText},
	}
}

func boolean(name, description string, required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionBoolean,
		Name:        name,
		Description: description,
		Required:    required,
	}
}

func role(name, description string, required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionRole,
		Name:        name,
		Description: description,
		Required:    required,
	}
}

func user(name, description string, required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionUser,
		Name:        name,
		Description: description,
		Required:    required,
	}
}

func text(name, description string, required bool, allowed ...string) *discordgo.ApplicationCommandOption {
	option := &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        name,
		Description: description,
		Required:    required,
	}
	if len(allowed) > 0 {
		option.Choices = choices(allowed...)
	}
	return option
}

// Command returns the complete /au tree.
//
// Options use the real Discord types: channel options are restricted to the
// channel kind they accept, roles are role options, flags are booleans and the
// timeout is an integer with server-side bounds. Nothing is flattened to a
// string, so Discord rejects most bad input before the bot ever sees it.
func Command() *discordgo.ApplicationCommand {
	minTimeout := float64(MinCaptureTimeoutSeconds)
	maxTimeout := float64(MaxCaptureTimeoutSeconds)

	timeout := &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionInteger,
		Name:        OptionTimeout,
		Description: "Seconds without capture before the fail-safe runs",
		Required:    false,
		MinValue:    &minTimeout,
		MaxValue:    maxTimeout,
	}

	return &discordgo.ApplicationCommand{
		Name:        Name,
		Description: "Among Us Voice Controller",
		Options: []*discordgo.ApplicationCommandOption{
			group(GroupSetup, "First-time setup",
				sub(SetupChannels, "Choose the voice and control channels",
					voiceChannel(OptionMainChannel, "Where living players belong", true),
					voiceChannel(OptionGhostChannel, "Where dead players are moved", true),
					textChannel(OptionControlChannel, "Where the bot posts game status", false),
				),
				sub(SetupPermissions, "Choose who may administer the bot",
					role(OptionAdminRole, "Role allowed to change settings", true),
				),
				sub(SetupReset, "Reset this guild to the default configuration",
					boolean(OptionConfirm, "Confirm that the current configuration is discarded", true),
				),
			),

			group(GroupSettings, "Inspect and change settings",
				sub(SettingsShow, "Show the current configuration"),
				sub(SettingsPreset, "Apply a named configuration preset",
					text(OptionPolicy, "Voice policy preset", true, VoicePolicies...),
				),
				sub(SettingsVoice, "Change voice handling",
					boolean(OptionEnabled, "Whether the bot manages voice at all", false),
					text(OptionPolicy, "Voice policy", false, VoicePolicies...),
				),
				sub(SettingsGhosts, "Change how dead players are handled",
					boolean(OptionAutoMoveGhosts, "Move dead players to the ghost channel", false),
					boolean(OptionEnforce, "Return players who switch channels themselves", false),
				),
				sub(SettingsSafety, "Change what happens when capture goes quiet",
					timeout,
					text(OptionTimeoutAction, "What to do when capture times out", false, CaptureTimeoutActions...),
					boolean(OptionAutoStart, "Start a session automatically when a game begins", false),
				),
				sub(SettingsExport, "Export the current configuration"),
			),

			group(GroupCapture, "Manage the capture connection",
				sub(CapturePair, "Create a one-time pairing code for the capture app"),
				sub(CaptureStatus, "Show the capture connection status"),
				sub(CaptureRevoke, "Revoke capture access for this guild",
					boolean(OptionConfirm, "Confirm that the capture app must pair again", true),
				),
			),

			group(GroupSession, "Control the running session",
				sub(SessionStart, "Start managing voice for the current game"),
				sub(SessionStop, "Stop managing voice and release everyone"),
				sub(SessionPause, "Temporarily stop applying voice changes"),
				sub(SessionResume, "Resume applying voice changes"),
				sub(SessionStatus, "Show the session status"),
			),

			sub(Link, "Link a Discord user to an Among Us player",
				text(OptionPlayer, "Among Us player name", true),
				user(OptionUser, "Discord user; defaults to you", false),
			),
			sub(Unlink, "Remove a link",
				user(OptionUser, "Discord user; defaults to you", false),
			),
			sub(Doctor, "Check configuration, permissions and connectivity"),
			sub(Version, "Show the AUVC version"),
		},
	}
}
