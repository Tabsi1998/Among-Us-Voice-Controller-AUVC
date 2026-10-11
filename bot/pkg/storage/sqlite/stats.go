package sqlite

import (
	"database/sql"
	"errors"
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
	Name          string
	DiscordUserID string
	Rounds        int
	Wins          int
	Impostor      int
	ImpostorWins  int
}

// Boards are the orders /au stats can list players in.
const (
	BoardRounds       = "rounds"
	BoardWinRate      = "win_rate"
	BoardImpostorRate = "impostor_rate"
)

// Rates need enough rounds to mean something; below that a player is not ranked.
const (
	MinRoundsForRate   = 5
	MinImpostorForRate = 3
)

// RecordRound adds one finished round to every player in it, in one
// transaction: a round counts for all of its players or for none. A player
// linked to a Discord account is counted for the account; rounds counted under
// the name before the link are moved over the first time.
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
		key, userID, err := playerKey(tx, guildID, name)
		if err != nil {
			return fmt.Errorf("record round for %s: %w", guildID, err)
		}
		if userID != "" {
			if err := foldNameIntoUser(tx, guildID, name, key, userID); err != nil {
				return fmt.Errorf("record round for %s: %w", guildID, err)
			}
		}
		won, impostor := boolInt(player.Won), boolInt(player.Impostor)
		if _, err := tx.Exec(`
			INSERT INTO round_stats (guild_id, player_key, discord_user_id, last_name, rounds, wins, impostor, impostor_wins, updated_at)
			VALUES (?, ?, ?, ?, 1, ?, ?, ?, unixepoch())
			ON CONFLICT(guild_id, player_key) DO UPDATE SET
				last_name     = excluded.last_name,
				rounds        = rounds + 1,
				wins          = wins + excluded.wins,
				impostor      = impostor + excluded.impostor,
				impostor_wins = impostor_wins + excluded.impostor_wins,
				updated_at    = unixepoch()`,
			guildID, key, userID, name, won, impostor, won*impostor); err != nil {
			return fmt.Errorf("record round for %s: %w", guildID, err)
		}
	}
	return tx.Commit()
}

// playerKey is the account a name is linked to, or the name itself.
func playerKey(tx *sql.Tx, guildID, name string) (key, userID string, err error) {
	err = tx.QueryRow(
		"SELECT discord_user_id FROM player_link WHERE guild_id = ? AND in_game_name = ? COLLATE NOCASE LIMIT 1",
		guildID, name).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "name:" + strings.ToLower(name), "", nil
	}
	if err != nil {
		return "", "", err
	}
	return "user:" + userID, userID, nil
}

// foldNameIntoUser moves the rounds a name played before it was linked to the account.
func foldNameIntoUser(tx *sql.Tx, guildID, name, key, userID string) error {
	nameKey := "name:" + strings.ToLower(name)
	if _, err := tx.Exec(`
		INSERT INTO round_stats (guild_id, player_key, discord_user_id, last_name, rounds, wins, impostor, impostor_wins, updated_at)
		SELECT guild_id, ?, ?, last_name, rounds, wins, impostor, impostor_wins, unixepoch()
		FROM round_stats WHERE guild_id = ? AND player_key = ?
		ON CONFLICT(guild_id, player_key) DO UPDATE SET
			rounds        = rounds + excluded.rounds,
			wins          = wins + excluded.wins,
			impostor      = impostor + excluded.impostor,
			impostor_wins = impostor_wins + excluded.impostor_wins`,
		key, userID, guildID, nameKey); err != nil {
		return err
	}
	_, err := tx.Exec("DELETE FROM round_stats WHERE guild_id = ? AND player_key = ?", guildID, nameKey)
	return err
}

// PlayerStats returns a guild's players in the order of a board, at most limit.
// The rate boards leave out who played too few rounds to be ranked.
func (d *DB) PlayerStats(guildID, board string, limit int) ([]PlayerStat, error) {
	query := `SELECT last_name, discord_user_id, rounds, wins, impostor, impostor_wins FROM round_stats WHERE guild_id = ?`
	switch board {
	case BoardWinRate:
		query += fmt.Sprintf(" AND rounds >= %d ORDER BY CAST(wins AS REAL) / rounds DESC, rounds DESC", MinRoundsForRate)
	case BoardImpostorRate:
		query += fmt.Sprintf(" AND impostor >= %d ORDER BY CAST(impostor_wins AS REAL) / impostor DESC, impostor DESC", MinImpostorForRate)
	default:
		query += " ORDER BY rounds DESC, wins DESC"
	}
	rows, err := d.db.Query(query+", last_name LIMIT ?", guildID, limit)
	if err != nil {
		return nil, fmt.Errorf("read player stats for %s: %w", guildID, err)
	}
	defer rows.Close()

	var stats []PlayerStat
	for rows.Next() {
		var stat PlayerStat
		if err := rows.Scan(&stat.Name, &stat.DiscordUserID, &stat.Rounds, &stat.Wins, &stat.Impostor, &stat.ImpostorWins); err != nil {
			return nil, fmt.Errorf("read player stats for %s: %w", guildID, err)
		}
		stats = append(stats, stat)
	}
	return stats, rows.Err()
}

// ResetPlayerStats forgets every round of a guild.
func (d *DB) ResetPlayerStats(guildID string) error {
	if _, err := d.db.Exec("DELETE FROM round_stats WHERE guild_id = ?", guildID); err != nil {
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
