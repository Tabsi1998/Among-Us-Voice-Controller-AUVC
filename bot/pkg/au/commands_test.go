package au

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

// contract is the command list from docs/requirements.md, verbatim.
var contract = []string{
	"/au setup channels",
	"/au setup permissions",
	"/au setup reset",
	"/au settings show",
	"/au settings preset",
	"/au settings voice",
	"/au settings ghosts",
	"/au settings safety",
	"/au settings export",
	"/au settings language",
	"/au capture pair",
	"/au capture status",
	"/au capture revoke",
	"/au session start",
	"/au session stop",
	"/au session pause",
	"/au session resume",
	"/au session status",
	"/au link",
	"/au unlink",
	"/au doctor",
	"/au version",
}

// flatten walks the tree and returns every invocable command path.
func flatten(command *discordgo.ApplicationCommand) []string {
	var paths []string
	for _, option := range command.Options {
		switch option.Type {
		case discordgo.ApplicationCommandOptionSubCommand:
			paths = append(paths, "/"+command.Name+" "+option.Name)
		case discordgo.ApplicationCommandOptionSubCommandGroup:
			for _, leaf := range option.Options {
				paths = append(paths, "/"+command.Name+" "+option.Name+" "+leaf.Name)
			}
		}
	}
	return paths
}

// The contract lists the commands AUVC must offer. Drifting from it silently is
// how a documented command quietly stops existing.
func TestTreeMatchesTheContract(t *testing.T) {
	got := flatten(Command())

	for _, want := range contract {
		if !slices.Contains(got, want) {
			t.Errorf("missing command from the contract: %s", want)
		}
	}
	for _, have := range got {
		if !slices.Contains(contract, have) {
			t.Errorf("command not in the contract: %s", have)
		}
	}
}

var namePattern = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

// Discord rejects a registration that breaks any of these, which surfaces as a
// panic at startup rather than a test failure. Checking here is cheaper.
func TestTreeSatisfiesDiscordConstraints(t *testing.T) {
	command := Command()

	// Discord allows command -> group -> subcommand -> leaf options, and nothing
	// deeper. kind says what this level is allowed to contain.
	const (
		topLevel     = iota // groups or subcommands
		inGroup             // subcommands only
		inSubcommand        // leaf options only
	)

	var walk func(path string, options []*discordgo.ApplicationCommandOption, kind int)
	walk = func(path string, options []*discordgo.ApplicationCommandOption, kind int) {
		if len(options) > 25 {
			t.Errorf("%s has %d options, Discord allows 25", path, len(options))
		}

		seenOptional := false
		for _, option := range options {
			here := path + " " + option.Name

			if !namePattern.MatchString(option.Name) {
				t.Errorf("%s: name %q is not a valid Discord option name", here, option.Name)
			}
			if option.Description == "" || len(option.Description) > 100 {
				t.Errorf("%s: description must be 1..100 characters, got %d",
					here, len(option.Description))
			}
			checkTranslations(t, here, option.DescriptionLocalizations)

			isGroup := option.Type == discordgo.ApplicationCommandOptionSubCommandGroup
			isSub := option.Type == discordgo.ApplicationCommandOptionSubCommand
			isContainer := isGroup || isSub

			switch kind {
			case topLevel:
				// groups and subcommands both allowed
			case inGroup:
				if !isSub {
					t.Errorf("%s: a subcommand group may only contain subcommands", here)
				}
			case inSubcommand:
				if isContainer {
					t.Errorf("%s: a subcommand may not contain another subcommand", here)
				}
			}

			if !isContainer {
				// Discord requires every required option before the optional
				// ones; violating it fails the whole registration.
				if option.Required && seenOptional {
					t.Errorf("%s: required option follows an optional one", here)
				}
				if !option.Required {
					seenOptional = true
				}
			}

			if isGroup {
				walk(here, option.Options, inGroup)
			} else if isSub {
				walk(here, option.Options, inSubcommand)
			}
		}
	}

	if !namePattern.MatchString(command.Name) {
		t.Errorf("command name %q is not valid", command.Name)
	}
	if command.DescriptionLocalizations == nil {
		t.Errorf("/%s has no translated description", command.Name)
	} else {
		checkTranslations(t, "/"+command.Name, *command.DescriptionLocalizations)
	}
	walk("/"+command.Name, command.Options, topLevel)
}

// checkTranslations holds a translated description to Discord's limit, counted
// in characters as Discord counts it, and makes sure every language AUVC
// describes its commands in is there. A description over the limit fails the
// whole registration, so /au would be missing in every server.
func checkTranslations(t *testing.T, path string, translations map[discordgo.Locale]string) {
	t.Helper()

	for locale := range DiscordLocales {
		described := translations[locale]
		if length := utf8.RuneCountInString(described); described == "" || length > 100 {
			t.Errorf("%s: the %s description must be 1..100 characters, got %d", path, locale, length)
		}
	}
	for locale := range translations {
		if _, ok := DiscordLocales[locale]; !ok {
			t.Errorf("%s: a description for %s, which AUVC does not speak", path, locale)
		}
	}
}

// "Use actual Discord channel, role, boolean and integer option types. Do not
// flatten everything to strings." A string option is only acceptable where the
// value is free text or comes from a fixed choice list.
func TestOptionsUseRealTypesRatherThanStrings(t *testing.T) {
	freeText := map[string]bool{OptionPlayer: true}

	forEachOption(t, Command(), func(path string, option *discordgo.ApplicationCommandOption) {
		if option.Type != discordgo.ApplicationCommandOptionString {
			return
		}
		if freeText[option.Name] {
			return
		}
		if len(option.Choices) == 0 {
			t.Errorf("%s: string option %q has neither choices nor a reason to be free text",
				path, option.Name)
		}
	})
}

// A channel option that accepts any channel lets an administrator pick a text
// channel as the ghost voice channel, which only fails much later.
func TestChannelOptionsAreRestrictedToAChannelType(t *testing.T) {
	forEachOption(t, Command(), func(path string, option *discordgo.ApplicationCommandOption) {
		if option.Type != discordgo.ApplicationCommandOptionChannel {
			return
		}
		if len(option.ChannelTypes) == 0 {
			t.Errorf("%s: channel option %q accepts any channel type", path, option.Name)
		}
	})
}

func TestTimeoutOptionCarriesItsBounds(t *testing.T) {
	var found bool

	forEachOption(t, Command(), func(path string, option *discordgo.ApplicationCommandOption) {
		if option.Name != OptionTimeout {
			return
		}
		found = true

		if option.Type != discordgo.ApplicationCommandOptionInteger {
			t.Errorf("%s: %q must be an integer option", path, OptionTimeout)
		}
		if option.MinValue == nil || *option.MinValue != float64(MinCaptureTimeoutSeconds) {
			t.Errorf("%s: %q is missing its minimum", path, OptionTimeout)
		}
		if option.MaxValue != float64(MaxCaptureTimeoutSeconds) {
			t.Errorf("%s: %q is missing its maximum", path, OptionTimeout)
		}
	})

	if !found {
		t.Errorf("no %q option in the tree", OptionTimeout)
	}
}

// Every group and subcommand name is referenced by handlers through the
// exported constants; a typo there would only show up at runtime.
func TestExportedNamesAppearInTheTree(t *testing.T) {
	paths := strings.Join(flatten(Command()), "\n")

	for _, name := range []string{
		GroupSetup, GroupSettings, GroupCapture, GroupSession,
		SetupChannels, SetupPermissions, SetupReset,
		SettingsShow, SettingsPreset, SettingsVoice, SettingsGhosts, SettingsSafety, SettingsExport, SettingsLanguage,
		CapturePair, CaptureStatus, CaptureRevoke,
		SessionStart, SessionStop, SessionPause, SessionResume, SessionStatus,
		Link, Unlink, Doctor, Version,
	} {
		if !strings.Contains(paths, " "+name) {
			t.Errorf("constant %q does not appear in the command tree", name)
		}
	}
}

func forEachOption(t *testing.T, command *discordgo.ApplicationCommand,
	visit func(path string, option *discordgo.ApplicationCommandOption)) {
	t.Helper()

	var walk func(path string, options []*discordgo.ApplicationCommandOption)
	walk = func(path string, options []*discordgo.ApplicationCommandOption) {
		for _, option := range options {
			here := path + " " + option.Name
			switch option.Type {
			case discordgo.ApplicationCommandOptionSubCommand,
				discordgo.ApplicationCommandOptionSubCommandGroup:
				walk(here, option.Options)
			default:
				visit(path, option)
			}
		}
	}
	walk("/"+command.Name, command.Options)
}
