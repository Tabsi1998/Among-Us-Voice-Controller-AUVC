# AUVC capture-to-bot protocol

Version 1.

This is the contract between AUVC capture and the AUVC bot. It is deliberately
transport-independent: it says what a message looks like and which order
messages may arrive in, and nothing about sockets. Phase 13 puts it on a
WebSocket without changing anything here.

Two implementations exist and neither is the reference:

- Go, in [bot/pkg/protocol](../bot/pkg/protocol) — the receiving side, including
  the rules below.
- C#, in [capture/AUVC.Protocol](../capture/AUVC.Protocol) — the sending side,
  plus the checks capture runs on its own messages before sending them.

Both are tested against the fixtures in [fixtures](fixtures), so the two cannot
drift apart without a test failing.

[fixtures/rounds](fixtures/rounds) holds whole rounds, one message per line, from
the snapshot after the handshake onwards. The C# tests feed the memory reader's
events for such a round through capture and compare every message capture sends
with the recording; the Go tests play the recording into the bot over a real
WebSocket and check at each step whom the voice policy mutes, deafens and moves.
`fifteen_players.jsonl` is a full lobby with a kill, a player who quits, an
exile reported twice as the reader reports it, a second kill, a dropped
connection and the end of the round.

## Envelope

Every message is a flat JSON object carrying the same four fields:

```json
{
  "protocol": 1,
  "type": "snapshot",
  "session": "7f3a1c9e-2b4d-4a6f-8e1b-5c9d0a3f7e21",
  "seq": 3,
  "phase": "tasks",
  "players": []
}
```

| Field | Meaning |
| --- | --- |
| `protocol` | Protocol version. A mismatch is refused with `incompatible_protocol`. |
| `type` | Message type, from the table below. |
| `session` | The capture session. A reconnect uses a new value. |
| `seq` | Message number within the session, starting at 1. |

The session travels on every message rather than only on `hello`. That is what
lets the bot check these rules from the messages alone, with no reference to a
connection, and it is what makes a reconnect testable by replaying messages.

## Message types

| Type | Carries | Meaning |
| --- | --- | --- |
| `hello` | `capture` | Opens a session and declares the capture build. |
| `authentication` | `credential` | Presents the credential from `/au capture pair`. |
| `heartbeat` | — | Capture is alive and still reading the game. |
| `snapshot` | `phase`, `players` | The complete state of the round. |
| `game_state_changed` | `phase` | A phase transition. |
| `player_joined` | `player` | A player entered the lobby. |
| `player_left` | `player` | A player left. |
| `player_changed` | `player` | A change that is neither death nor departure. |
| `player_died` | `player` | A death. The event the voice policy reacts to. |
| `game_ended` | — | The round is over. |
| `error` | `code`, `message` | A refusal, in both directions. |

`phase` is one of `lobby`, `tasks`, `discussion`, `menu`, `ended`. Meetings and
voting are both `discussion`, which is what the game state reports.

A `player` is `{"name", "color", "dead", "disconnected"}`. The in-game name is
what links a player to a Discord account. There is no Discord identity in this
protocol on purpose: capture reads the game and must not need to know anything
about Discord.

## Order

```text
hello -> authentication -> snapshot -> events
```

Each step earns its place:

- `hello` carries the protocol version, so an incompatible capture is turned
  away with an explanation instead of sending messages that mean something else
  on the other side.
- `authentication` comes before any game data, so an unpaired capture cannot
  drive the voice of a guild.
- `snapshot` comes before any incremental event, because events alone cannot
  rebuild a round that was already running.

## Sequence, duplicates and gaps

`seq` increases by one per message within a session.

- **Duplicate or late** (`seq` at or below the last accepted): ignored silently.
  Retransmission is ordinary on a reconnecting transport, and answering it with
  an error would turn a working recovery into a reported fault.
- **Gap** (`seq` above the next expected): events were lost. Applying the ones
  that did arrive would leave the bot confidently wrong about who is alive, so
  it refuses with `snapshot_required` and accepts nothing further until a
  complete `snapshot` arrives.
- **`snapshot` across a gap**: accepted. It is the message that repairs a broken
  stream, so it is not refused for arriving across the gap it exists to close.
- **`heartbeat` across a gap**: accepted. It says nothing about the round, and
  refusing it would make a capture that is plainly alive look dead, which is
  what triggers the fail-open timeout.

## Reconnecting

A reconnect is a new `session`, and everything learned about the previous one is
dropped, including the snapshot. The new session sends the full handshake again
and a complete `snapshot` before any event is accepted. Messages carrying the
replaced session are refused with `expected_hello`.

A repeated `hello` for the session that is already running is a retransmission
and does not reset it.

## Error codes

| Code | Meaning |
| --- | --- |
| `incompatible_protocol` | The builds do not speak the same version. Only a matching capture build fixes it. |
| `expected_hello` | The session was used before it was opened. |
| `unauthenticated` | Game data arrived before authentication, the credential was refused, or it was revoked while the session was open. The server closes the connection after sending it; only pairing again fixes a revoked credential. |
| `snapshot_required` | The receiver cannot trust its picture of the round and needs a complete snapshot. |
| `malformed` | The message could not be understood. |

An `error` from capture is accepted whatever state the session is in. Refusing
to hear about a problem is how a problem stays invisible.

## Fixtures

[fixtures/messages](fixtures/messages) holds one valid example per message type;
[fixtures/invalid](fixtures/invalid) holds documents both implementations must
refuse. The tests on each side assert that every type has a fixture, that each
one decodes to its type, that re-encoding reproduces the same document, and that
every invalid fixture is rejected either while decoding or while validating.

Adding a message type therefore means adding a fixture, or both test suites
fail — which is the point.

## Carrying it

The bot exposes two endpoints, off unless `AUVC_CAPTURE_ADDR` names an address:

| Endpoint | Purpose |
| --- | --- |
| `POST /capture/pair` | Exchange a typed pairing code for a credential. |
| `GET /capture/link` | The WebSocket that carries the messages above. |

Pairing sits outside the WebSocket on purpose. It happens once, before there is
a session to speak in, and folding it into the protocol would have meant a
message that exists only for the first connection of a capture install's life.
The request carries the code and nothing else: capture never sees a guild id,
because asking a player to copy a Discord snowflake out of a developer menu is
not a setup flow anyone finishes.

The credential is returned by that one response and never again. Only its hash
is stored.

Authentication failures close the connection rather than answering and carrying
on. The receiver has already recorded that an authentication message arrived in
the right order by the time the credential is checked, so a session that stayed
open would believe it was authenticated when it was not.

A refusal ends the connection when the session cannot recover from it — an
incompatible protocol, a session that never said hello, a failed credential, a
frame that is not a message. `snapshot_required` does not: the session fixes it
by sending a snapshot, and closing the connection would turn an ordinary
recovery into a reconnect loop.

Only requests without an `Origin` header are upgraded. Capture is a desktop
application and sends none; a browser always does, so a web page cannot reach
the handshake at all.

### The capture side

`capture/AUVC.Transport` is the other end. `CaptureSession` builds the messages
and numbers them. `CaptureLink` keeps a connection open over an
`IMessageChannel`, which `WebSocketMessageChannel` implements. Each connection
sends hello, the credential and a snapshot of the round the link has been
recording, then events, and a heartbeat every five seconds. The link answers
`snapshot_required` with a snapshot on the same connection, reconnects by itself
after a drop, and stops retrying after `unauthenticated` or
`incompatible_protocol`, which another attempt cannot fix. Messages are built
and queued under one lock, because building one assigns its sequence number. The
channel is an interface for the same reason the protocol has no socket in it:
the rules can then be tested without one.

`PairingClient` redeems a code against `/capture/pair`. `DpapiCredentialStore`
keeps the result encrypted with the Windows Data Protection API, tied to the
current user account, so the file is unreadable to another account and to anyone
who copies it elsewhere. That is not protection against the user's own account
being compromised — nothing stored on a machine is — which is what
`/au capture revoke` exists for.

The bot does not acknowledge a successful authentication. It answers only to
refuse, and then closes. A connection still open after the handshake is an
authenticated one.

### TLS

`AUVC_CAPTURE_TLS_CERT` and `AUVC_CAPTURE_TLS_KEY` make the listener terminate
TLS itself. Without them it serves plain HTTP and logs a warning on every start,
because terminating TLS at a reverse proxy is a legitimate and common way to run
this — and a plain listener that is *not* behind one carries credentials in the
clear with nothing else to say so.

## Changing this protocol

Bump `protocol` and record the change here. The version exists so that a capture
and a bot from different releases refuse each other clearly at the handshake
instead of misreading each other mid-round.
