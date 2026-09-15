// Package protocol defines the contract between AUVC capture and the bot.
//
// The contract is deliberately transport-independent: it says what a message
// looks like and which order messages may arrive in, and nothing about sockets.
// Phase 13 puts it on a WebSocket; the rules here are unchanged by that, and
// they can be tested without opening one.
//
// Messages are flat JSON envelopes, as specified in docs/requirements.md:
//
//	{"protocol": 1, "type": "snapshot", "session": "...", "seq": 123, ...}
//
// The C# side in capture/AUVC.Protocol mirrors these definitions, and both
// implementations are tested against the shared fixtures in protocol/fixtures,
// so the two cannot drift apart without a test failing.
package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Version is the protocol version this build speaks. A capture reporting any
// other version is refused with an understandable error rather than being
// allowed to send messages that mean something different here.
const Version = 1

// ErrMalformed is returned by Decode for anything it cannot understand.
var ErrMalformed = errors.New("malformed protocol message")

// Type names a message. The wire values are lower_snake_case and are part of
// the contract, so they are written out rather than derived.
type Type string

const (
	TypeHello            Type = "hello"
	TypeAuthentication   Type = "authentication"
	TypeHeartbeat        Type = "heartbeat"
	TypeSnapshot         Type = "snapshot"
	TypeGameStateChanged Type = "game_state_changed"
	TypePlayerJoined     Type = "player_joined"
	TypePlayerLeft       Type = "player_left"
	TypePlayerChanged    Type = "player_changed"
	TypePlayerDied       Type = "player_died"
	TypeGameEnded        Type = "game_ended"
	TypeError            Type = "error"
)

// Types lists every message type in the contract, in handshake-then-events
// order. Tests walk it so a new type cannot be added without being considered
// everywhere.
var Types = []Type{
	TypeHello,
	TypeAuthentication,
	TypeHeartbeat,
	TypeSnapshot,
	TypeGameStateChanged,
	TypePlayerJoined,
	TypePlayerLeft,
	TypePlayerChanged,
	TypePlayerDied,
	TypeGameEnded,
	TypeError,
}

// Phase is the game phase as it travels on the wire. These are the protocol's
// own names, kept separate from the bot's internal game.Phase so that renaming
// one does not silently change the other.
type Phase string

const (
	PhaseLobby      Phase = "lobby"
	PhaseTasks      Phase = "tasks"
	PhaseDiscussion Phase = "discussion"
	PhaseMenu       Phase = "menu"
	PhaseEnded      Phase = "ended"
)

// Phases lists every valid phase value.
var Phases = []Phase{PhaseLobby, PhaseTasks, PhaseDiscussion, PhaseMenu, PhaseEnded}

// Valid reports whether the phase is one the contract defines.
func (p Phase) Valid() bool {
	for _, known := range Phases {
		if p == known {
			return true
		}
	}
	return false
}

// Player is one player as capture sees them in the game.
//
// The name is the in-game name, which is what links a player to a Discord user.
// There is no Discord identity here on purpose: capture reads the game and must
// not need to know anything about Discord.
type Player struct {
	Name         string `json:"name"`
	Color        int    `json:"color"`
	Dead         bool   `json:"dead"`
	Disconnected bool   `json:"disconnected"`
}

// Lobby is the lobby capture is in: its code and its map. It travels on a
// snapshot and on a phase change, and is left out while capture knows neither.
type Lobby struct {
	// Code is the lobby code, or six asterisks when the host hides it.
	Code string `json:"code"`
	// Map is one of Maps, or a map this build does not know yet.
	Map string `json:"map"`
}

// Map names as they travel on the wire. A map this build does not know is
// ignored rather than refused, so a newer capture keeps working with an older
// bot.
const (
	MapTheSkeld = "the_skeld"
	MapMiraHQ   = "mira_hq"
	MapPolus    = "polus"
	MapDleks    = "dleks"
	MapAirship  = "airship"
	MapFungle   = "fungle"
)

// Maps lists every map name the contract defines.
var Maps = []string{MapTheSkeld, MapMiraHQ, MapPolus, MapDleks, MapAirship, MapFungle}

// IsLobbyCode reports whether a code is one the game uses: four or six capital
// letters, or six asterisks when the host hides it.
func IsLobbyCode(code string) bool {
	if code == "******" {
		return true
	}
	if len(code) != 4 && len(code) != 6 {
		return false
	}
	for _, letter := range code {
		if letter < 'A' || letter > 'Z' {
			return false
		}
	}
	return true
}

// Header is carried by every message.
//
// Session travels on every message rather than only on hello. That makes the
// rules checkable from the messages alone, with no reference to a connection,
// which is what keeps this package transport-independent and lets a reconnect
// be tested by replaying messages.
type Header struct {
	Protocol int    `json:"protocol"`
	Type     Type   `json:"type"`
	Session  string `json:"session"`
	Seq      uint64 `json:"seq"`
}

func (h Header) envelope() Header { return h }

// Message is any protocol message. The interface is deliberately tiny: it
// exists so the receiver can read the envelope of a message without being
// taught every payload.
type Message interface {
	envelope() Header
}

// Envelope returns the header every message carries.
func Envelope(message Message) Header { return message.envelope() }

// Hello opens a session. It is always the first message.
type Hello struct {
	Header
	// Capture is the capture build, for diagnostics and /au version.
	Capture string `json:"capture"`
}

// Authentication presents the long-term credential issued by /au capture pair.
// The credential is a secret and must never reach a log.
type Authentication struct {
	Header
	Credential string `json:"credential"`
}

// Heartbeat is capture reporting that it is alive and still reading the game.
type Heartbeat struct {
	Header
}

// Snapshot is the complete state of the round. Capture sends one after every
// reconnect, because incremental events alone cannot rebuild what was missed.
type Snapshot struct {
	Header
	Phase   Phase    `json:"phase"`
	Players []Player `json:"players"`
	Lobby   *Lobby   `json:"lobby,omitempty"`
}

// GameStateChanged reports a phase transition.
type GameStateChanged struct {
	Header
	Phase Phase `json:"phase"`
	// Lobby comes along once capture has read it. The game reads it only after
	// the phase has changed, so joining a lobby arrives as a change into the
	// phase capture is already in.
	Lobby *Lobby `json:"lobby,omitempty"`
}

// PlayerJoined reports a player entering the lobby.
type PlayerJoined struct {
	Header
	Player Player `json:"player"`
}

// PlayerLeft reports a player leaving.
type PlayerLeft struct {
	Header
	Player Player `json:"player"`
}

// PlayerChanged reports a change to a player that is neither death nor
// departure, such as a colour or name change.
type PlayerChanged struct {
	Header
	Player Player `json:"player"`
}

// PlayerDied reports a death. It is separate from PlayerChanged because it is
// the event the voice policy reacts to.
type PlayerDied struct {
	Header
	Player Player `json:"player"`
}

// GameEnded reports that the round is over.
type GameEnded struct {
	Header
}

// Error reports a refusal. Code is stable and machine-readable; Message is
// written for a person reading a log or a Discord warning.
type Error struct {
	Header
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// Error codes. They are part of the contract, so both sides can react to them
// without parsing prose.
const (
	// CodeIncompatibleProtocol means the two builds do not speak the same
	// protocol version. Only a matching capture build fixes it.
	CodeIncompatibleProtocol = "incompatible_protocol"
	// CodeExpectedHello means a session was used before it was opened.
	CodeExpectedHello = "expected_hello"
	// CodeUnauthenticated means messages arrived before authentication.
	CodeUnauthenticated = "unauthenticated"
	// CodeSnapshotRequired means the receiver cannot trust its own picture of
	// the round and needs a complete snapshot before any further events.
	CodeSnapshotRequired = "snapshot_required"
	// CodeMalformed means the message could not be understood at all.
	CodeMalformed = "malformed"
)

// NewError builds an error message addressed to the session it refuses.
func NewError(session string, seq uint64, code, message string) *Error {
	return &Error{
		Header:  Header{Protocol: Version, Type: TypeError, Session: session, Seq: seq},
		Code:    code,
		Message: message,
	}
}

// Decode turns one JSON message into its typed form.
//
// The envelope is read first so that a message of an unknown type produces a
// clear error instead of a half-decoded struct.
func Decode(data []byte) (Message, error) {
	var header Header
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrMalformed, err)
	}

	into := func(message Message) (Message, error) {
		if err := json.Unmarshal(data, message); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrMalformed, err)
		}
		return message, nil
	}

	switch header.Type {
	case TypeHello:
		return into(&Hello{})
	case TypeAuthentication:
		return into(&Authentication{})
	case TypeHeartbeat:
		return into(&Heartbeat{})
	case TypeSnapshot:
		return into(&Snapshot{})
	case TypeGameStateChanged:
		return into(&GameStateChanged{})
	case TypePlayerJoined:
		return into(&PlayerJoined{})
	case TypePlayerLeft:
		return into(&PlayerLeft{})
	case TypePlayerChanged:
		return into(&PlayerChanged{})
	case TypePlayerDied:
		return into(&PlayerDied{})
	case TypeGameEnded:
		return into(&GameEnded{})
	case TypeError:
		return into(&Error{})
	case "":
		return nil, fmt.Errorf("%w: message has no type", ErrMalformed)
	default:
		return nil, fmt.Errorf("%w: unknown message type %q", ErrMalformed, header.Type)
	}
}

// Encode writes a message as JSON.
func Encode(message Message) ([]byte, error) {
	return json.Marshal(message)
}
