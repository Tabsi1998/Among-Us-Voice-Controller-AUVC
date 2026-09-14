package au

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
)

func openLinkService(t *testing.T) (*Service, *sqlite.DB) {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "auvc.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewService(db, "test", "test"), db
}

func TestTheAppLinksMovesAndUnlinksACrewmate(t *testing.T) {
	service, db := openLinkService(t)

	if err := service.LinkFromApp("guild", " Red ", "user-1"); err != nil {
		t.Fatalf("link: %v", err)
	}
	// One link per member, as with /au link: the same member on another
	// crewmate moves the link.
	if err := service.LinkFromApp("guild", "Blue", "user-1"); err != nil {
		t.Fatalf("move: %v", err)
	}
	links, err := db.Links("guild")
	if err != nil || len(links) != 1 || links[0].InGameName != "Blue" || links[0].DiscordUserID != "user-1" {
		t.Fatalf("links %+v (%v)", links, err)
	}

	if err := service.LinkFromApp("guild", "Blue", ""); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if links, _ := db.Links("guild"); len(links) != 0 {
		t.Errorf("still linked: %+v", links)
	}
}

func TestTheAppCannotLinkWithoutAGuildOrAPlayer(t *testing.T) {
	service, db := openLinkService(t)

	for name, call := range map[string][3]string{
		"no guild":        {"", "Red", "user-1"},
		"no player":       {"guild", "", "user-1"},
		"a blank player":  {"guild", "   ", "user-1"},
		"unlink no guild": {"", "Red", ""},
	} {
		if err := service.LinkFromApp(call[0], call[1], call[2]); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: got %v, want %v", name, err, ErrInvalidInput)
		}
	}
	if links, _ := db.Links("guild"); len(links) != 0 {
		t.Errorf("a refused link was stored: %+v", links)
	}
}
