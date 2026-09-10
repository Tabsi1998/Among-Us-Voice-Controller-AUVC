package au

import (
	"fmt"
	"slices"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
)

// Problem is one reason a configuration cannot be saved. Field names the option
// the administrator would change, so a handler can point at it directly.
type Problem struct {
	Field   string
	Message string
}

func (p Problem) Error() string {
	return p.Field + ": " + p.Message
}

// Validate checks a guild configuration before it is stored.
//
// The requirements call for validating configuration before saving rather than
// discovering the problem mid-round. Discord already enforces option types and
// the integer bounds; these are the rules it cannot express.
func Validate(config sqlite.GuildConfig) []Problem {
	var problems []Problem

	add := func(field, message string) {
		problems = append(problems, Problem{Field: field, Message: message})
	}

	if config.GuildID == "" {
		add("guild_id", "missing")
	}

	// A voice policy the bot does not implement would silently do nothing.
	if !slices.Contains(VoicePolicies, config.VoicePolicy) {
		add(OptionPolicy, fmt.Sprintf("must be one of %v, got %q", VoicePolicies, config.VoicePolicy))
	}

	if !slices.Contains(CaptureTimeoutActions, config.CaptureTimeoutAction) {
		add(OptionTimeoutAction, fmt.Sprintf("must be one of %v, got %q",
			CaptureTimeoutActions, config.CaptureTimeoutAction))
	}

	// Discord enforces these bounds on the option, but a configuration can also
	// arrive from an import or an older row.
	if config.CaptureTimeoutSeconds < MinCaptureTimeoutSeconds ||
		config.CaptureTimeoutSeconds > MaxCaptureTimeoutSeconds {
		add(OptionTimeout, fmt.Sprintf("must be between %d and %d seconds, got %d",
			MinCaptureTimeoutSeconds, MaxCaptureTimeoutSeconds, config.CaptureTimeoutSeconds))
	}

	// One channel for both would make every move a no-op and every ghost audible
	// to the living, which is the one thing the bot exists to prevent.
	if config.MainVoiceChannelID != "" && config.MainVoiceChannelID == config.GhostVoiceChannelID {
		add(OptionGhostChannel, "must differ from the main voice channel")
	}

	return problems
}

// Valid reports whether a configuration passes Validate.
func Valid(config sqlite.GuildConfig) bool {
	return len(Validate(config)) == 0
}

// NotReady returns why a guild cannot run a session yet, or nothing when it can.
//
// This is deliberately separate from Validate. A freshly added bot has a valid
// configuration that is simply not set up yet: rejecting it as invalid would
// mean the default row could never be written. Whether a guild is operational
// is a state, reported by /au doctor and checked before a session starts, not a
// reason to refuse a save.
func NotReady(config sqlite.GuildConfig) []Problem {
	var problems []Problem

	add := func(field, message string) {
		problems = append(problems, Problem{Field: field, Message: message})
	}

	if !config.Enabled {
		add(OptionEnabled, "the bot is disabled for this guild")
	}
	if config.MainVoiceChannelID == "" {
		add(OptionMainChannel, "not set; run /au setup channels")
	}
	if config.GhostVoiceChannelID == "" {
		add(OptionGhostChannel, "not set; run /au setup channels")
	}

	return problems
}

// Ready reports whether a guild is configured well enough to run a session.
func Ready(config sqlite.GuildConfig) bool {
	return len(NotReady(config)) == 0
}
