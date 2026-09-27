package bot

import (
	"log"
	"sync"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
)

// The two moments where changing voice the instant the game says so is too
// abrupt for the people in the call.
const (
	// waitAfterMeeting keeps everybody audible for a moment after a meeting,
	// so the last sentence is not cut off when the round resumes.
	waitAfterMeeting = 3 * time.Second
	// waitAtRoundEnd delays the release at the end of a round, so the win
	// screen is not accompanied by every ghost coming back at once.
	waitAtRoundEnd = 3 * time.Second
)

// voiceWait is how long AUVC waits before it changes anybody's voice for a
// phase change.
//
// Everything else stays immediate. A kill must be silent at once, a player
// joining the lobby has to be released at once, and the release after a crash
// never runs through here at all.
func voiceWait(from, to game.Phase) time.Duration {
	switch to {
	case game.TASKS:
		// Back to the tasks after a meeting. Only from a meeting: the first
		// round of a game starts from the lobby and is silenced right away.
		if from == game.DISCUSS {
			return waitAfterMeeting
		}
	case game.GAMEOVER, game.LOBBY:
		// The end of a round, however capture reports it. An exile that ends
		// the game arrives as the lobby, a normal end as game over.
		if from == game.TASKS || from == game.DISCUSS {
			return waitAtRoundEnd
		}
	}
	return 0
}

// stopper is what a scheduled wait can be cancelled with. *time.Timer is one.
type stopper interface {
	Stop() bool
}

// voiceWaits runs the voice work of a guild, now or after a wait.
//
// Only one wait per guild can be pending. A newer phase replaces an older one
// that is still waiting, because the state the older one would apply is gone:
// waiting three seconds and then muting everyone for a meeting that already
// ended is worse than not waiting at all.
type voiceWaits struct {
	mu      sync.Mutex
	pending map[string]stopper

	// after schedules work. It is a field so tests can run without a clock.
	after func(time.Duration, func()) stopper
	// onError reports a failure of work that ran after a wait, where there is
	// no caller left to return it to.
	onError func(guildID string, err error)
}

func newVoiceWaits() *voiceWaits {
	return &voiceWaits{
		pending: map[string]stopper{},
		after:   func(d time.Duration, f func()) stopper { return time.AfterFunc(d, f) },
		onError: func(guildID string, err error) {
			log.Printf("Voice change for guild %s after the wait failed: %v", guildID, err)
		},
	}
}

// run performs work for one guild, immediately when wait is zero.
//
// A pending wait for the same guild is always cancelled first, whether this
// call waits itself or not. The error of work that had to wait is reported
// through onError; run returns only what work returned when it ran at once.
func (w *voiceWaits) run(guildID string, wait time.Duration, work func() error) error {
	w.mu.Lock()
	if pending, ok := w.pending[guildID]; ok {
		pending.Stop()
		delete(w.pending, guildID)
	}
	if wait <= 0 {
		w.mu.Unlock()
		return work()
	}
	w.pending[guildID] = w.after(wait, func() {
		w.mu.Lock()
		delete(w.pending, guildID)
		w.mu.Unlock()

		if err := work(); err != nil && w.onError != nil {
			w.onError(guildID, err)
		}
	})
	w.mu.Unlock()
	return nil
}

// waiting reports whether a guild has a wait pending. It exists for the tests
// and for reasoning about the state in one place.
func (w *voiceWaits) waiting(guildID string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	_, ok := w.pending[guildID]
	return ok
}
