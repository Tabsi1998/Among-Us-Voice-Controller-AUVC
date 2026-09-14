package pairing

import (
	"path/filepath"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
)

func TestRevokingTellsTheListener(t *testing.T) {
	service, _ := testService(t)
	pairAndRedeem(t, service)

	var told []string
	service.OnRevoke(func(guildID string) { told = append(told, guildID) })

	if _, err := service.Revoke(guild); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if len(told) != 1 || told[0] != guild {
		t.Errorf("the listener was told %v, want [%s]", told, guild)
	}
}

// A credential revoked earlier can still hold a connection opened before that
// revoke. Running the command again has to be able to end it.
func TestRevokingTellsTheListenerEvenWhenNothingWasLeftToRevoke(t *testing.T) {
	service, _ := testService(t)

	told := 0
	service.OnRevoke(func(string) { told++ })

	revoked, err := service.Revoke(guild)
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if revoked != 0 {
		t.Fatalf("revoked %d credentials in an unpaired guild", revoked)
	}
	if told != 1 {
		t.Errorf("the listener was told %d times, want 1", told)
	}
}

// A revoke that did not reach the database changed nothing, and ending
// connections for it would tell capture a revoke happened that did not.
func TestAFailedRevokeTellsNobody(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "auvc.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	service := NewService(db)
	db.Close()

	told := 0
	service.OnRevoke(func(string) { told++ })

	if _, err := service.Revoke(guild); err == nil {
		t.Fatal("a revoke against a closed database succeeded")
	}
	if told != 0 {
		t.Errorf("the listener was told about a revoke that failed")
	}
}
