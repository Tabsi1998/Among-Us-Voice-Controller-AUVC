package voice

import (
	"sync"
	"testing"
	"time"
)

// waves records how the reconciler sends its edits. Every move is held until
// the test lets go, so whatever else is sent meanwhile becomes visible.
type waves struct {
	mu   sync.Mutex
	rest []string

	moveArrived chan string
	restArrived chan string
	release     chan struct{}
}

func newWaves() *waves {
	return &waves{
		moveArrived: make(chan string, 16),
		restArrived: make(chan string, 16),
		release:     make(chan struct{}),
	}
}

func (w *waves) Apply(_ string, change Change) error {
	if change.MoveTo != nil {
		w.moveArrived <- change.UserID
		<-w.release
		return nil
	}

	w.mu.Lock()
	w.rest = append(w.rest, change.UserID)
	w.mu.Unlock()

	w.restArrived <- change.UserID
	return nil
}

// Every player of a phase change is edited at the same time. One after the
// other left the last player of a full lobby unmuted seconds after the first,
// because every request waited for the answer of the one before it.
//
// The moves still have to land before anybody is relaxed, so they are a wave of
// their own: a living player who can hear again while a corpse is still in the
// main channel gives the round away.
func TestMovesGoOutTogetherAndBeforeTheRest(t *testing.T) {
	applier := newWaves()
	reconciler := NewReconciler(applier)

	// Three players move into the ghost channel, two are only unmuted.
	observed := map[string]Observed{
		"move-a": {ChannelID: main},
		"move-b": {ChannelID: main},
		"move-c": {ChannelID: main},
		"rest-a": {ChannelID: main, Muted: true},
		"rest-b": {ChannelID: main, Muted: true},
	}
	desired := map[string]DesiredVoiceState{
		"move-a": {TargetChannelID: ghost},
		"move-b": {TargetChannelID: ghost},
		"move-c": {TargetChannelID: ghost},
		"rest-a": {TargetChannelID: main},
		"rest-b": {TargetChannelID: main},
	}

	done := make(chan error, 1)
	go func() { done <- reconciler.Reconcile("guild", observed, desired) }()

	// All three moves are in flight at once. Sending them one after the other
	// would never get past the first, because Apply holds every move.
	for i := 0; i < 3; i++ {
		select {
		case <-applier.moveArrived:
		case <-time.After(5 * time.Second):
			t.Fatalf("only %d of 3 moves were sent at once", i)
		}
	}

	// Nothing else may be sent while a move is still open.
	select {
	case user := <-applier.restArrived:
		t.Fatalf("%s was edited while the moves were still open", user)
	case <-time.After(300 * time.Millisecond):
	}

	close(applier.release)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("reconcile: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the reconciliation did not finish")
	}

	applier.mu.Lock()
	defer applier.mu.Unlock()
	if len(applier.rest) != 2 {
		t.Errorf("%d players were unmuted after the moves, want 2", len(applier.rest))
	}
}
