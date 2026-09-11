package protocol

import "fmt"

// Action is what the caller should do with a message the receiver has checked.
type Action int

const (
	// ActionProcess means the message is valid and in order: apply it.
	ActionProcess Action = iota
	// ActionIgnore means the message is a duplicate or arrived late. It is not
	// an error and must not be answered; it simply already happened.
	ActionIgnore
)

func (a Action) String() string {
	if a == ActionIgnore {
		return "ignore"
	}
	return "process"
}

// Receiver enforces the order a capture session has to speak in.
//
// It is a pure state machine over messages, with no transport and no clock, so
// a reconnect, a duplicate and a lost event are all just message sequences a
// test can replay.
//
// The order is: hello, authentication, snapshot, then events. Each step exists
// for a reason the requirements give:
//
//   - hello carries the protocol version, so an incompatible capture is turned
//     away with an explanation instead of sending messages that mean something
//     else here.
//   - authentication comes before any game data, so an unpaired capture cannot
//     drive the voice of a guild.
//   - a snapshot comes before any incremental event, because events alone
//     cannot rebuild a round that was already running.
//
// The zero value is ready to use.
type Receiver struct {
	session       string
	authenticated bool
	haveSnapshot  bool
	lastSeq       uint64
	started       bool
}

// Session returns the session currently being tracked, empty before hello.
func (r *Receiver) Session() string { return r.session }

// Ready reports whether the receiver has a session it can apply events to.
func (r *Receiver) Ready() bool { return r.started && r.authenticated && r.haveSnapshot }

// LastSeq returns the sequence number of the last accepted message.
func (r *Receiver) LastSeq() uint64 { return r.lastSeq }

// Accept checks one message against the contract.
//
// A non-nil error is a refusal that should be sent back to capture; the message
// must not be applied. ActionIgnore means the message is a duplicate or stale
// and must be dropped silently.
func (r *Receiver) Accept(message Message) (Action, *Error) {
	envelope := Envelope(message)

	if envelope.Protocol != Version {
		return ActionIgnore, NewError(envelope.Session, envelope.Seq, CodeIncompatibleProtocol,
			fmt.Sprintf("this bot speaks protocol %d and capture speaks %d; install the matching capture build",
				Version, envelope.Protocol))
	}
	if envelope.Session == "" {
		return ActionIgnore, NewError("", envelope.Seq, CodeMalformed, "the message carries no session")
	}
	if envelope.Seq == 0 {
		return ActionIgnore, NewError(envelope.Session, 0, CodeMalformed, "sequence numbers start at 1")
	}
	if err := validatePayload(message); err != nil {
		return ActionIgnore, err
	}

	// An error from capture is a report, not a step in the handshake. It is
	// accepted whatever state the session is in, because refusing to hear about
	// a problem is how a problem stays invisible.
	if hello, isHello := message.(*Hello); isHello {
		return r.acceptHello(hello)
	}
	if _, isError := message.(*Error); isError {
		return ActionProcess, nil
	}

	if !r.started || envelope.Session != r.session {
		return ActionIgnore, NewError(envelope.Session, envelope.Seq, CodeExpectedHello,
			"this session has not said hello yet")
	}

	// Duplicates and late arrivals are ordinary on a reconnecting transport.
	// Answering them with an error would turn a retransmission into a fault.
	if envelope.Seq <= r.lastSeq {
		return ActionIgnore, nil
	}

	switch message.(type) {
	case *Authentication:
		r.authenticated = true
		r.lastSeq = envelope.Seq
		return ActionProcess, nil
	}

	if !r.authenticated {
		return ActionIgnore, NewError(envelope.Session, envelope.Seq, CodeUnauthenticated,
			"capture must authenticate before sending game data")
	}

	if _, isSnapshot := message.(*Snapshot); isSnapshot {
		// A snapshot is the one message that repairs a broken stream, so it is
		// accepted across a gap rather than being refused for causing one.
		r.haveSnapshot = true
		r.lastSeq = envelope.Seq
		return ActionProcess, nil
	}

	// A heartbeat says nothing about the round. It is liveness, and refusing it
	// while waiting for a snapshot would make a capture that is plainly alive
	// look dead, which is what triggers the fail-open timeout.
	if _, isHeartbeat := message.(*Heartbeat); isHeartbeat {
		r.lastSeq = envelope.Seq
		return ActionProcess, nil
	}

	if !r.haveSnapshot {
		return ActionIgnore, NewError(envelope.Session, envelope.Seq, CodeSnapshotRequired,
			"send a complete snapshot before any further events")
	}

	if envelope.Seq != r.lastSeq+1 {
		// Events are missing. Applying the ones that did arrive would leave the
		// bot confidently wrong about who is alive, which is worse than asking
		// for the whole picture again.
		r.haveSnapshot = false
		return ActionIgnore, NewError(envelope.Session, envelope.Seq, CodeSnapshotRequired,
			fmt.Sprintf("expected sequence %d but received %d; send a complete snapshot",
				r.lastSeq+1, envelope.Seq))
	}

	r.lastSeq = envelope.Seq
	return ActionProcess, nil
}

// acceptHello starts a session, or replaces the one that was running.
//
// A hello for a different session is a reconnect: everything learned about the
// old session is dropped, including the snapshot, so the new one has to send a
// complete picture before it can move anybody.
func (r *Receiver) acceptHello(hello *Hello) (Action, *Error) {
	envelope := hello.Header

	if r.started && envelope.Session == r.session {
		// A repeated hello for the running session is a retransmission.
		if envelope.Seq <= r.lastSeq {
			return ActionIgnore, nil
		}
	}

	r.session = envelope.Session
	r.authenticated = false
	r.haveSnapshot = false
	r.lastSeq = envelope.Seq
	r.started = true
	return ActionProcess, nil
}

// validatePayload rejects values that are inside the envelope but outside the
// contract, so a caller never has to guard against, say, an unknown phase.
func validatePayload(message Message) *Error {
	envelope := Envelope(message)

	refuse := func(reason string) *Error {
		return NewError(envelope.Session, envelope.Seq, CodeMalformed, reason)
	}

	switch typed := message.(type) {
	case *Hello:
		if typed.Capture == "" {
			return refuse("hello does not say which capture build it is")
		}
	case *Authentication:
		if typed.Credential == "" {
			return refuse("authentication carries no credential")
		}
	case *Snapshot:
		if !typed.Phase.Valid() {
			return refuse(fmt.Sprintf("unknown phase %q", typed.Phase))
		}
		for _, player := range typed.Players {
			if player.Name == "" {
				return refuse("a player in the snapshot has no name")
			}
		}
	case *GameStateChanged:
		if !typed.Phase.Valid() {
			return refuse(fmt.Sprintf("unknown phase %q", typed.Phase))
		}
	case *PlayerJoined:
		return validatePlayer(typed.Player, refuse)
	case *PlayerLeft:
		return validatePlayer(typed.Player, refuse)
	case *PlayerChanged:
		return validatePlayer(typed.Player, refuse)
	case *PlayerDied:
		return validatePlayer(typed.Player, refuse)
	case *Error:
		if typed.Code == "" {
			return refuse("error carries no code")
		}
	}
	return nil
}

// validatePlayer checks the player a single-player event carries. The name is
// what links a player to a Discord account, so an event without one cannot be
// acted on at all.
func validatePlayer(player Player, refuse func(string) *Error) *Error {
	if player.Name == "" {
		return refuse("the event carries a player with no name")
	}
	return nil
}
