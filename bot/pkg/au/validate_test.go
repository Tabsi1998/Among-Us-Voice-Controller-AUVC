package au

import (
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
)

func configured() sqlite.GuildConfig {
	config := sqlite.DefaultGuildConfig("guild")
	config.MainVoiceChannelID = "main"
	config.GhostVoiceChannelID = "ghost"
	return config
}

// The default a guild is created with must be storable, or EnsureGuildConfig
// would write a row that validation then rejects.
func TestDefaultConfigurationIsValid(t *testing.T) {
	if problems := Validate(sqlite.DefaultGuildConfig("guild")); len(problems) != 0 {
		t.Errorf("the default configuration must validate, got %v", problems)
	}
}

func TestValidateRejectsUnknownEnumValues(t *testing.T) {
	config := configured()
	config.VoicePolicy = "free-for-all"
	config.CaptureTimeoutAction = "explode"

	problems := Validate(config)
	if !mentions(problems, OptionPolicy) {
		t.Errorf("expected a problem for %s, got %v", OptionPolicy, problems)
	}
	if !mentions(problems, OptionTimeoutAction) {
		t.Errorf("expected a problem for %s, got %v", OptionTimeoutAction, problems)
	}
}

// Discord enforces the bounds on the option itself, but a configuration can
// also arrive from an import or an older row.
func TestValidateEnforcesTimeoutBounds(t *testing.T) {
	for _, seconds := range []int{0, MinCaptureTimeoutSeconds - 1, MaxCaptureTimeoutSeconds + 1} {
		config := configured()
		config.CaptureTimeoutSeconds = seconds

		if !mentions(Validate(config), OptionTimeout) {
			t.Errorf("timeout %d should have been rejected", seconds)
		}
	}

	for _, seconds := range []int{MinCaptureTimeoutSeconds, 60, MaxCaptureTimeoutSeconds} {
		config := configured()
		config.CaptureTimeoutSeconds = seconds

		if mentions(Validate(config), OptionTimeout) {
			t.Errorf("timeout %d should have been accepted", seconds)
		}
	}
}

// One channel for both would make every move a no-op and let the living hear
// the dead, which is precisely what the bot exists to prevent.
func TestValidateRejectsTheSameChannelTwice(t *testing.T) {
	config := configured()
	config.GhostVoiceChannelID = config.MainVoiceChannelID

	if !mentions(Validate(config), OptionGhostChannel) {
		t.Error("using one channel for both roles must be rejected")
	}
}

func TestValidateRejectsAnEmptyGuildID(t *testing.T) {
	config := configured()
	config.GuildID = ""

	if Valid(config) {
		t.Error("a configuration without a guild id must not validate")
	}
}

// Readiness is a state, not a validation failure: a freshly added bot has a
// perfectly storable configuration that is simply not set up yet.
func TestNotReadyReportsMissingSetupWithoutFailingValidation(t *testing.T) {
	fresh := sqlite.DefaultGuildConfig("guild")

	if !Valid(fresh) {
		t.Fatal("the fresh configuration should still validate")
	}
	if Ready(fresh) {
		t.Fatal("a guild without channels is not ready")
	}

	problems := NotReady(fresh)
	if !mentions(problems, OptionMainChannel) || !mentions(problems, OptionGhostChannel) {
		t.Errorf("expected both channels to be reported, got %v", problems)
	}
}

func TestReadyOnceChannelsAreSet(t *testing.T) {
	if !Ready(configured()) {
		t.Errorf("a configured guild should be ready, got %v", NotReady(configured()))
	}
}

func TestNotReadyReportsADisabledGuild(t *testing.T) {
	config := configured()
	config.Enabled = false

	if !mentions(NotReady(config), OptionEnabled) {
		t.Error("a disabled guild must be reported as not ready")
	}
}

func mentions(problems []Problem, field string) bool {
	for _, problem := range problems {
		if problem.Field == field {
			return true
		}
	}
	return false
}
