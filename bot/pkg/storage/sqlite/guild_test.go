package sqlite

import (
	"path/filepath"
	"testing"
)

// The Go defaults and the SQL column defaults describe the same thing in two
// places. If they ever drift, a guild created by one path behaves differently
// from a guild created by the other, which is close to impossible to debug.
func TestGoDefaultsMatchTheSQLColumnDefaults(t *testing.T) {
	db, _ := openTemp(t)

	// Insert supplying only the columns that have no default.
	if _, err := db.db.Exec(
		"INSERT INTO guild_config (guild_id, created_at, updated_at) VALUES (?, unixepoch(), unixepoch())",
		"from-sql",
	); err != nil {
		t.Fatalf("insert with SQL defaults: %v", err)
	}

	fromSQL, err := db.GuildConfig("from-sql")
	if err != nil {
		t.Fatalf("read SQL-defaulted guild: %v", err)
	}

	fromGo := DefaultGuildConfig("from-sql")
	fromGo.CreatedAt, fromGo.UpdatedAt = fromSQL.CreatedAt, fromSQL.UpdatedAt

	if fromSQL != fromGo {
		t.Errorf("defaults drifted:\n  SQL: %+v\n  Go:  %+v", fromSQL, fromGo)
	}
}

// The requirements set enforce_channels to true by default; getting this wrong
// silently disables channel enforcement for every new guild.
func TestEnforceChannelsDefaultsToTrue(t *testing.T) {
	db, _ := openTemp(t)

	config, err := db.EnsureGuildConfig("guild")
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if !config.EnforceChannels {
		t.Error("enforce_channels must default to true")
	}
	if config.VoicePolicy != "ghost-chat" {
		t.Errorf("voice policy = %q, want ghost-chat", config.VoicePolicy)
	}
	if config.CaptureTimeoutAction != "fail-open" {
		t.Errorf("capture timeout action = %q, want fail-open", config.CaptureTimeoutAction)
	}
}

// EnsureGuildConfig runs on every guild join, so it must never reset settings
// an administrator already made.
func TestEnsureGuildConfigDoesNotOverwrite(t *testing.T) {
	db, _ := openTemp(t)

	config, err := db.EnsureGuildConfig("guild")
	if err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	config.MainVoiceChannelID = "main-channel"
	config.EnforceChannels = false
	if err := db.SaveGuildConfig(config); err != nil {
		t.Fatalf("save: %v", err)
	}

	again, err := db.EnsureGuildConfig("guild")
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if again.MainVoiceChannelID != "main-channel" || again.EnforceChannels {
		t.Errorf("ensure overwrote existing configuration: %+v", again)
	}
}

func TestSaveGuildConfigRoundTrip(t *testing.T) {
	db, _ := openTemp(t)

	want := DefaultGuildConfig("guild")
	want.Enabled = false
	want.MainVoiceChannelID = "main"
	want.GhostVoiceChannelID = "ghost"
	want.ControlTextChannelID = "control"
	want.AdminRoleID = "admins"
	want.AutoMoveGhosts = false
	want.EnforceChannels = false
	want.CaptureTimeoutSeconds = 120
	want.CaptureTimeoutAction = "pause"
	want.AutoStart = true

	if err := db.SaveGuildConfig(want); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := db.GuildConfig("guild")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want.CreatedAt, want.UpdatedAt = got.CreatedAt, got.UpdatedAt
	if got != want {
		t.Errorf("round trip lost data:\n  got:  %+v\n  want: %+v", got, want)
	}
}

// created_at records when a guild was first configured. An update must not
// reset it, or the field means nothing.
func TestSavePreservesCreatedAt(t *testing.T) {
	db, _ := openTemp(t)

	first, err := db.EnsureGuildConfig("guild")
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}

	first.AdminRoleID = "admins"
	if err := db.SaveGuildConfig(first); err != nil {
		t.Fatalf("save: %v", err)
	}

	second, err := db.GuildConfig("guild")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if second.CreatedAt != first.CreatedAt {
		t.Errorf("created_at changed on update: %d -> %d", first.CreatedAt, second.CreatedAt)
	}
}

func TestSaveGuildConfigRejectsAnEmptyGuildID(t *testing.T) {
	db, _ := openTemp(t)

	if err := db.SaveGuildConfig(GuildConfig{}); err == nil {
		t.Error("expected saving a config without a guild id to fail")
	}
}

// The acceptance criterion is that configuration survives a bot or container
// restart, so this closes the database and opens it again from disk rather
// than reusing the handle.
func TestConfigurationSurvivesARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "amongus.db")

	first, err := Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	config := DefaultGuildConfig("guild")
	config.MainVoiceChannelID = "main"
	config.GhostVoiceChannelID = "ghost"
	config.CaptureTimeoutSeconds = 90
	if err := first.SaveGuildConfig(config); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := first.SaveLink("guild", "Red", "user-1"); err != nil {
		t.Fatalf("save link: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer second.Close()

	restored, err := second.GuildConfig("guild")
	if err != nil {
		t.Fatalf("read after restart: %v", err)
	}
	if restored.MainVoiceChannelID != "main" || restored.GhostVoiceChannelID != "ghost" {
		t.Errorf("channels lost across restart: %+v", restored)
	}
	if restored.CaptureTimeoutSeconds != 90 {
		t.Errorf("capture timeout lost across restart: %d", restored.CaptureTimeoutSeconds)
	}

	links, err := second.Links("guild")
	if err != nil {
		t.Fatalf("read links after restart: %v", err)
	}
	if len(links) != 1 || links[0].InGameName != "Red" || links[0].DiscordUserID != "user-1" {
		t.Errorf("links lost across restart: %+v", links)
	}
}
