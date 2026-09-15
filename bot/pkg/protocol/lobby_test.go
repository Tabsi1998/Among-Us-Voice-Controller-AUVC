package protocol

import "testing"

// The lobby code is posted in a Discord channel, so only a code the game uses
// gets through, on a snapshot and on a phase change alike.
func TestOnlyALobbyCodeTheGameUsesIsAccepted(t *testing.T) {
	header := func(kind Type) Header {
		return Header{Protocol: Version, Type: kind, Session: "s", Seq: 1}
	}

	for code, valid := range map[string]bool{
		"ABCD":      true,
		"ABCDEF":    true,
		"******":    true,
		"":          true,
		"abcd":      false,
		"ABCDE":     false,
		"AB CD":     false,
		"<b>hi</b>": false,
	} {
		lobby := &Lobby{Code: code, Map: MapPolus}
		for _, message := range []Message{
			&GameStateChanged{Header: header(TypeGameStateChanged), Phase: PhaseLobby, Lobby: lobby},
			&Snapshot{Header: header(TypeSnapshot), Phase: PhaseLobby, Lobby: lobby},
		} {
			if refused := validatePayload(message) != nil; refused == valid {
				t.Errorf("%s with code %q: refused %t", Envelope(message).Type, code, refused)
			}
		}
	}
}

// A map this build does not know is left for the bot to ignore, so a newer
// capture keeps working with an older bot.
func TestAnUnknownMapIsNotRefused(t *testing.T) {
	message := &GameStateChanged{
		Header: Header{Protocol: Version, Type: TypeGameStateChanged, Session: "s", Seq: 1},
		Phase:  PhaseLobby,
		Lobby:  &Lobby{Code: "ABCDEF", Map: "a_map_from_the_future"},
	}

	if refusal := validatePayload(message); refusal != nil {
		t.Errorf("refused: %s", refusal.Message)
	}
}
