package storage

import (
	"github.com/jackc/pgconn"
	"github.com/pashagolub/pgxmock"
	"testing"
)

const (
	UserID            = "123123123123123123"
	UserIDInt  uint64 = 123123123123123123
	GuildID           = "234234234234234234"
	GuildIDInt uint64 = 234234234234234234
)

func TestInsertUser(t *testing.T) {
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	mock.ExpectExec("^INSERT INTO users VALUES ((.+), true, NULL)(.+)$").
		WithArgs(UserIDInt).WillReturnResult(pgconn.CommandTag{})

	err = insertUser(mock, UserIDInt)
	if err != nil {
		t.Error(err)
	}

	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestGetUser(t *testing.T) {
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	// make sure an empty response (no user) returns an error
	mock.ExpectQuery("^SELECT (.+) FROM users WHERE user_id = (.+)$").
		WithArgs(UserIDInt).
		WillReturnRows(
			pgxmock.NewRows([]string{"user_id", "opt", "vote_time_unix"}))

	user, err := getUser(mock, UserIDInt)
	if err == nil {
		t.Error("error should not be nil when no users are returned")
	}
	if user != nil {
		t.Error("user should be nil")
	}

	// make sure a populated response doesn't return an error
	mock.ExpectQuery("^SELECT (.+) FROM users WHERE user_id = (.+)$").
		WithArgs(UserIDInt).
		WillReturnRows(
			pgxmock.NewRows([]string{"user_id", "opt", "vote_time_unix"}).
				AddRow(UserIDInt, true, nil))

	user, err = getUser(mock, UserIDInt)
	if err != nil {
		t.Error(err)
	}
	if user == nil {
		t.Error("expected user to not be nil")
	}
	if user.UserID != UserIDInt || !user.Opt {
		t.Error("userID or opt mismatches what was returned from Postgres")
	}

	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestOptUser(t *testing.T) {
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	mock.ExpectQuery("^SELECT (.+) FROM users WHERE user_id = (.+)$").
		WithArgs(UserIDInt).
		WillReturnRows(
			pgxmock.NewRows([]string{"user_id", "opt", "vote_time_unix"}).
				AddRow(UserIDInt, true, nil)) //return the vote time being now

	err = optUser(mock, UserIDInt, true)
	if err == nil {
		t.Error("Expected opting a user that is already opted to fail with error")
	}

	mock.ExpectQuery("^SELECT (.+) FROM users WHERE user_id = (.+)$").
		WithArgs(UserIDInt).
		WillReturnRows(
			pgxmock.NewRows([]string{"user_id", "opt", "vote_time_unix"}).
				AddRow(UserIDInt, true, nil)) //return the vote time being now

	// expect to de-op the user
	mock.ExpectExec("^UPDATE users SET opt = (.+) WHERE user_id = (.+)$").
		WithArgs(false, UserIDInt).
		WillReturnResult(pgconn.CommandTag{})

	// expect the respective game_events to be unlinked from the user
	mock.ExpectExec("^UPDATE game_events SET user_id = NULL WHERE user_id = (.+)$").
		WithArgs(UserIDInt).
		WillReturnResult(pgconn.CommandTag{})

	// expect all the user's games to be deleted
	mock.ExpectExec("^DELETE FROM users_games WHERE user_id = (.+)$").
		WithArgs(UserIDInt).
		WillReturnResult(pgconn.CommandTag{})

	err = optUser(mock, UserIDInt, false)
	if err != nil {
		t.Error(err)
	}

	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
