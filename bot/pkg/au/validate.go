package au

import (
	"slices"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
)

// Problem is one reason a configuration cannot be saved. Field names the option
// the administrator would change, so a handler can point at it directly.
type Problem struct {
	Field string
	Key   text.Key
	Args  []any
}

// Describe says what is wrong, in a language, after the option it concerns.
func (p Problem) Describe(language text.Language) string {
	return p.Field + ": " + language.Say(p.Key, p.Args...)
}

func (p Problem) Error() string {
	return p.Describe(text.English)
}

// Validate checks a guild configuration before it is stored.
//
// The requirements call for validating configuration before saving rather than
// discovering the problem mid-round. Discord already enforces option types and
// the integer bounds; these are the rules it cannot express.
func Validate(config sqlite.GuildConfig) []Problem {
	var problems []Problem

	add := func(field string, key text.Key, args ...any) {
		problems = append(problems, Problem{Field: field, Key: key, Args: args})
	}

	if config.GuildID == "" {
		add("guild_id", text.ProblemMissing)
	}

	// A voice policy the bot does not implement would silently do nothing.
	if !slices.Contains(VoicePolicies, config.VoicePolicy) {
		add(OptionPolicy, text.ProblemOneOf, VoicePolicies, config.VoicePolicy)
	}

	if !slices.Contains(CaptureTimeoutActions, config.CaptureTimeoutAction) {
		add(OptionTimeoutAction, text.ProblemOneOf, CaptureTimeoutActions, config.CaptureTimeoutAction)
	}

	// Discord enforces these bounds on the option, but a configuration can also
	// arrive from an import or an older row.
	if config.CaptureTimeoutSeconds < MinCaptureTimeoutSeconds ||
		config.CaptureTimeoutSeconds > MaxCaptureTimeoutSeconds {
		add(OptionTimeout, text.ProblemTimeoutBounds,
			MinCaptureTimeoutSeconds, MaxCaptureTimeoutSeconds, config.CaptureTimeoutSeconds)
	}

	// One channel for both would make every move a no-op and every ghost audible
	// to the living, which is the one thing the bot exists to prevent.
	if config.MainVoiceChannelID != "" && config.MainVoiceChannelID == config.GhostVoiceChannelID {
		add(OptionGhostChannel, text.ProblemSameChannel)
	}

	// A language AUVC does not speak would quietly come out in English.
	if _, ok := text.Parse(config.Language); config.Language != "" && !ok {
		add(OptionLanguage, text.ProblemOneOf, text.Languages, config.Language)
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

	add := func(field string, key text.Key) {
		problems = append(problems, Problem{Field: field, Key: key})
	}

	if !config.Enabled {
		add(OptionEnabled, text.ProblemDisabled)
	}
	if config.MainVoiceChannelID == "" {
		add(OptionMainChannel, text.ProblemNotSet)
	}
	// A ghost channel is only needed while the dead are moved into it. Without
	// that everyone stays in the main channel and the dead stay muted (#161).
	if config.AutoMoveGhosts && config.GhostVoiceChannelID == "" {
		add(OptionGhostChannel, text.ProblemNotSet)
	}

	return problems
}

// Ready reports whether a guild is configured well enough to run a session.
func Ready(config sqlite.GuildConfig) bool {
	return len(NotReady(config)) == 0
}
