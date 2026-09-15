package session

import (
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
)

func TestALobbyIsAChangeOnlyWhenItDiffers(t *testing.T) {
	live := NewLive()
	skeld := Lobby{Code: "ABCDEF", Map: "the_skeld"}

	if live.Lobby() != (Lobby{}) {
		t.Fatalf("a new session already has a lobby: %+v", live.Lobby())
	}
	if !live.SetLobby(skeld) {
		t.Error("the first lobby counted as no change")
	}
	if live.SetLobby(skeld) {
		t.Error("the same lobby again counted as a change")
	}
	if !live.SetLobby(Lobby{Code: "ABCDEF", Map: "polus"}) {
		t.Error("another map counted as no change")
	}
}

// A snapshot replaces the players, not what capture said about the lobby: the
// lobby arrives with the same message and is set separately.
func TestASnapshotLeavesTheLobbyToItsOwnField(t *testing.T) {
	live := NewLive()
	live.SetLobby(Lobby{Code: "ABCD", Map: "fungle"})

	live.Reset(game.TASKS, []GamePlayer{{Name: "Red"}})

	if got := live.Lobby(); got != (Lobby{Code: "ABCD", Map: "fungle"}) {
		t.Errorf("the lobby is %+v after a snapshot", got)
	}
}
