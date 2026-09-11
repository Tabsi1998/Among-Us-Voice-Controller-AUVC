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
| `unauthenticated` | Game data arrived before authentication. |
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

## Changing this protocol

Bump `protocol` and record the change here. The version exists so that a capture
and a bot from different releases refuse each other clearly at the handshake
instead of misreading each other mid-round.

## Not in this phase

Credential issuance (`/au capture pair`), the WebSocket transport and the
heartbeat timeout behaviour are separate phases. This document defines only the
messages and their ordering. See [docs/requirements.md](../docs/requirements.md)
for the full contract and [docs/roadmap.md](../docs/roadmap.md) for the order.
