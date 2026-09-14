package sqlite

import "testing"

func TestAVoiceHoldIsRememberedUpdatedAndForgotten(t *testing.T) {
	db, _ := openTemp(t)

	none, err := db.VoiceHold("guild", "member")
	if err != nil || !none.Empty() || none.GuildID != "guild" || none.UserID != "member" {
		t.Fatalf("a member without a record: %+v, %v", none, err)
	}

	held := VoiceHold{GuildID: "guild", UserID: "member", Muted: true, Deafened: true}
	if err := db.SaveVoiceHold(held); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got, _ := db.VoiceHold("guild", "member"); got != held {
		t.Fatalf("read back %+v, want %+v", got, held)
	}

	ghost := VoiceHold{GuildID: "guild", UserID: "member", GhostChannelID: "ghost"}
	if err := db.SaveVoiceHold(ghost); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got, _ := db.VoiceHold("guild", "member"); got != ghost {
		t.Fatalf("after update %+v, want %+v", got, ghost)
	}

	if err := db.SaveVoiceHold(VoiceHold{GuildID: "guild", UserID: "member"}); err != nil {
		t.Fatalf("release: %v", err)
	}
	if holds, _ := db.VoiceHolds(); len(holds) != 0 {
		t.Errorf("a released hold is still on record: %+v", holds)
	}
}

func TestVoiceHoldsAreListedInOrder(t *testing.T) {
	db, _ := openTemp(t)

	for _, hold := range []VoiceHold{
		{GuildID: "guild-b", UserID: "member-1", Muted: true},
		{GuildID: "guild-a", UserID: "member-2", Deafened: true},
		{GuildID: "guild-a", UserID: "member-1", GhostChannelID: "ghost"},
	} {
		if err := db.SaveVoiceHold(hold); err != nil {
			t.Fatalf("save %+v: %v", hold, err)
		}
	}

	holds, err := db.VoiceHolds()
	if err != nil || len(holds) != 3 {
		t.Fatalf("list: %+v, %v", holds, err)
	}
	order := []string{holds[0].GuildID + "/" + holds[0].UserID, holds[1].GuildID + "/" + holds[1].UserID,
		holds[2].GuildID + "/" + holds[2].UserID}
	if order[0] != "guild-a/member-1" || order[1] != "guild-a/member-2" || order[2] != "guild-b/member-1" {
		t.Errorf("unexpected order %v", order)
	}
}

func TestAVoiceHoldNeedsAGuildAndAMember(t *testing.T) {
	db, _ := openTemp(t)

	if err := db.SaveVoiceHold(VoiceHold{UserID: "member", Muted: true}); err == nil {
		t.Error("a hold without a guild was saved")
	}
	if err := db.SaveVoiceHold(VoiceHold{GuildID: "guild", Muted: true}); err == nil {
		t.Error("a hold without a member was saved")
	}
}
