package sqlite

import (
	"errors"
	"fmt"
)

// PlayerLink ties one Among Us player name to one Discord user inside a guild.
// Restart recovery reads these back so a round survives a bot restart without
// everyone having to link again.
type PlayerLink struct {
	InGameName    string
	DiscordUserID string
	UpdatedAt     int64
}

// ReplaceLink atomically keeps one persistent player link per Discord user.
// The player-name primary key already guarantees the inverse direction. Doing
// both statements in one transaction avoids losing the user's previous link if
// the replacement insert fails.
func (d *DB) ReplaceLink(guildID, inGameName, discordUserID string) error {
	if guildID == "" || inGameName == "" || discordUserID == "" {
		return errors.New("replace link: guild, player name and user id are all required")
	}

	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin replacing link %s/%s: %w", guildID, discordUserID, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		"DELETE FROM player_link WHERE guild_id = ? AND discord_user_id = ?", guildID, discordUserID,
	); err != nil {
		return fmt.Errorf("remove previous link %s/%s: %w", guildID, discordUserID, err)
	}
	if _, err := tx.Exec(`
		INSERT INTO player_link (guild_id, in_game_name, discord_user_id, updated_at)
		VALUES (?, ?, ?, unixepoch())
		ON CONFLICT(guild_id, in_game_name) DO UPDATE SET
			discord_user_id = excluded.discord_user_id,
			updated_at      = unixepoch()`,
		guildID, inGameName, discordUserID,
	); err != nil {
		return fmt.Errorf("write replacement link %s/%s: %w", guildID, inGameName, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replacement link %s/%s: %w", guildID, inGameName, err)
	}
	return nil
}

// SaveLink records or replaces the link for an Among Us player name.
//
// A name maps to at most one Discord user per guild, enforced by the primary
// key: linking a name that is already taken moves it rather than creating a
// second row that a later read would resolve arbitrarily.
func (d *DB) SaveLink(guildID, inGameName, discordUserID string) error {
	if guildID == "" || inGameName == "" || discordUserID == "" {
		return errors.New("save link: guild, player name and user id are all required")
	}

	_, err := d.db.Exec(`
		INSERT INTO player_link (guild_id, in_game_name, discord_user_id, updated_at)
		VALUES (?, ?, ?, unixepoch())
		ON CONFLICT(guild_id, in_game_name) DO UPDATE SET
			discord_user_id = excluded.discord_user_id,
			updated_at      = unixepoch()`,
		guildID, inGameName, discordUserID)
	if err != nil {
		return fmt.Errorf("save link %s/%s: %w", guildID, inGameName, err)
	}
	return nil
}

// Links returns every link in a guild, ordered by player name so the result is
// reproducible.
func (d *DB) Links(guildID string) ([]PlayerLink, error) {
	rows, err := d.db.Query(`
		SELECT in_game_name, discord_user_id, updated_at
		FROM player_link WHERE guild_id = ? ORDER BY in_game_name`, guildID)
	if err != nil {
		return nil, fmt.Errorf("read links for %s: %w", guildID, err)
	}
	defer rows.Close()

	var links []PlayerLink
	for rows.Next() {
		var link PlayerLink
		if err := rows.Scan(&link.InGameName, &link.DiscordUserID, &link.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan link for %s: %w", guildID, err)
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read links for %s: %w", guildID, err)
	}
	return links, nil
}

// DeleteLink removes the link for one player name. Removing a link that is not
// there is not an error: unlinking should be idempotent.
func (d *DB) DeleteLink(guildID, inGameName string) error {
	_, err := d.db.Exec(
		"DELETE FROM player_link WHERE guild_id = ? AND in_game_name = ?", guildID, inGameName)
	if err != nil {
		return fmt.Errorf("delete link %s/%s: %w", guildID, inGameName, err)
	}
	return nil
}

// DeleteLinksForUser removes every link a Discord user holds in a guild. A user
// can hold at most one in practice, but /au unlink must not depend on that.
func (d *DB) DeleteLinksForUser(guildID, discordUserID string) error {
	_, err := d.db.Exec(
		"DELETE FROM player_link WHERE guild_id = ? AND discord_user_id = ?", guildID, discordUserID)
	if err != nil {
		return fmt.Errorf("delete links for user %s/%s: %w", guildID, discordUserID, err)
	}
	return nil
}

// ClearLinks removes every link in a guild, for the end of a round.
func (d *DB) ClearLinks(guildID string) error {
	if _, err := d.db.Exec("DELETE FROM player_link WHERE guild_id = ?", guildID); err != nil {
		return fmt.Errorf("clear links for %s: %w", guildID, err)
	}
	return nil
}
