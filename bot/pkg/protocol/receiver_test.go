package protocol

import "testing"

const session = "session-a"

func header(messageType Type, seq uint64) Header {
	return Header{Protocol: Version, Type: messageType, Session: session, Seq: seq}
}

func hello(seq uint64) *Hello {
	return &Hello{Header: header(TypeHello, seq), Capture: "1.0.0"}
}

func auth(seq uint64) *Authentication {
	return &Authentication{Header: header(TypeAuthentication, seq), Credential: "secret"}
}

func snapshot(seq uint64) *Snapshot {
	return &Snapshot{
		Header:  header(TypeSnapshot, seq),
		Phase:   PhaseTasks,
		Players: []Player{{Name: "Red"}},
	}
}

func died(seq uint64) *PlayerDied {
	return &PlayerDied{Header: header(TypePlayerDied, seq), Player: Player{Name: "Red", Dead: true}}
}

func heartbeat(seq uint64) *Heartbeat {
	return &Heartbeat{Header: header(TypeHeartbeat, seq)}
}

// mustAccept drives the receiver through messages that are all expected to be
// processed, so a test can get to the state it actually wants to examine.
func mustAccept(t *testing.T, receiver *Receiver, messages ...Message) {
	t.Helper()

	for _, message := range messages {
		action, refusal := receiver.Accept(message)
		if refusal != nil {
			t.Fatalf("%s was refused: %v", Envelope(message).Type, refusal)
		}
		if action != ActionProcess {
			t.Fatalf("%s was not processed: %s", Envelope(message).Type, action)
		}
	}
}

func refusal(t *testing.T, receiver *Receiver, message Message) *Error {
	t.Helper()

	action, refused := receiver.Accept(message)
	if refused == nil {
		t.Fatalf("%s was accepted, expected a refusal", Envelope(message).Type)
	}
	if action == ActionProcess {
		t.Fatalf("%s was refused and still marked for processing", Envelope(message).Type)
	}
	return refused
}

func TestAFullHandshakeAndEventStreamIsAccepted(t *testing.T) {
	var receiver Receiver

	mustAccept(t, &receiver, hello(1), auth(2), snapshot(3), heartbeat(4), died(5))

	if !receiver.Ready() {
		t.Error("the session should be ready after hello, authentication and a snapshot")
	}
	if receiver.Session() != session {
		t.Errorf("tracking session %q, want %q", receiver.Session(), session)
	}
	if receiver.LastSeq() != 5 {
		t.Errorf("last sequence is %d, want 5", receiver.LastSeq())
	}
}

// A session that was never opened has no protocol version, no credential and no
// snapshot behind it. Accepting its events would mean acting on a stream nobody
// vouched for.
func TestEventsBeforeHelloAreRefused(t *testing.T) {
	var receiver Receiver

	if got := refusal(t, &receiver, died(1)).Code; got != CodeExpectedHello {
		t.Errorf("got code %q, want %q", got, CodeExpectedHello)
	}
}

// The version mismatch has to be named, because the only thing that fixes it is
// installing a matching capture build and the user has to be told that.
func TestAnIncompatibleProtocolIsRefusedWithAnExplanation(t *testing.T) {
	var receiver Receiver
	stranger := hello(1)
	stranger.Protocol = Version + 1

	refused := refusal(t, &receiver, stranger)
	if refused.Code != CodeIncompatibleProtocol {
		t.Errorf("got code %q, want %q", refused.Code, CodeIncompatibleProtocol)
	}
	if refused.Message == "" {
		t.Error("an incompatible protocol must be explained, not just coded")
	}
	if receiver.Ready() {
		t.Error("an incompatible capture must not open a session")
	}
}

func TestGameDataBeforeAuthenticationIsRefused(t *testing.T) {
	var receiver Receiver
	mustAccept(t, &receiver, hello(1))

	if got := refusal(t, &receiver, snapshot(2)).Code; got != CodeUnauthenticated {
		t.Errorf("got code %q, want %q", got, CodeUnauthenticated)
	}
}

// Incremental events cannot rebuild a round that was already running, so the
// receiver refuses to guess and asks for the whole picture.
func TestEventsBeforeASnapshotAreRefused(t *testing.T) {
	var receiver Receiver
	mustAccept(t, &receiver, hello(1), auth(2))

	if got := refusal(t, &receiver, died(3)).Code; got != CodeSnapshotRequired {
		t.Errorf("got code %q, want %q", got, CodeSnapshotRequired)
	}
}

// A retransmission is ordinary on a reconnecting transport. Answering it with
// an error would turn a working recovery into a reported fault.
func TestADuplicateIsIgnoredRatherThanRefused(t *testing.T) {
	var receiver Receiver
	mustAccept(t, &receiver, hello(1), auth(2), snapshot(3), died(4))

	action, refused := receiver.Accept(died(4))
	if refused != nil {
		t.Fatalf("a duplicate must not be an error: %v", refused)
	}
	if action != ActionIgnore {
		t.Errorf("a duplicate must be ignored, got %s", action)
	}
	if receiver.LastSeq() != 4 {
		t.Errorf("a duplicate moved the sequence to %d", receiver.LastSeq())
	}
}

func TestAStaleEventIsIgnored(t *testing.T) {
	var receiver Receiver
	mustAccept(t, &receiver, hello(1), auth(2), snapshot(3), died(4), heartbeat(5))

	action, refused := receiver.Accept(died(4))
	if refused != nil || action != ActionIgnore {
		t.Errorf("a stale event must be ignored quietly, got %s / %v", action, refused)
	}
}

// A gap means events were lost. Applying the ones that did arrive would leave
// the bot confidently wrong about who is alive.
func TestAGapDemandsANewSnapshot(t *testing.T) {
	var receiver Receiver
	mustAccept(t, &receiver, hello(1), auth(2), snapshot(3))

	refused := refusal(t, &receiver, died(9))
	if refused.Code != CodeSnapshotRequired {
		t.Errorf("got code %q, want %q", refused.Code, CodeSnapshotRequired)
	}
	if receiver.Ready() {
		t.Error("the receiver must not consider itself ready after losing events")
	}

	// Nothing gets through until the whole picture arrives again.
	if got := refusal(t, &receiver, died(10)).Code; got != CodeSnapshotRequired {
		t.Errorf("events after a gap must stay refused, got %q", got)
	}

	mustAccept(t, &receiver, snapshot(11), died(12))
	if !receiver.Ready() {
		t.Error("a snapshot must repair the session")
	}
}

// A snapshot is the message that repairs a broken stream, so it must not be
// refused for arriving across the very gap it exists to close.
func TestASnapshotIsAcceptedAcrossAGap(t *testing.T) {
	var receiver Receiver
	mustAccept(t, &receiver, hello(1), auth(2), snapshot(3))

	mustAccept(t, &receiver, snapshot(50))
	if receiver.LastSeq() != 50 {
		t.Errorf("last sequence is %d, want 50", receiver.LastSeq())
	}
}

// Refusing a heartbeat while waiting for a snapshot would make a capture that
// is plainly alive look dead, which is what triggers the fail-open timeout.
func TestHeartbeatsSurviveAGap(t *testing.T) {
	var receiver Receiver
	mustAccept(t, &receiver, hello(1), auth(2), snapshot(3))
	refusal(t, &receiver, died(9))

	mustAccept(t, &receiver, heartbeat(10))
}

// Reconnecting is a new session, and the bot knows nothing about what happened
// while it was gone. The requirements are explicit that a complete snapshot
// follows every reconnect.
func TestAReconnectRequiresACompleteSnapshotAgain(t *testing.T) {
	var receiver Receiver
	mustAccept(t, &receiver, hello(1), auth(2), snapshot(3), died(4))

	reconnect := &Hello{
		Header:  Header{Protocol: Version, Type: TypeHello, Session: "session-b", Seq: 1},
		Capture: "1.0.0",
	}
	mustAccept(t, &receiver, reconnect)

	if receiver.Ready() {
		t.Error("a reconnected session must not inherit the old snapshot")
	}
	if receiver.Session() != "session-b" {
		t.Errorf("tracking session %q after the reconnect", receiver.Session())
	}

	authB := &Authentication{
		Header:     Header{Protocol: Version, Type: TypeAuthentication, Session: "session-b", Seq: 2},
		Credential: "secret",
	}
	mustAccept(t, &receiver, authB)

	deathB := &PlayerDied{
		Header: Header{Protocol: Version, Type: TypePlayerDied, Session: "session-b", Seq: 3},
		Player: Player{Name: "Red", Dead: true},
	}
	if got := refusal(t, &receiver, deathB).Code; got != CodeSnapshotRequired {
		t.Errorf("got code %q, want %q", got, CodeSnapshotRequired)
	}
}

// Messages from the session that was replaced are late by definition.
func TestMessagesFromAReplacedSessionAreRefused(t *testing.T) {
	var receiver Receiver
	mustAccept(t, &receiver, hello(1), auth(2), snapshot(3))

	reconnect := &Hello{
		Header:  Header{Protocol: Version, Type: TypeHello, Session: "session-b", Seq: 1},
		Capture: "1.0.0",
	}
	mustAccept(t, &receiver, reconnect)

	if got := refusal(t, &receiver, died(4)).Code; got != CodeExpectedHello {
		t.Errorf("got code %q, want %q", got, CodeExpectedHello)
	}
}

// A repeated hello for the running session is a retransmission, not a reason to
// throw away a session that is working.
func TestARepeatedHelloForTheSameSessionIsIgnored(t *testing.T) {
	var receiver Receiver
	mustAccept(t, &receiver, hello(1), auth(2), snapshot(3))

	action, refused := receiver.Accept(hello(1))
	if refused != nil || action != ActionIgnore {
		t.Fatalf("a repeated hello must be ignored, got %s / %v", action, refused)
	}
	if !receiver.Ready() {
		t.Error("a retransmitted hello must not reset a working session")
	}
}

// Refusing to hear about a problem is how a problem stays invisible.
func TestAnErrorFromCaptureIsAlwaysAccepted(t *testing.T) {
	var receiver Receiver

	report := &Error{
		Header:  header(TypeError, 1),
		Code:    "game_not_found",
		Message: "Among Us is not running",
	}
	mustAccept(t, &receiver, report)
}

func TestMalformedEnvelopesAreRefused(t *testing.T) {
	cases := map[string]Message{
		"no session":          &Heartbeat{Header: Header{Protocol: Version, Type: TypeHeartbeat, Seq: 1}},
		"zero sequence":       &Heartbeat{Header: Header{Protocol: Version, Type: TypeHeartbeat, Session: session}},
		"hello with no build": &Hello{Header: header(TypeHello, 1)},
		"unknown phase": &GameStateChanged{
			Header: header(TypeGameStateChanged, 1),
			Phase:  "voting",
		},
		"player with no name": &PlayerDied{Header: header(TypePlayerDied, 1)},
		"credential missing":  &Authentication{Header: header(TypeAuthentication, 1)},
	}

	for name, message := range cases {
		t.Run(name, func(t *testing.T) {
			var receiver Receiver
			if got := refusal(t, &receiver, message).Code; got != CodeMalformed {
				t.Errorf("got code %q, want %q", got, CodeMalformed)
			}
		})
	}
}

// The snapshot is what the whole round is rebuilt from, so a player without a
// name in it is worse than a missing snapshot: it looks complete and is not.
func TestASnapshotWithANamelessPlayerIsRefused(t *testing.T) {
	var receiver Receiver
	mustAccept(t, &receiver, hello(1), auth(2))

	broken := snapshot(3)
	broken.Players = append(broken.Players, Player{Color: 4})

	if got := refusal(t, &receiver, broken).Code; got != CodeMalformed {
		t.Errorf("got code %q, want %q", got, CodeMalformed)
	}
}

// Every phase the contract defines has to be accepted, or capture cannot report
// a state the bot already knows how to handle.
func TestEveryDefinedPhaseIsAccepted(t *testing.T) {
	for _, phase := range Phases {
		t.Run(string(phase), func(t *testing.T) {
			var receiver Receiver
			mustAccept(t, &receiver, hello(1), auth(2))

			first := snapshot(3)
			first.Phase = phase
			mustAccept(t, &receiver, first)
		})
	}
}
