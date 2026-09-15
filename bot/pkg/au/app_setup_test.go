package au

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
)

func appSetupService(t *testing.T) (*Service, *sqlite.DB) {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "auvc.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewService(db, "test", "test"), db
}

func TestTheAppSavesChannelsAndAutoStart(t *testing.T) {
	service, _ := appSetupService(t)

	saved, err := service.ConfigureFromApp("guild-1", AppSetup{
		MainVoiceChannelID:   "main",
		GhostVoiceChannelID:  "ghost",
		ControlTextChannelID: "control",
		AutoStart:            true,
	})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}

	stored, err := service.GuildConfig("guild-1")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if stored.MainVoiceChannelID != "main" || stored.GhostVoiceChannelID != "ghost" ||
		stored.ControlTextChannelID != "control" || !stored.AutoStart {
		t.Errorf("stored %+v", stored)
	}
	if saved != stored {
		t.Errorf("the returned configuration differs from the stored one:\n%+v\n%+v", saved, stored)
	}
}

// Finishing a setup in the app must not quietly undo what an administrator set
// in Discord.
func TestTheAppKeepsWhatItDoesNotOffer(t *testing.T) {
	service, db := appSetupService(t)

	config, err := db.EnsureGuildConfig("guild-1")
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	config.AdminRoleID = "role-admins"
	config.EnforceChannels = false
	config.CaptureTimeoutSeconds = 120
	if err := db.SaveGuildConfig(config); err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, err := service.ConfigureFromApp("guild-1", AppSetup{MainVoiceChannelID: "main", GhostVoiceChannelID: "ghost"}); err != nil {
		t.Fatalf("configure: %v", err)
	}

	stored, err := service.GuildConfig("guild-1")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if stored.AdminRoleID != "role-admins" || stored.EnforceChannels || stored.CaptureTimeoutSeconds != 120 {
		t.Errorf("the app changed settings it does not offer: %+v", stored)
	}
}

// The app gets the same validation as /au setup channels.
func TestTheAppCannotStoreWhatTheCommandWouldRefuse(t *testing.T) {
	service, _ := appSetupService(t)

	for name, setup := range map[string]AppSetup{
		"no main channel":  {GhostVoiceChannelID: "ghost"},
		"no ghost channel": {MainVoiceChannelID: "main"},
		"the same channel": {MainVoiceChannelID: "voice", GhostVoiceChannelID: "voice"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := service.ConfigureFromApp("guild-1", setup); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("got %v, want %v", err, ErrInvalidInput)
			}
		})
	}

	if _, err := service.ConfigureFromApp("", AppSetup{MainVoiceChannelID: "main", GhostVoiceChannelID: "ghost"}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("a setup without a guild gave %v, want %v", err, ErrInvalidInput)
	}
}

// With the dead staying muted in the main channel no ghost channel is needed,
// and the guild can still run a session (#161).
func TestTheAppCanKeepTheDeadInTheMainChannel(t *testing.T) {
	service, _ := appSetupService(t)
	stay := false

	if _, err := service.ConfigureFromApp("guild-1", AppSetup{MainVoiceChannelID: "main", AutoMoveGhosts: &stay}); err != nil {
		t.Fatalf("configure: %v", err)
	}

	stored, err := service.GuildConfig("guild-1")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if stored.AutoMoveGhosts || stored.GhostVoiceChannelID != "" || stored.MainVoiceChannelID != "main" {
		t.Errorf("stored %+v", stored)
	}
	if _, ready, err := service.VoiceConfig("guild-1"); err != nil || !ready {
		t.Errorf("a guild whose dead stay in the main channel must be ready, got ready=%v err=%v", ready, err)
	}
}

// Choosing the ghost channel again needs one.
func TestMovingTheDeadNeedsAGhostChannel(t *testing.T) {
	service, _ := appSetupService(t)
	move := true

	if _, err := service.ConfigureFromApp("guild-1", AppSetup{MainVoiceChannelID: "main", AutoMoveGhosts: &move}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want %v", err, ErrInvalidInput)
	}
}

// An app that does not offer the choice keeps what an administrator chose in
// Discord.
func TestAnAppWithoutTheChoiceKeepsIt(t *testing.T) {
	service, db := appSetupService(t)

	config, err := db.EnsureGuildConfig("guild-1")
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	config.AutoMoveGhosts = false
	if err := db.SaveGuildConfig(config); err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, err := service.ConfigureFromApp("guild-1", AppSetup{MainVoiceChannelID: "main"}); err != nil {
		t.Fatalf("configure: %v", err)
	}

	stored, err := service.GuildConfig("guild-1")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if stored.AutoMoveGhosts {
		t.Error("an app without the choice switched moving the dead back on")
	}
}
