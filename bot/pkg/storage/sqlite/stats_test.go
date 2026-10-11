package sqlite

import "testing"

func round(t *testing.T, db *DB, players ...RoundPlayer) {
	t.Helper()
	if err := db.RecordRound("guild", players); err != nil {
		t.Fatalf("record round: %v", err)
	}
}

func statsOf(t *testing.T, db *DB, board string) []PlayerStat {
	t.Helper()
	stats, err := db.PlayerStats("guild", board, 20)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	return stats
}

// A linked player keeps their rounds under a new name in the game, and what the
// name played before the link moves to the account.
func TestALinkedPlayerIsCountedByAccount(t *testing.T) {
	db, _ := openTemp(t)
	round(t, db, RoundPlayer{Name: "Mira", Impostor: true, Won: true})
	if err := db.SaveLink("guild", "Mira", "user-1"); err != nil {
		t.Fatal(err)
	}
	round(t, db, RoundPlayer{Name: "mira", Won: true})
	if err := db.ReplaceLink("guild", "Mira2", "user-1"); err != nil {
		t.Fatal(err)
	}
	round(t, db, RoundPlayer{Name: "Mira2"})

	stats := statsOf(t, db, BoardRounds)

	want := PlayerStat{Name: "Mira2", DiscordUserID: "user-1", Rounds: 3, Wins: 2, Impostor: 1, ImpostorWins: 1}
	if len(stats) != 1 || stats[0] != want {
		t.Fatalf("stats = %+v, want only %+v", stats, want)
	}
}

// Without a link a player is their name, whatever its case.
func TestAnUnlinkedPlayerIsTheirName(t *testing.T) {
	db, _ := openTemp(t)
	round(t, db, RoundPlayer{Name: "Leon"})
	round(t, db, RoundPlayer{Name: "LEON", Won: true})

	if stats := statsOf(t, db, BoardRounds); len(stats) != 1 || stats[0].Rounds != 2 || stats[0].Name != "LEON" {
		t.Fatalf("stats = %+v", stats)
	}
}

// The rate boards rank by rate and leave out who played too little.
func TestTheBoardsRankByRate(t *testing.T) {
	db, _ := openTemp(t)
	for i := 0; i < MinRoundsForRate; i++ {
		round(t, db,
			RoundPlayer{Name: "Often", Won: i < 2},
			RoundPlayer{Name: "Lucky", Won: true, Impostor: i < MinImpostorForRate},
			RoundPlayer{Name: "Imp", Impostor: true, Won: i == 0})
	}
	round(t, db, RoundPlayer{Name: "Once", Won: true})

	rate := statsOf(t, db, BoardWinRate)
	if len(rate) != 3 || rate[0].Name != "Lucky" || rate[2].Name != "Imp" {
		t.Errorf("win rate board = %+v", rate)
	}
	impostors := statsOf(t, db, BoardImpostorRate)
	if len(impostors) != 2 || impostors[0].Name != "Lucky" || impostors[0].ImpostorWins != MinImpostorForRate || impostors[1].Name != "Imp" {
		t.Errorf("impostor board = %+v", impostors)
	}
	if most := statsOf(t, db, BoardRounds); len(most) != 4 || most[3].Name != "Once" {
		t.Errorf("rounds board = %+v", most)
	}
}
