package bot

import (
	"testing"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
)

const (
	guild        = "guild-1"
	shortTimeout = 10 * time.Second
)

func fixedTimeout(string) time.Duration { return shortTimeout }

func TestASessionThatKeepsTalkingIsNeverStalled(t *testing.T) {
	sessions := NewCaptureSessions()
	sessions.SetMode(guild, Running)

	now := time.Unix(1_700_000_000, 0)
	sessions.seen(guild, now)

	if stalled := sessions.Stalled(now.Add(shortTimeout-time.Second), fixedTimeout); len(stalled) != 0 {
		t.Errorf("a session inside its timeout was reported as stalled: %v", stalled)
	}
}

func TestASilentSessionStallsAfterItsTimeout(t *testing.T) {
	sessions := NewCaptureSessions()
	sessions.SetMode(guild, Running)

	now := time.Unix(1_700_000_000, 0)
	sessions.seen(guild, now)

	stalled := sessions.Stalled(now.Add(shortTimeout+time.Second), fixedTimeout)
	if len(stalled) != 1 || stalled[0] != guild {
		t.Errorf("expected %s to be stalled, got %v", guild, stalled)
	}
}

// Nothing is being applied to a session that is not running, so there is
// nothing to fail open from. Warning about it would mean telling a guild its
// capture failed when it never asked the bot to use one.
func TestOnlyARunningSessionCanStall(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)

	for _, mode := range []Mode{Stopped, Paused} {
		t.Run(mode.String(), func(t *testing.T) {
			sessions := NewCaptureSessions()
			sessions.SetMode(guild, mode)
			sessions.seen(guild, now)

			if stalled := sessions.Stalled(now.Add(time.Hour), fixedTimeout); len(stalled) != 0 {
				t.Errorf("a %s session was reported as stalled: %v", mode, stalled)
			}
		})
	}
}

// A capture that has never connected has not stopped responding.
func TestAGuildThatNeverConnectedDoesNotStall(t *testing.T) {
	sessions := NewCaptureSessions()
	sessions.SetMode(guild, Running)

	if stalled := sessions.Stalled(time.Unix(1_700_000_000, 0), fixedTimeout); len(stalled) != 0 {
		t.Errorf("a guild with no capture was reported as stalled: %v", stalled)
	}
}

// The watchdog runs every second and a dead capture stays dead. Without this,
// one crash would warn a channel once a second until somebody noticed.
func TestAStallIsOnlyReportedOnce(t *testing.T) {
	sessions := NewCaptureSessions()
	sessions.SetMode(guild, Running)
	sessions.seen(guild, time.Unix(1_700_000_000, 0))

	if !sessions.markStalled(guild) {
		t.Fatal("the first stall should be reported")
	}
	if sessions.markStalled(guild) {
		t.Error("the same stall was reported twice")
	}
	if got := sessions.Mode(guild); got != Paused {
		t.Errorf("a stalled session is %s, want paused", got)
	}
	if stalled := sessions.Stalled(time.Now().Add(time.Hour), fixedTimeout); len(stalled) != 0 {
		t.Errorf("an already stalled session was reported again: %v", stalled)
	}
}

// Capture coming back restores what the administrator chose, rather than
// leaving a session paused by a fault nobody asked for.
func TestCaptureComingBackRestoresTheChosenMode(t *testing.T) {
	sessions := NewCaptureSessions()
	sessions.SetMode(guild, Running)
	sessions.seen(guild, time.Unix(1_700_000_000, 0))
	sessions.markStalled(guild)

	restored, wasStalled := sessions.recovered(guild)
	if !wasStalled {
		t.Fatal("the session was stalled and recovery said otherwise")
	}
	if restored != Running {
		t.Errorf("restored %s, want running", restored)
	}
	if got := sessions.Mode(guild); got != Running {
		t.Errorf("the session is %s, want running", got)
	}
}

// A pause an administrator asked for must survive a capture that comes and
// goes. Resuming it on their behalf would apply voice changes they switched off.
func TestRecoveryDoesNotResumeASessionNobodyStalled(t *testing.T) {
	sessions := NewCaptureSessions()
	sessions.SetMode(guild, Paused)

	if _, wasStalled := sessions.recovered(guild); wasStalled {
		t.Error("a session that was never stalled reported a recovery")
	}
	if got := sessions.Mode(guild); got != Paused {
		t.Errorf("the session is %s, want paused", got)
	}
}

// Any message proves capture is alive, not only a heartbeat: a session sending
// game events is evidently running.
func TestAnyMessageCountsAsALifeSign(t *testing.T) {
	controller := &Bot{CaptureSessions: NewCaptureSessions()}
	controller.CaptureSessions.SetMode(guild, Paused)

	before := controller.CaptureSessions.forGuild(guild).lastSeen
	if !before.IsZero() {
		t.Fatal("a fresh session should have no life sign")
	}

	if err := controller.HandleCapture(guild, &protocol.Snapshot{
		Phase:   protocol.PhaseTasks,
		Players: []protocol.Player{{Name: "Red"}},
	}); err != nil {
		t.Fatalf("handling a snapshot: %v", err)
	}

	if controller.CaptureSessions.forGuild(guild).lastSeen.IsZero() {
		t.Error("a snapshot did not count as a life sign")
	}
}

// An unreadable configuration must not switch the fail-safe off. A fail-safe
// that stops working when a database blinks is not one.
func TestTheTimeoutFallsBackToTheDocumentedDefault(t *testing.T) {
	controller := &Bot{CaptureSessions: NewCaptureSessions()}

	if got := controller.captureTimeout(guild); got != 60*time.Second {
		t.Errorf("got %s, want the documented 60s default", got)
	}
}

// The whole reason the fail-safe exists: a capture that dies mid-round leaves
// players server-muted, and nothing in Discord undoes that on its own.
func TestACrashPausesTheSessionSoPlayersAreNotLeftMuted(t *testing.T) {
	controller := &Bot{CaptureSessions: NewCaptureSessions()}
	controller.CaptureSessions.SetMode(guild, Running)

	now := time.Unix(1_700_000_000, 0)
	controller.CaptureSessions.seen(guild, now)

	controller.checkCaptureTimeouts(now.Add(2 * time.Minute))

	if got := controller.CaptureSessions.Mode(guild); got != Paused {
		t.Errorf("after a capture crash the session is %s, want paused", got)
	}
}
