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

**That the two halves connect over a real network.** `pkg/transport` and
`bot/recovery_test.go` open real WebSockets, but on localhost, in one process,
with no TLS, no proxy and no home router in between.

**That the installer installs.** It is built by the release workflow and has
never been run by anything here.

## Manual smoke test

Run this against a real Discord server and a real game before calling a release
good. It takes one round.

**Setup**

1. Invite the bot, run `/au setup channels`, then `/au doctor`. Every line
   should be green; fix anything that is not before going on.
2. `/au capture pair`, type the code into the capture app, `/au doctor` again.
   Capture and heartbeat should now be green.
3. `/au link` each player to their in-game name.

**One round**

4. Everyone in the main channel. Start a game. The living should go muted and
   deafened as tasks begin.
5. Have somebody killed. **Watch the channel list**: nothing should move, and
   the victim should be muted. This is the check that a screenshot cannot fake
   and the one worth doing carefully.
6. Call a meeting. The victim should move to the ghost channel and be able to
   talk; the living should be able to hear each other and not the ghost.
7. Back to tasks. The ghost stays in the ghost channel; the living go silent
   again.
8. Drag a living player into the ghost channel. They should be pulled back.
9. End the round. Everyone should be back in main, unmuted.

**Failure handling**

10. Kill the capture app mid-round. Within the configured timeout everybody
    should be released and the control channel should say so. This is the one
    that matters most: getting it wrong leaves a room full of people unable to
    speak.
11. Start capture again. The session should resume on its own.
12. Restart the bot mid-round. Capture reconnects, sends a snapshot, and voice
    should match the game again within a few seconds.

**Leave no trace**

13. `/au capture revoke`, then confirm the capture app can no longer connect.

Record the result in the release notes. A release that has not been through this
has been tested against a simulation of Discord, which is not the same as
Discord.
