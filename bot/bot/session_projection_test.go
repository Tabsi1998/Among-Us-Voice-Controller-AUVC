package bot

import (
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/amongus"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
)

func projectionFixture() *GameState {
	dgs := NewDiscordGameState("guild")
	dgs.GameData.Phase = game.TASKS
	dgs.GameData.PlayerData = map[string]amongus.PlayerData{
		"Red":   {Name: "Red", Color: 0, IsAlive: true},
		"Blue":  {Name: "Blue", Color: 1, IsAlive: false},
		"Green": {Name: "Green", Color: 2, IsAlive: true},
	}
	dgs.UserData = UserDataSet{
		"200": {User: User{UserID: "200"}, InGameName: "Blue"},
		"100": {User: User{UserID: "100"}, InGameName: "Red"},
		"300": {User: User{UserID: "300"}, InGameName: amongus.UnlinkedPlayerName},
		"400": {User: User{UserID: "400", IsBot: true}, InGameName: "Green"},
	}
	return dgs
}

func TestSessionStateCarriesPhaseAndAliveness(t *testing.T) {
	state := projectionFixture().SessionState()

	if state.Phase != game.TASKS {
		t.Errorf("phase = %v, want TASKS", state.Phase)
	}

	alive, ok := state.Player("100")
	if !ok || !alive.Alive || alive.InGameName != "Red" {
		t.Errorf("living player projected wrong: %+v (found=%v)", alive, ok)
	}

	dead, ok := state.Player("200")
	if !ok || dead.Alive || dead.InGameName != "Blue" {
		t.Errorf("dead player projected wrong: %+v (found=%v)", dead, ok)
	}
}

// The sentinel name means "no player", not a player literally called
// UnlinkedPlayer. Leaking it into the projection would make the policy try to
// manage spectators.
func TestSessionStateTreatsSentinelNameAsUnlinked(t *testing.T) {
	state := projectionFixture().SessionState()

	player, ok := state.Player("300")
	if !ok {
		t.Fatal("expected the unlinked user to be projected")
	}
	if player.Linked() {
		t.Errorf("sentinel name leaked into the projection: %q", player.InGameName)
	}
	if !player.Alive {
		t.Error("an unlinked user must not be projected as dead")
	}
}

func TestSessionStateExcludesBotsFromManagement(t *testing.T) {
	state := projectionFixture().SessionState()

	bot, ok := state.Player("400")
	if !ok {
		t.Fatal("expected the bot account to be projected")
	}
	if !bot.Linked() {
		t.Error("the bot is linked to Green and should report as linked")
	}
	if bot.Managed() {
		t.Error("a bot account must never be managed by the voice policy")
	}

	if got := state.CountManaged(); got != 2 {
		t.Errorf("CountManaged = %d, want 2 (Red and Blue only)", got)
	}
}

// UserData is a map, so an unsorted projection would reorder between runs and
// make any downstream diff or log noisy.
func TestSessionStateIsOrderedByUserID(t *testing.T) {
	state := projectionFixture().SessionState()

	want := []string{"100", "200", "300", "400"}
	if len(state.Players) != len(want) {
		t.Fatalf("expected %d players, got %d", len(want), len(state.Players))
	}
	for i, id := range want {
		if state.Players[i].UserID != id {
			t.Fatalf("player %d = %q, want %q", i, state.Players[i].UserID, id)
		}
	}
}

// A link can outlive the player it points at, for example after a name change.
// The projection must not invent aliveness for a player that is gone.
func TestSessionStateKeepsLinkWhenPlayerIsMissing(t *testing.T) {
	dgs := projectionFixture()
	delete(dgs.GameData.PlayerData, "Red")

	player, ok := dgs.SessionState().Player("100")
	if !ok {
		t.Fatal("expected the user to remain projected")
	}
	if !player.Linked() {
		t.Error("the link must survive a missing player entry")
	}
	if !player.Alive {
		t.Error("a missing player entry must default to alive, not dead")
	}
}
