package au

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
)

func testService(t *testing.T) (*Service, *sqlite.DB) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "amongus.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewService(db, "v0.1.0-test", "abc123"), db
}

func request(group, command string) Request {
	return Request{
		GuildID: "guild",
		Group:   group,
		Command: command,
		Invoker: Invoker{UserID: "owner", IsGuildOwner: true},
		Values: Values{
			Strings:  map[string]string{},
			Booleans: map[string]bool{},
			Integers: map[string]int64{},
		},
	}
}

func TestSetupAndTypedSettingsPersist(t *testing.T) {
	service, db := testService(t)

	setup := request(GroupSetup, SetupChannels)
	setup.Values.Strings[OptionMainChannel] = "main"
	setup.Values.Strings[OptionGhostChannel] = "ghost"
	setup.Values.Strings[OptionControlChannel] = "control"
	if _, err := service.Handle(setup); err != nil {
		t.Fatalf("setup channels: %v", err)
	}

	safety := request(GroupSettings, SettingsSafety)
	safety.Values.Integers[OptionTimeout] = 90
	safety.Values.Strings[OptionTimeoutAction] = "pause"
	safety.Values.Booleans[OptionAutoStart] = true
	if _, err := service.Handle(safety); err != nil {
		t.Fatalf("save safety settings: %v", err)
	}

	ghosts := request(GroupSettings, SettingsGhosts)
	ghosts.Values.Booleans[OptionAutoMoveGhosts] = false
	ghosts.Values.Booleans[OptionEnforce] = false
	if _, err := service.Handle(ghosts); err != nil {
		t.Fatalf("save ghost settings: %v", err)
	}

	stored, err := db.GuildConfig("guild")
	if err != nil {
		t.Fatalf("read configuration: %v", err)
	}
	if stored.MainVoiceChannelID != "main" || stored.GhostVoiceChannelID != "ghost" || stored.ControlTextChannelID != "control" {
		t.Errorf("channels were not persisted: %+v", stored)
	}
	if stored.CaptureTimeoutSeconds != 90 || stored.CaptureTimeoutAction != "pause" || !stored.AutoStart {
		t.Errorf("safety settings were not persisted: %+v", stored)
	}
	if stored.AutoMoveGhosts || stored.EnforceChannels {
		t.Errorf("explicit false values were lost: %+v", stored)
	}
}

func TestAdminCommandsRequireConfiguredAuthority(t *testing.T) {
	service, db := testService(t)
	config := sqlite.DefaultGuildConfig("guild")
	config.AdminRoleID = "auvc-admin"
	if err := db.SaveGuildConfig(config); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	ordinary := request(GroupSettings, SettingsShow)
	ordinary.Invoker = Invoker{UserID: "member", RoleIDs: []string{"other"}}
	if _, err := service.Handle(ordinary); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("ordinary member error = %v, want ErrUnauthorized", err)
	}

	withRole := ordinary
	withRole.Invoker.RoleIDs = []string{"auvc-admin"}
	if _, err := service.Handle(withRole); err != nil {
		t.Fatalf("configured admin role was rejected: %v", err)
	}
}

func TestLinkReplacementIsOneToOnePerUser(t *testing.T) {
	service, db := testService(t)

	first := request("", Link)
	first.Invoker = Invoker{UserID: "user"}
	first.Values.Strings[OptionPlayer] = "Red"
	if _, err := service.Handle(first); err != nil {
		t.Fatalf("first link: %v", err)
	}

	second := first
	second.Values = Values{Strings: map[string]string{OptionPlayer: "Blue"}, Booleans: map[string]bool{}, Integers: map[string]int64{}}
	if _, err := service.Handle(second); err != nil {
		t.Fatalf("replacement link: %v", err)
	}

	links, err := db.Links("guild")
	if err != nil {
		t.Fatalf("read links: %v", err)
	}
	if len(links) != 1 || links[0].InGameName != "Blue" || links[0].DiscordUserID != "user" {
		t.Errorf("replacement left stale links: %+v", links)
	}
}

func TestLinkingAnotherUserRequiresAdmin(t *testing.T) {
	service, db := testService(t)
	config := sqlite.DefaultGuildConfig("guild")
	config.AdminRoleID = "auvc-admin"
	if err := db.SaveGuildConfig(config); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	link := request("", Link)
	link.Invoker = Invoker{UserID: "member"}
	link.Values.Strings[OptionPlayer] = "Red"
	link.Values.Strings[OptionUser] = "somebody-else"
	if _, err := service.Handle(link); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("linking another user error = %v, want ErrUnauthorized", err)
	}
}

func TestResetRequiresExplicitTrueAndKeepsLinks(t *testing.T) {
	service, db := testService(t)
	if err := db.ReplaceLink("guild", "Red", "user"); err != nil {
		t.Fatalf("seed link: %v", err)
	}

	reset := request(GroupSetup, SetupReset)
	reset.Values.Booleans[OptionConfirm] = false
	if _, err := service.Handle(reset); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("false confirmation error = %v, want ErrInvalidInput", err)
	}

	reset.Values.Booleans[OptionConfirm] = true
	if _, err := service.Handle(reset); err != nil {
		t.Fatalf("confirmed reset: %v", err)
	}
	links, err := db.Links("guild")
	if err != nil || len(links) != 1 {
		t.Fatalf("reset should keep persistent links, links=%+v err=%v", links, err)
	}
}

func TestSettingsExportUsesStableJSONFieldNames(t *testing.T) {
	service, _ := testService(t)
	export := request(GroupSettings, SettingsExport)
	content, err := service.Handle(export)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	content = strings.TrimSuffix(strings.TrimPrefix(content, "```json\n"), "\n```")
	var document map[string]any
	if err := json.Unmarshal([]byte(content), &document); err != nil {
		t.Fatalf("export is not JSON: %v", err)
	}
	for _, field := range []string{"guild_id", "voice_policy", "capture_timeout_seconds", "config_version"} {
		if _, ok := document[field]; !ok {
			t.Errorf("export is missing %q: %s", field, content)
		}
	}
}

func TestDoctorReportsDatabaseAndMissingSetup(t *testing.T) {
	service, _ := testService(t)
	doctor := request("", Doctor)
	content, err := service.Handle(doctor)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	for _, expected := range []string{"✅ SQLite reachable", "⚠️ main_channel", "⚠️ ghost_channel"} {
		if !strings.Contains(content, expected) {
			t.Errorf("doctor response missing %q: %s", expected, content)
		}
	}
}

func TestVersionDoesNotClaimAProductionBuildWhenMetadataIsEmpty(t *testing.T) {
	_, db := testService(t)
	service := NewService(db, "", "")
	content, err := service.Handle(request("", Version))
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if content != "AUVC development (`unknown`)" {
		t.Errorf("version response = %q", content)
	}
}
