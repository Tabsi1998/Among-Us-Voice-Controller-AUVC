package sqlite

import (
	"errors"
	"testing"
)

func TestACrewmateBoardIsRememberedReplacedAndForgotten(t *testing.T) {
	db, _ := openTemp(t)

	if _, err := db.CrewmateBoard("guild"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a guild without a board: got %v, want ErrNotFound", err)
	}

	first := CrewmateBoard{GuildID: "guild", ChannelID: "text-1", MessageID: "message-1"}
	if err := db.SaveCrewmateBoard(first); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got, err := db.CrewmateBoard("guild"); err != nil || got != first {
		t.Fatalf("read back %+v, %v; want %+v", got, err, first)
	}

	// Moving the board to another channel replaces it: a guild has one.
	moved := CrewmateBoard{GuildID: "guild", ChannelID: "text-2", MessageID: "message-2"}
	if err := db.SaveCrewmateBoard(moved); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if err := db.SaveCrewmateBoard(CrewmateBoard{GuildID: "other", ChannelID: "text-3", MessageID: "message-3"}); err != nil {
		t.Fatalf("save another guild: %v", err)
	}

	boards, err := db.CrewmateBoards()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(boards) != 2 || boards[0] != moved || boards[1].GuildID != "other" {
		t.Fatalf("unexpected boards: %+v", boards)
	}

	if err := db.DeleteCrewmateBoard("guild"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := db.DeleteCrewmateBoard("guild"); err != nil {
		t.Fatalf("deleting twice must not fail: %v", err)
	}
	if _, err := db.CrewmateBoard("guild"); !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete: got %v, want ErrNotFound", err)
	}
}

func TestACrewmateBoardNeedsEveryId(t *testing.T) {
	db, _ := openTemp(t)

	for _, board := range []CrewmateBoard{
		{ChannelID: "text", MessageID: "message"},
		{GuildID: "guild", MessageID: "message"},
		{GuildID: "guild", ChannelID: "text"},
	} {
		if err := db.SaveCrewmateBoard(board); err == nil {
			t.Errorf("%+v was saved", board)
		}
	}
}
