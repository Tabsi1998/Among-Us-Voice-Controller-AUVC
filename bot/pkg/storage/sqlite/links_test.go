package sqlite

import "testing"

func TestSaveAndReadLinks(t *testing.T) {
	db, _ := openTemp(t)

	for _, l := range []struct{ name, user string }{
		{"Red", "user-1"},
		{"Blue", "user-2"},
	} {
		if err := db.SaveLink("guild", l.name, l.user); err != nil {
			t.Fatalf("save %s: %v", l.name, err)
		}
	}

	links, err := db.Links("guild")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}
	// Ordered by player name so the result is reproducible.
	if links[0].InGameName != "Blue" || links[1].InGameName != "Red" {
		t.Errorf("links are not ordered by player name: %+v", links)
	}
	if links[0].UpdatedAt == 0 {
		t.Error("updated_at was not set")
	}
}

// Linking a name that is already taken must move it. A second row would leave
// the reader to pick one arbitrarily, and the wrong player would get muted.
func TestSaveLinkReplacesAnExistingName(t *testing.T) {
	db, _ := openTemp(t)

	if err := db.SaveLink("guild", "Red", "user-1"); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := db.SaveLink("guild", "Red", "user-2"); err != nil {
		t.Fatalf("second save: %v", err)
	}

	links, err := db.Links("guild")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected the name to be moved, got %d links: %+v", len(links), links)
	}
	if links[0].DiscordUserID != "user-2" {
		t.Errorf("link points at %q, want user-2", links[0].DiscordUserID)
	}
}

func TestLinksAreScopedPerGuild(t *testing.T) {
	db, _ := openTemp(t)

	if err := db.SaveLink("guild-a", "Red", "user-1"); err != nil {
		t.Fatalf("save a: %v", err)
	}
	if err := db.SaveLink("guild-b", "Red", "user-2"); err != nil {
		t.Fatalf("save b: %v", err)
	}

	a, err := db.Links("guild-a")
	if err != nil {
		t.Fatalf("read a: %v", err)
	}
	if len(a) != 1 || a[0].DiscordUserID != "user-1" {
		t.Errorf("guild-a leaked: %+v", a)
	}
}

func TestSaveLinkRejectsEmptyValues(t *testing.T) {
	db, _ := openTemp(t)

	for _, c := range []struct{ guild, name, user string }{
		{"", "Red", "user"},
		{"guild", "", "user"},
		{"guild", "Red", ""},
	} {
		if err := db.SaveLink(c.guild, c.name, c.user); err == nil {
			t.Errorf("expected an error for %+v", c)
		}
	}
}

// Unlinking must be idempotent: /au unlink should not fail because the user
// already unlinked.
func TestDeleteLinkIsIdempotent(t *testing.T) {
	db, _ := openTemp(t)

	if err := db.DeleteLink("guild", "absent"); err != nil {
		t.Errorf("deleting a missing link failed: %v", err)
	}

	if err := db.SaveLink("guild", "Red", "user-1"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := db.DeleteLink("guild", "Red"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := db.DeleteLink("guild", "Red"); err != nil {
		t.Errorf("second delete failed: %v", err)
	}

	links, err := db.Links("guild")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("expected no links, got %+v", links)
	}
}

func TestDeleteLinksForUser(t *testing.T) {
	db, _ := openTemp(t)

	if err := db.SaveLink("guild", "Red", "user-1"); err != nil {
		t.Fatalf("save red: %v", err)
	}
	if err := db.SaveLink("guild", "Blue", "user-2"); err != nil {
		t.Fatalf("save blue: %v", err)
	}

	if err := db.DeleteLinksForUser("guild", "user-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	links, err := db.Links("guild")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(links) != 1 || links[0].DiscordUserID != "user-2" {
		t.Errorf("wrong links remain: %+v", links)
	}
}

func TestClearLinksOnlyAffectsOneGuild(t *testing.T) {
	db, _ := openTemp(t)

	if err := db.SaveLink("guild-a", "Red", "user-1"); err != nil {
		t.Fatalf("save a: %v", err)
	}
	if err := db.SaveLink("guild-b", "Blue", "user-2"); err != nil {
		t.Fatalf("save b: %v", err)
	}

	if err := db.ClearLinks("guild-a"); err != nil {
		t.Fatalf("clear: %v", err)
	}

	a, err := db.Links("guild-a")
	if err != nil {
		t.Fatalf("read a: %v", err)
	}
	if len(a) != 0 {
		t.Errorf("guild-a still has links: %+v", a)
	}

	b, err := db.Links("guild-b")
	if err != nil {
		t.Fatalf("read b: %v", err)
	}
	if len(b) != 1 {
		t.Errorf("clearing guild-a affected guild-b: %+v", b)
	}
}
