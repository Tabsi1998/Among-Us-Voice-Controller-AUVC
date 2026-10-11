package sqlite

import "testing"

// Edge case: a player plays rounds unlinked under one case of their name,
// is then linked under a different case; past rounds should fold into the account.
func TestUnlinkedThenLinkedUnderDifferentCase(t *testing.T) {
	db, _ := openTemp(t)
	// Two rounds unlinked under "ALICE"
	round(t, db, RoundPlayer{Name: "ALICE", Won: true})
	round(t, db, RoundPlayer{Name: "alice", Impostor: true, Won: true})
	// Now link under "Alice" (different case)
	if err := db.SaveLink("guild", "Alice", "user-42"); err != nil {
		t.Fatal(err)
	}
	// Record a round under yet another case
	round(t, db, RoundPlayer{Name: "aLiCe"})

	stats := statsOf(t, db, BoardRounds)
	if len(stats) != 1 {
		t.Fatalf("expected one row (folded), got %+v", stats)
	}
	want := PlayerStat{Name: "aLiCe", DiscordUserID: "user-42", Rounds: 3, Wins: 2, Impostor: 1, ImpostorWins: 1}
	if stats[0] != want {
		t.Fatalf("stats[0] = %+v, want %+v", stats[0], want)
	}
}
