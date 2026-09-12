package session

import (
	"reflect"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
)

// linkAll resolves every player to a Discord user named after them, none of
// them bots. It is the uninteresting case, so tests that care about something
// else can say so briefly.
func linkAll(name string) (string, bool, bool) {
	return "user-" + name, false, true
}

func names(players []GamePlayer) []string {
	list := make([]string, 0, len(players))
	for _, player := range players {
		list = append(list, player.Name)
	}
	return list
}

func TestASnapshotReplacesTheWholePlayerSet(t *testing.T) {
	live := NewLive()
	live.Reset(game.TASKS, []GamePlayer{
		{Name: "Red", Alive: true},
		{Name: "Blue", Alive: true},
	})

	// A reconnect brings a lobby that no longer has Blue in it.
	live.Reset(game.LOBBY, []GamePlayer{
		{Name: "Red", Alive: true},
		{Name: "Green", Alive: true},
	})

	if got, want := names(live.Players()), []string{"Green", "Red"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if live.Phase() != game.LOBBY {
		t.Errorf("phase is %v, want LOBBY", live.Phase())
	}
}

// Merging a snapshot instead of replacing would leave the bot managing the
// voice of somebody who is no longer in the game.
func TestASnapshotDropsPlayersWhoAreGone(t *testing.T) {
	live := NewLive()
	live.Reset(game.TASKS, []GamePlayer{{Name: "Red"}, {Name: "Blue"}})
	live.Reset(game.TASKS, []GamePlayer{{Name: "Red"}})

	state := live.Project(linkAll)
	if len(state.Players) != 1 || state.Players[0].InGameName != "Red" {
		t.Errorf("expected only Red, got %+v", state.Players)
	}
}

func TestASnapshotIgnoresANamelessPlayer(t *testing.T) {
	live := NewLive()
	live.Reset(game.TASKS, []GamePlayer{{Name: "Red"}, {Name: ""}})

	if got := len(live.Players()); got != 1 {
		t.Errorf("expected one usable player, got %d", got)
	}
}

func TestSetPhaseReportsWhetherAnythingChanged(t *testing.T) {
	live := NewLive()
	live.Reset(game.LOBBY, nil)

	if !live.SetPhase(game.TASKS) {
		t.Error("moving from lobby to tasks is a change")
	}
	if live.SetPhase(game.TASKS) {
		t.Error("staying in tasks is not a change")
	}
	if live.Phase() != game.TASKS {
		t.Errorf("phase is %v, want TASKS", live.Phase())
	}
}

// Capture can report a change for somebody the bot missed. Refusing to learn
// about them would leave one player unmanaged for the rest of the round.
func TestUpsertAddsAPlayerTheSessionHadNotSeen(t *testing.T) {
	live := NewLive()
	live.Reset(game.TASKS, []GamePlayer{{Name: "Red", Alive: true}})

	live.Upsert(GamePlayer{Name: "Blue", Alive: true})

	if got, want := names(live.Players()), []string{"Blue", "Red"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestUpsertReplacesWhatIsKnown(t *testing.T) {
	live := NewLive()
	live.Reset(game.TASKS, []GamePlayer{{Name: "Red", Alive: true, Color: 0}})

	live.Upsert(GamePlayer{Name: "Red", Alive: false, Color: 5})

	state := live.Project(linkAll)
	if len(state.Players) != 1 {
		t.Fatalf("expected one player, got %+v", state.Players)
	}
	if state.Players[0].Alive {
		t.Error("Red died and is still reported as alive")
	}
}

func TestRemoveDropsAPlayer(t *testing.T) {
	live := NewLive()
	live.Reset(game.TASKS, []GamePlayer{{Name: "Red"}, {Name: "Blue"}})

	live.Remove("Blue")

	if got, want := names(live.Players()), []string{"Red"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// A round ending is not everybody leaving. Dropping the players would mean the
// bot stops recognising the same lobby a moment later.
func TestEndingARoundKeepsThePlayersAndRevivesThem(t *testing.T) {
	live := NewLive()
	live.Reset(game.TASKS, []GamePlayer{
		{Name: "Red", Alive: false},
		{Name: "Blue", Alive: true},
	})

	live.EndRound()

	if live.Phase() != game.GAMEOVER {
		t.Errorf("phase is %v, want GAMEOVER", live.Phase())
	}
	for _, player := range live.Players() {
		if !player.Alive {
			t.Errorf("%s is still dead after the round ended", player.Name)
		}
	}
}

// A player the bot cannot tie to a Discord user must be absent from the
// projection entirely: one that is not in the map cannot be moved by mistake.
func TestUnresolvedPlayersAreAbsentFromTheProjection(t *testing.T) {
	live := NewLive()
	live.Reset(game.TASKS, []GamePlayer{{Name: "Red"}, {Name: "Stranger"}})

	state := live.Project(func(name string) (string, bool, bool) {
		if name == "Red" {
			return "user-red", false, true
		}
		return "", false, false
	})

	if len(state.Players) != 1 || state.Players[0].UserID != "user-red" {
		t.Errorf("expected only the linked player, got %+v", state.Players)
	}
}

func TestBotAccountsAreMarkedAndNotManaged(t *testing.T) {
	live := NewLive()
	live.Reset(game.TASKS, []GamePlayer{{Name: "MusicBot"}})

	state := live.Project(func(string) (string, bool, bool) { return "user-bot", true, true })

	if len(state.Players) != 1 {
		t.Fatalf("expected the bot to be projected, got %+v", state.Players)
	}
	if state.Players[0].Managed() {
		t.Error("a bot account must never be managed")
	}
	if len(state.Managed()) != 0 {
		t.Errorf("expected nothing managed, got %+v", state.Managed())
	}
}

// A disconnect is exactly when somebody might legitimately be sitting in
// another channel, and they are not in the game any more either way.
func TestDisconnectedPlayersAreLeftAlone(t *testing.T) {
	live := NewLive()
	live.Reset(game.TASKS, []GamePlayer{
		{Name: "Red", Alive: true},
		{Name: "Blue", Alive: true, Disconnected: true},
	})

	state := live.Project(linkAll)

	if len(state.Players) != 1 || state.Players[0].InGameName != "Red" {
		t.Errorf("expected only the connected player, got %+v", state.Players)
	}
}

// The projection is keyed by Discord user, so it has to be ordered that way or
// two identical sessions produce different values.
func TestTheProjectionIsReproducible(t *testing.T) {
	live := NewLive()
	live.Reset(game.TASKS, []GamePlayer{
		{Name: "Zed", Alive: true},
		{Name: "Amy", Alive: true},
		{Name: "Mid", Alive: true},
	})

	first := live.Project(linkAll)
	for i := 0; i < 5; i++ {
		if again := live.Project(linkAll); !reflect.DeepEqual(first, again) {
			t.Fatalf("projection changed between calls:\n%+v\n%+v", first, again)
		}
	}

	for i := 1; i < len(first.Players); i++ {
		if first.Players[i-1].UserID > first.Players[i].UserID {
			t.Errorf("the projection is not ordered by user id: %+v", first.Players)
		}
	}
}

func TestAFreshSessionProjectsNothing(t *testing.T) {
	state := NewLive().Project(linkAll)

	if len(state.Players) != 0 {
		t.Errorf("a session with no snapshot has no players, got %+v", state.Players)
	}
}
