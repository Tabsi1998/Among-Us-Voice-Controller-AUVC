package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
)

// CrewmateBoard is the message in a guild's control channel where players
// choose their crewmate.
type CrewmateBoard struct {
	GuildID   string
	ChannelID string
	MessageID string
}

// CrewmateBoard returns a guild's board, or ErrNotFound when it has none.
func (d *DB) CrewmateBoard(guildID string) (CrewmateBoard, error) {
	board := CrewmateBoard{GuildID: guildID}
	err := d.db.QueryRow(
		"SELECT channel_id, message_id FROM crewmate_board WHERE guild_id = ?", guildID,
	).Scan(&board.ChannelID, &board.MessageID)
	if errors.Is(err, sql.ErrNoRows) {
		return CrewmateBoard{}, ErrNotFound
	}
	if err != nil {
		return CrewmateBoard{}, fmt.Errorf("read crewmate board for %s: %w", guildID, err)
	}
	return board, nil
}

// SaveCrewmateBoard records where a guild's board is, replacing the previous
// one.
func (d *DB) SaveCrewmateBoard(board CrewmateBoard) error {
	if board.GuildID == "" || board.ChannelID == "" || board.MessageID == "" {
		return errors.New("save crewmate board: guild, channel and message id are all required")
	}

	_, err := d.db.Exec(`
		INSERT INTO crewmate_board (guild_id, channel_id, message_id, updated_at)
		VALUES (?, ?, ?, unixepoch())
		ON CONFLICT(guild_id) DO UPDATE SET
			channel_id = excluded.channel_id,
			message_id = excluded.message_id,
			updated_at = unixepoch()`,
		board.GuildID, board.ChannelID, board.MessageID)
	if err != nil {
		return fmt.Errorf("save crewmate board for %s: %w", board.GuildID, err)
	}
	return nil
}

// DeleteCrewmateBoard forgets a guild's board. Forgetting one that is not there
// is not an error.
func (d *DB) DeleteCrewmateBoard(guildID string) error {
	if _, err := d.db.Exec("DELETE FROM crewmate_board WHERE guild_id = ?", guildID); err != nil {
		return fmt.Errorf("delete crewmate board for %s: %w", guildID, err)
	}
	return nil
}

// CrewmateBoards returns every board on record, ordered by guild, so a stopping
// bot can remove the ones it posted in an earlier run too.
func (d *DB) CrewmateBoards() ([]CrewmateBoard, error) {
	rows, err := d.db.Query(
		"SELECT guild_id, channel_id, message_id FROM crewmate_board ORDER BY guild_id")
	if err != nil {
		return nil, fmt.Errorf("read crewmate boards: %w", err)
	}
	defer rows.Close()

	var boards []CrewmateBoard
	for rows.Next() {
		var board CrewmateBoard
		if err := rows.Scan(&board.GuildID, &board.ChannelID, &board.MessageID); err != nil {
			return nil, fmt.Errorf("scan crewmate board: %w", err)
		}
		boards = append(boards, board)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read crewmate boards: %w", err)
	}
	return boards, nil
}
