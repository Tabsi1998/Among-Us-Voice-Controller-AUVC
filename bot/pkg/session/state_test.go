package session

import (
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
)

func sample() State {
	return State{
		Phase: game.TASKS,
		Players: []PlayerState{
			{UserID: "alive", InGameName: "Red", Alive: true},
			{UserID: "dead", InGameName: "Blue", Alive: false},
			{UserID: "unlinked", Alive: true},
			{UserID: "musicbot", InGameName: "Green", Alive: true, Bot: true},
		},
	}
}

// The contract is explicit that bots and unlinked users are never managed.
// Getting this wrong means the bot moves or mutes people it has no business
// touching, so it is asserted directly rather than implied by other tests.
func TestManagedExcludesUnlinkedUsersAndBots(t *testing.T) {
	managed := sample().Managed()

	if len(managed) != 2 {
		t.Fatalf("expected 2 managed players, got %d: %+v", len(managed), managed)
	}
	for _, player := range managed {
		if !player.Linked() {
			t.Errorf("unlinked user %q reported as managed", player.UserID)
		}
		if player.Bot {
			t.Errorf("bot account %q reported as managed", player.UserID)
		}
	}
}

func TestLinkedIgnoresAliveAndBotFlags(t *testing.T) {
	if (PlayerState{UserID: "u"}).Linked() {
		t.Error("a player without an in-game name must not count as linked")
	}
	if !(PlayerState{UserID: "u", InGameName: "Red", Bot: true}).Linked() {
		t.Error("a bot linked to a player is still linked, only not managed")
	}
}

func TestCounts(t *testing.T) {
	state := sample()

	if got := state.CountManaged(); got != 2 {
		t.Errorf("CountManaged = %d, want 2", got)
	}
	if got := state.CountAlive(); got != 1 {
		t.Errorf("CountAlive = %d, want 1", got)
	}
	if got := state.CountDead(); got != 1 {
		t.Errorf("CountDead = %d, want 1", got)
	}
}

// An unlinked user must never inflate the dead count; the policy would
// otherwise try to move a spectator into the ghost channel.
func TestUnlinkedUsersDoNotCountAsDead(t *testing.T) {
	state := State{Players: []PlayerState{{UserID: "spectator", Alive: false}}}

	if got := state.CountDead(); got != 0 {
		t.Errorf("CountDead = %d, want 0 for an unlinked user", got)
	}
}

func TestPlayerLookup(t *testing.T) {
	state := sample()

	player, ok := state.Player("dead")
	if !ok {
		t.Fatal("expected to find the dead player")
	}
	if player.InGameName != "Blue" || player.Alive {
		t.Errorf("unexpected player state: %+v", player)
	}

	if _, ok := state.Player("absent"); ok {
		t.Error("expected no result for an unknown user id")
	}
}
