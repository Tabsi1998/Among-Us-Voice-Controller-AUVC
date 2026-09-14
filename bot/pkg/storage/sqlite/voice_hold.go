package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
)

// VoiceHold is what AUVC set on one member and has not lifted yet.
type VoiceHold struct {
	GuildID  string
	UserID   string
	Muted    bool
	Deafened bool
	// GhostChannelID is the ghost channel AUVC moved the member into, or empty.
	GhostChannelID string
}

// Empty reports whether the hold records nothing left to lift.
func (h VoiceHold) Empty() bool {
	return !h.Muted && !h.Deafened && h.GhostChannelID == ""
}

// VoiceHold returns what AUVC holds on a member. A member without a record gets
// an empty hold, not an error: most members never had anything set.
func (d *DB) VoiceHold(guildID, userID string) (VoiceHold, error) {
	hold := VoiceHold{GuildID: guildID, UserID: userID}
	err := d.db.QueryRow(
		"SELECT muted, deafened, ghost_channel_id FROM voice_hold WHERE guild_id = ? AND user_id = ?",
		guildID, userID,
	).Scan(&hold.Muted, &hold.Deafened, &hold.GhostChannelID)
	if errors.Is(err, sql.ErrNoRows) {
		return hold, nil
	}
	if err != nil {
		return VoiceHold{}, fmt.Errorf("read voice hold %s/%s: %w", guildID, userID, err)
	}
	return hold, nil
}

// SaveVoiceHold records a hold, or removes the record when nothing is held any
// more.
func (d *DB) SaveVoiceHold(hold VoiceHold) error {
	if hold.GuildID == "" || hold.UserID == "" {
		return errors.New("save voice hold: guild and user id are required")
	}

	if hold.Empty() {
		if _, err := d.db.Exec("DELETE FROM voice_hold WHERE guild_id = ? AND user_id = ?",
			hold.GuildID, hold.UserID); err != nil {
			return fmt.Errorf("remove voice hold %s/%s: %w", hold.GuildID, hold.UserID, err)
		}
		return nil
	}

	_, err := d.db.Exec(`
		INSERT INTO voice_hold (guild_id, user_id, muted, deafened, ghost_channel_id, updated_at)
		VALUES (?, ?, ?, ?, ?, unixepoch())
		ON CONFLICT(guild_id, user_id) DO UPDATE SET
			muted            = excluded.muted,
			deafened         = excluded.deafened,
			ghost_channel_id = excluded.ghost_channel_id,
			updated_at       = unixepoch()`,
		hold.GuildID, hold.UserID, hold.Muted, hold.Deafened, hold.GhostChannelID)
	if err != nil {
		return fmt.Errorf("save voice hold %s/%s: %w", hold.GuildID, hold.UserID, err)
	}
	return nil
}

// VoiceHolds returns every hold on record, ordered by guild and member.
func (d *DB) VoiceHolds() ([]VoiceHold, error) {
	rows, err := d.db.Query(
		"SELECT guild_id, user_id, muted, deafened, ghost_channel_id FROM voice_hold ORDER BY guild_id, user_id")
	if err != nil {
		return nil, fmt.Errorf("read voice holds: %w", err)
	}
	defer rows.Close()

	var holds []VoiceHold
	for rows.Next() {
		var hold VoiceHold
		if err := rows.Scan(&hold.GuildID, &hold.UserID, &hold.Muted, &hold.Deafened, &hold.GhostChannelID); err != nil {
			return nil, fmt.Errorf("scan voice hold: %w", err)
		}
		holds = append(holds, hold)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read voice holds: %w", err)
	}
	return holds, nil
}
