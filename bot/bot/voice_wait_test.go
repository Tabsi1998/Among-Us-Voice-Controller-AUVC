package bot

import (
	"errors"
	"testing"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
)

// When AUVC waits before it changes voice. Waiting where it should not delays a
// kill being silenced; not waiting where it should cuts people off mid-sentence.
func TestVoiceWaitOnlyAfterMeetingsAndAtRoundEnd(t *testing.T) {
	for _, tc := range []struct {
		name     string
		from, to game.Phase
		want     time.Duration
	}{
		{"after a meeting", game.DISCUSS, game.TASKS, waitAfterMeeting},
		{"round over", game.TASKS, game.GAMEOVER, waitAtRoundEnd},
		{"round over during a meeting", game.DISCUSS, game.GAMEOVER, waitAtRoundEnd},
		{"an exile ends the game, reported as the lobby", game.DISCUSS, game.LOBBY, waitAtRoundEnd},
		{"the tasks end in the lobby", game.TASKS, game.LOBBY, waitAtRoundEnd},
		{"the first round starts", game.LOBBY, game.TASKS, 0},
		{"a meeting starts", game.TASKS, game.DISCUSS, 0},
		{"joining a lobby from the menu", game.MENU, game.LOBBY, 0},
		{"nothing changed", game.TASKS, game.TASKS, 0},
		{"leaving the game", game.TASKS, game.MENU, 0},
		{"the first message after a restart", game.UNINITIALIZED, game.TASKS, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := voiceWait(tc.from, tc.to); got != tc.want {
				t.Errorf("voiceWait(%v, %v) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

// scheduled is a wait that runs only when the test says so.
type scheduled struct {
	work    func()
	stopped bool
}

func (s *scheduled) Stop() bool {
	s.stopped = true
	return true
}

// waitsForTest returns the waits together with the list of scheduled work, in
// order, so a test can run or drop it by hand.
func waitsForTest() (*voiceWaits, *[]*scheduled) {
	scheduledWork := []*scheduled{}
	waits := newVoiceWaits()
	waits.after = func(_ time.Duration, work func()) stopper {
		entry := &scheduled{work: work}
		scheduledWork = append(scheduledWork, entry)
		return entry
	}
	return waits, &scheduledWork
}

func TestWorkWithoutAWaitRunsAtOnceAndReportsItsError(t *testing.T) {
	waits, scheduledWork := waitsForTest()
	failure := errors.New("discord said no")
	ran := 0

	err := waits.run("guild", 0, func() error {
		ran++
		return failure
	})

	if ran != 1 {
		t.Errorf("the work ran %d times, want once", ran)
	}
	if !errors.Is(err, failure) {
		t.Errorf("run returned %v, want the work's error", err)
	}
	if len(*scheduledWork) != 0 {
		t.Errorf("%d waits were scheduled, want none", len(*scheduledWork))
	}
}

func TestWorkWithAWaitRunsOnlyWhenTheWaitIsOver(t *testing.T) {
	waits, scheduledWork := waitsForTest()
	ran := 0

	if err := waits.run("guild", waitAfterMeeting, func() error { ran++; return nil }); err != nil {
		t.Fatalf("run: %v", err)
	}
	if ran != 0 {
		t.Fatal("the work ran before the wait was over")
	}
	if !waits.waiting("guild") {
		t.Error("the guild is not marked as waiting")
	}
	if len(*scheduledWork) != 1 {
		t.Fatalf("%d waits were scheduled, want one", len(*scheduledWork))
	}

	(*scheduledWork)[0].work()

	if ran != 1 {
		t.Errorf("the work ran %d times after the wait, want once", ran)
	}
	if waits.waiting("guild") {
		t.Error("the guild still counts as waiting after its work ran")
	}
}

// The phase the waiting change belongs to is over. Applying it afterwards would
// mute a meeting that already ended.
func TestANewerPhaseReplacesAWaitingOne(t *testing.T) {
	waits, scheduledWork := waitsForTest()
	older, newer := 0, 0

	if err := waits.run("guild", waitAfterMeeting, func() error { older++; return nil }); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := waits.run("guild", 0, func() error { newer++; return nil }); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !(*scheduledWork)[0].stopped {
		t.Error("the older wait was not cancelled")
	}
	if older != 0 {
		t.Error("the older work ran even though a newer phase arrived")
	}
	if newer != 1 {
		t.Errorf("the newer work ran %d times, want once", newer)
	}
	if waits.waiting("guild") {
		t.Error("the guild still waits after the newer work ran at once")
	}
}

func TestAWaitOfAnotherGuildIsLeftAlone(t *testing.T) {
	waits, scheduledWork := waitsForTest()
	first := 0

	if err := waits.run("guild-a", waitAtRoundEnd, func() error { first++; return nil }); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := waits.run("guild-b", 0, func() error { return nil }); err != nil {
		t.Fatalf("run: %v", err)
	}

	if (*scheduledWork)[0].stopped {
		t.Error("the wait of the other guild was cancelled")
	}
	if !waits.waiting("guild-a") {
		t.Error("guild-a stopped waiting because of guild-b")
	}

	(*scheduledWork)[0].work()
	if first != 1 {
		t.Errorf("the work of guild-a ran %d times, want once", first)
	}
}

// Nobody is left to return the error to once the wait is over, so it has to be
// reported rather than dropped.
func TestAFailureAfterTheWaitIsReported(t *testing.T) {
	waits, scheduledWork := waitsForTest()
	failure := errors.New("discord said no")
	var reported error
	waits.onError = func(_ string, err error) { reported = err }

	if err := waits.run("guild", waitAtRoundEnd, func() error { return failure }); err != nil {
		t.Fatalf("run returned %v, want nil while it waits", err)
	}
	(*scheduledWork)[0].work()

	if !errors.Is(reported, failure) {
		t.Errorf("the error was reported as %v, want the work's error", reported)
	}
}

// The handler has to ask for the wait, or the whole rule above never reaches a
// real round. A bot without a reconciler applies nothing, which is what keeps
// this test to the decision it is about.
func TestHandleCaptureHoldsTheVoiceChangeAfterAMeeting(t *testing.T) {
	controller := &Bot{CaptureSessions: NewCaptureSessions()}
	scheduledWork := []*scheduled{}
	controller.CaptureSessions.waits.after = func(_ time.Duration, work func()) stopper {
		entry := &scheduled{work: work}
		scheduledWork = append(scheduledWork, entry)
		return entry
	}
	controller.CaptureSessions.SetMode(guild, Running)

	seq := uint64(0)
	send := func(phase protocol.Phase) {
		t.Helper()

		seq++
		message := &protocol.GameStateChanged{
			Header: protocol.Header{Protocol: protocol.Version, Type: protocol.TypeGameStateChanged,
				Session: "session-a", Seq: seq},
			Phase: phase,
		}
		if err := controller.HandleCapture(guild, message); err != nil {
			t.Fatalf("handling %s: %v", phase, err)
		}
	}

	// Capture adopts a guild with a snapshot; anything else from an unknown
	// session is ignored.
	if err := controller.HandleCapture(guild, &protocol.Snapshot{
		Header: protocol.Header{Protocol: protocol.Version, Type: protocol.TypeSnapshot,
			Session: "session-a", Seq: 1},
		Phase: protocol.PhaseLobby,
	}); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	seq = 1

	send(protocol.PhaseTasks)
	send(protocol.PhaseDiscussion)
	if len(scheduledWork) != 0 {
		t.Fatalf("%d waits were scheduled before the meeting ended, want none", len(scheduledWork))
	}

	send(protocol.PhaseTasks)
	if len(scheduledWork) != 1 {
		t.Fatalf("%d waits were scheduled after the meeting, want one", len(scheduledWork))
	}
	if !controller.CaptureSessions.waits.waiting(guild) {
		t.Error("the guild does not wait after a meeting")
	}
}

func TestHandleCaptureHoldsTheReleaseAtTheEndOfARound(t *testing.T) {
	controller := &Bot{CaptureSessions: NewCaptureSessions()}
	scheduledWork := []*scheduled{}
	controller.CaptureSessions.waits.after = func(_ time.Duration, work func()) stopper {
		entry := &scheduled{work: work}
		scheduledWork = append(scheduledWork, entry)
		return entry
	}
	controller.CaptureSessions.SetMode(guild, Running)

	header := func(seq uint64, messageType protocol.Type) protocol.Header {
		return protocol.Header{Protocol: protocol.Version, Type: messageType, Session: "session-a", Seq: seq}
	}
	if err := controller.HandleCapture(guild, &protocol.Snapshot{
		Header: header(1, protocol.TypeSnapshot), Phase: protocol.PhaseTasks}); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(scheduledWork) != 0 {
		t.Fatalf("%d waits were scheduled when the round started, want none", len(scheduledWork))
	}

	if err := controller.HandleCapture(guild, &protocol.GameEnded{
		Header: header(2, protocol.TypeGameEnded)}); err != nil {
		t.Fatalf("game ended: %v", err)
	}
	if len(scheduledWork) != 1 {
		t.Fatalf("%d waits were scheduled at the end of the round, want one", len(scheduledWork))
	}
}
