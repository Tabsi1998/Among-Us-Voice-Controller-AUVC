# Acceptance

Where each required scenario is proved, and — just as important — what the
automated tests do not prove.

## What the automated tests actually run

They drive a real lobby through the real pieces. Protocol messages reach the
real session, the real player links resolve them, the real voice policy decides
and the real reconciler works out the minimal set of changes. The simulated
Discord then applies those changes, so the observed state moves the way a
server's would and the next decision is made against it.

What is absent is the Discord API call itself and the memory reading in capture.
That absence is the whole point of this page.

## The required scenarios

| # | Scenario | Where |
| --- | --- | --- |
| 1 | Lobby: everyone open in main | `bot/acceptance_test.go` |
| 2 | Tasks: living muted and deafened | `bot/acceptance_test.go` |
| 3 | Death during tasks is not announced by a move | `bot/acceptance_test.go`, `pkg/voice/secrecy_test.go` |
| 4 | The meeting sends the ghost to the ghost channel | `bot/acceptance_test.go` |
| 5 | Two ghosts share the ghost channel | `bot/acceptance_test.go` |
| 6 | Meeting: living unmuted and undeafened | `bot/acceptance_test.go` |
| 7 | Ghosts keep the ghost channel through the next tasks | `bot/acceptance_test.go` |
| 8 | Ghosts can talk during the meeting | `bot/acceptance_test.go` |
| 9 | Tasks resume: living silenced again | `bot/acceptance_test.go` |
| 10 | A ghost who walks into main is sent back | `bot/acceptance_test.go` |
| 11 | A living player who walks into ghost is sent back | `bot/acceptance_test.go` |
| 12 | Game ends: everyone back in main, open | `bot/acceptance_test.go` |
| 13 | Capture timeout: fail-safe | `bot/capture_watchdog_test.go`, `bot/recovery_test.go` |
| 14 | Capture reconnect: full recovery | `bot/recovery_test.go` (real WebSocket) |
| 15 | Bot restart: settings and links survive | `bot/acceptance_test.go` |
| 16 | Bot restart mid-match: snapshot restores the round | `bot/recovery_test.go` |
| 17 | Deleted channel: the doctor reports it | `bot/doctor_test.go`, `bot/acceptance_test.go` |
| 18 | Missing Move Members: the doctor reports it | `pkg/permission`, `bot/doctor_test.go` |
| 19 | Unlinked users are untouched | `bot/acceptance_test.go`, `pkg/voice/policy_test.go` |

Beyond the list, the suites also cover duplicate and stale events, a reconnect
that never sends a snapshot, protocol incompatibility, credential expiry, replay
and revocation.

## What none of this proves

**That Discord does what we asked.** Every test stops at the point where the bot
decides what to send. Whether Discord actually moves a member, whether a
permission overwrite resolves the way `UserChannelPermissions` said, and whether
a move lands before an undeafen on a real connection are answered only by a real
server.

**That capture reads the game correctly.** The offsets, the process handle and
the memory layout are exercised against fixtures, not against Among Us. A game
update that moves an offset breaks nothing in this repository's tests and
everything in practice.

**That the two halves connect at all.** `pkg/transport` and
`bot/recovery_test.go` open real WebSockets, but with a Go client, on localhost,
in one process, with no TLS, no proxy and no home router in between. The capture
side's `CaptureLinkTests` run against a scripted channel. Since
[#117](https://github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/pull/117)
both sides are held to one recorded round,
`protocol/fixtures/rounds/fifteen_players.jsonl`: `CaptureLinkTests` checks that
capture sends exactly those messages, and `bot/recorded_round_test.go` plays them
through the real WebSocket server and the bot. Neither side can read that round
differently without its test failing. The C# client and the Go server have
still never spoken to each other in a test.

**That the app's windows work.** What they decide is tested:
- **Status line:** which status the top line shows (`AppStatusTests`).
- **Bot checks:** how the bot's checks read (`DoctorLineTests`).
- **Pairing:** what it sends and how it fails (`PairingClientTests`).
- **Buttons:** every button has a name a screen reader can read (`XamlAccessibilityTests`).

Whether the setup window, the pairing and the connection indicator actually show
all this is not tested.

**That the installer installs.** `scripts/local_release.py` builds it, and so
does the release workflow for finished releases, but nothing here has ever run
it.

## Manual smoke test

Run this against a real Discord server and a real game before calling a release
good. It takes one round. It is written for the usual case, one PC with the bot
inside the app; where a bot on another computer behaves differently, the step
says so.

**Setup**

1. Set AUVC up with its setup window: token, invite, server and channels. Its
   last step lists the bot's checks, and none should show ❌. `/au doctor` in
   Discord should say the same.
2. **Bot on another computer:** set that bot up with `/au setup channels`, then
   run `/au capture pair` and type the code into the app behind its pairing
   button. `/au doctor` should show *Capture* and *Heartbeat* green.
3. Open a lobby.
   - **Text channel:** the crewmate message should appear there.
   - **`/au doctor`:** it should show *Crewmate menu*, *Capture* and
     *Heartbeat* green.
   - **Linking:** every player picks their crewmate in the crewmate message;
     one player uses `/au link` without a name instead.
   - **Status line:** the app should say *Ready* and count the linked players.

**One round**

4. Everyone in the main channel. Start a game. The living should go muted and
   deafened as tasks begin.
5. Have somebody killed. **Watch the channel list and the crewmate message**:
   nothing should move or change, and the victim should be muted. This is the check that a screenshot cannot fake
   and the one worth doing carefully.
6. Call a meeting. The victim should move to the ghost channel and be able to
   talk; the living should be able to hear each other and not the ghost.
7. Back to tasks. The ghost stays in the ghost channel; the living go silent
   again.
8. Drag a living player into the ghost channel. They should be pulled back.
9. End the round. Everyone should be back in main, unmuted.

**Failure handling**

10. End AUVC mid-round, for example in Task Manager. This is the one that
    matters most: getting it wrong leaves a room full of people unable to speak.
    - **Bot inside the app:** the bot ends with it, so nobody is released at
      that moment. Start AUVC again. The bot should release everyone it had
      muted, deafened or moved into the ghost channel, and nobody else.
    - **Bot on another computer:** within the configured timeout everybody
      should be released, and the text channel should say so.
11. Carry on playing. AUVC should follow the game again on its own. With a bot
    on another computer the text channel should say the app is back.
12. Restart the bot mid-round: **Bot → Status → Restart the bot**, or restart
    the bot on the other computer. The app reconnects and sends a snapshot.
    Voice should match the game again within a few seconds.

**Leave no trace**

13. With the app connected, run `/au capture revoke`. The app should be
    disconnected at once.
    - **Bot inside the app:** the app takes a new credential from its own bot,
      once per start, and connects again. Revoke a second time. Now the app
      should stay disconnected, and its status line should say *The bot
      refused this app*.
    - **Bot on another computer:** the app should say its access was revoked.
      It should not connect again until it is paired anew.

Record the result in the release notes. A release that has not been through this
has been tested against a simulation of Discord, which is not the same as
Discord.
