package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
)

// CurrentConfigVersion is the shape of GuildConfig this build writes. It is
// stored per row so a later migration can tell which defaults a guild was
// created with, independently of the schema version.
const CurrentConfigVersion = 1

// GuildConfig is the per-guild configuration required by docs/requirements.md.
type GuildConfig struct {
	GuildID string `json:"guild_id"`

	Enabled bool `json:"enabled"`

	MainVoiceChannelID   string `json:"main_voice_channel_id"`
	GhostVoiceChannelID  string `json:"ghost_voice_channel_id"`
	ControlTextChannelID string `json:"control_text_channel_id"`
	AdminRoleID          string `json:"admin_role_id"`

	VoicePolicy     string `json:"voice_policy"`
	AutoMoveGhosts  bool   `json:"auto_move_ghosts"`
	EnforceChannels bool   `json:"enforce_channels"`

	CaptureTimeoutSeconds int    `json:"capture_timeout_seconds"`
	CaptureTimeoutAction  string `json:"capture_timeout_action"`

	AutoStart bool `json:"auto_start"`

	// Language is what AUVC writes in for everybody in the server, such as
	// "de". Empty follows the server language set in Discord.
	Language string `json:"language"`

	ConfigVersion int `json:"config_version"`

	// CreatedAt and UpdatedAt are Unix seconds, maintained by the store.
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

// DefaultGuildConfig returns the configuration a guild starts with. The values
// mirror the defaults declared in the initial migration so that a row created
// in Go and one created by SQL defaults cannot drift apart.
func DefaultGuildConfig(guildID string) GuildConfig {
	return GuildConfig{
		GuildID:               guildID,
		Enabled:               true,
		VoicePolicy:           "ghost-chat",
		AutoMoveGhosts:        true,
		EnforceChannels:       true,
		CaptureTimeoutSeconds: 60,
		CaptureTimeoutAction:  "fail-open",
		AutoStart:             false,
		Language:              "",
		ConfigVersion:         CurrentConfigVersion,
	}
}

const guildColumns = `guild_id, enabled, main_voice_channel_id, ghost_voice_channel_id,
	control_text_channel_id, admin_role_id, voice_policy, auto_move_ghosts,
	enforce_channels, capture_timeout_seconds, capture_timeout_action, auto_start,
	language, config_version, created_at, updated_at`

// GuildConfig returns the stored configuration for a guild, or ErrNotFound.
func (d *DB) GuildConfig(guildID string) (GuildConfig, error) {
	row := d.db.QueryRow(
		"SELECT "+guildColumns+" FROM guild_config WHERE guild_id = ?", guildID)

	config, err := scanGuildConfig(row)
	if errors.Is(err, sql.ErrNoRows) {
		return GuildConfig{}, fmt.Errorf("guild %s: %w", guildID, ErrNotFound)
	}
	if err != nil {
		return GuildConfig{}, fmt.Errorf("read guild %s: %w", guildID, err)
	}
	return config, nil
}

// EnsureGuildConfig returns the stored configuration for a guild, creating it
// with the defaults if it does not exist yet. It never overwrites an existing
// row, so calling it on every guild join is safe.
func (d *DB) EnsureGuildConfig(guildID string) (GuildConfig, error) {
	config, err := d.GuildConfig(guildID)
	if err == nil {
		return config, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return GuildConfig{}, err
	}

	if err := d.SaveGuildConfig(DefaultGuildConfig(guildID)); err != nil {
		return GuildConfig{}, err
	}
	return d.GuildConfig(guildID)
}

// SaveGuildConfig writes a guild's configuration. created_at is preserved from
// the existing row; updated_at is always set to now, so callers cannot forge
// either timestamp.
func (d *DB) SaveGuildConfig(config GuildConfig) error {
	if config.GuildID == "" {
		return errors.New("save guild config: empty guild id")
	}
	if config.ConfigVersion == 0 {
		config.ConfigVersion = CurrentConfigVersion
	}

	_, err := d.db.Exec(`
		INSERT INTO guild_config (`+guildColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, unixepoch(), unixepoch())
		ON CONFLICT(guild_id) DO UPDATE SET
			enabled                 = excluded.enabled,
			main_voice_channel_id   = excluded.main_voice_channel_id,
			ghost_voice_channel_id  = excluded.ghost_voice_channel_id,
			control_text_channel_id = excluded.control_text_channel_id,
			admin_role_id           = excluded.admin_role_id,
			voice_policy            = excluded.voice_policy,
			auto_move_ghosts        = excluded.auto_move_ghosts,
			enforce_channels        = excluded.enforce_channels,
			capture_timeout_seconds = excluded.capture_timeout_seconds,
			capture_timeout_action  = excluded.capture_timeout_action,
			auto_start              = excluded.auto_start,
			language                = excluded.language,
			config_version          = excluded.config_version,
			updated_at              = unixepoch()`,
		config.GuildID, config.Enabled, config.MainVoiceChannelID, config.GhostVoiceChannelID,
		config.ControlTextChannelID, config.AdminRoleID, config.VoicePolicy, config.AutoMoveGhosts,
		config.EnforceChannels, config.CaptureTimeoutSeconds, config.CaptureTimeoutAction,
		config.AutoStart, config.Language, config.ConfigVersion)
	if err != nil {
		return fmt.Errorf("save guild %s: %w", config.GuildID, err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanGuildConfig(row scanner) (GuildConfig, error) {
	var c GuildConfig
	err := row.Scan(
		&c.GuildID, &c.Enabled, &c.MainVoiceChannelID, &c.GhostVoiceChannelID,
		&c.ControlTextChannelID, &c.AdminRoleID, &c.VoicePolicy, &c.AutoMoveGhosts,
		&c.EnforceChannels, &c.CaptureTimeoutSeconds, &c.CaptureTimeoutAction,
		&c.AutoStart, &c.Language, &c.ConfigVersion, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}
