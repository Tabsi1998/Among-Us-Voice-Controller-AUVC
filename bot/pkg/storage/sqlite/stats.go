package sqlite

import (
	"fmt"
	"strings"
)

// RoundPlayer is one player's part in a finished round.
type RoundPlayer struct {
	Name     string
	Won      bool
	Impostor bool
}

// PlayerStat is what a player did in the rounds AUVC saw in a guild.
type PlayerStat struct {
	Name     string
	Rounds   int
	Wins     int
	Impostor int
}

// RecordRound adds one finished round to every player in it, in one
// transaction: a round counts for all of its players or for none.
func (d *DB) RecordRound(guildID string, players []RoundPlayer) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("record round for %s: %w", guildID, err)
	}
	defer tx.Rollback()

	for _, player := range players {
		name := strings.TrimSpace(player.Name)
		if name == "" {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO player_stats (guild_id, player_name, rounds, wins, impostor, updated_at)
			VALUES (?, ?, 1, ?, ?, unixepoch())
			ON CONFLICT(guild_id, player_name) DO UPDATE SET
				rounds     = rounds + 1,
				wins       = wins + excluded.wins,
				impostor   = impostor + excluded.impostor,
				updated_at = unixepoch()`,
			guildID, name, boolInt(player.Won), boolInt(player.Impostor)); err != nil {
			return fmt.Errorf("record round for %s: %w", guildID, err)
		}
	}
	return tx.Commit()
}

// PlayerStats returns a guild's players, most rounds first, at most limit.
func (d *DB) PlayerStats(guildID string, limit int) ([]PlayerStat, error) {
	rows, err := d.db.Query(`
		SELECT player_name, rounds, wins, impostor FROM player_stats
		WHERE guild_id = ? ORDER BY rounds DESC, wins DESC, player_name LIMIT ?`, guildID, limit)
	if err != nil {
		return nil, fmt.Errorf("read player stats for %s: %w", guildID, err)
	}
	defer rows.Close()

	var stats []PlayerStat
	for rows.Next() {
		var stat PlayerStat
		if err := rows.Scan(&stat.Name, &stat.Rounds, &stat.Wins, &stat.Impostor); err != nil {
			return nil, fmt.Errorf("read player stats for %s: %w", guildID, err)
		}
		stats = append(stats, stat)
	}
	return stats, rows.Err()
}

// ResetPlayerStats forgets every round of a guild.
func (d *DB) ResetPlayerStats(guildID string) error {
	if _, err := d.db.Exec("DELETE FROM player_stats WHERE guild_id = ?", guildID); err != nil {
		return fmt.Errorf("reset player stats for %s: %w", guildID, err)
	}
	return nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
