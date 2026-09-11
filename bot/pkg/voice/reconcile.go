package voice

import (
	"fmt"
	"sort"
	"sync"
)

// Observed is the voice state Discord currently reports for a player.
type Observed struct {
	// ChannelID is the voice channel the player is connected to. Empty means
	// they are not in voice at all.
	ChannelID string
	Muted     bool
	Deafened  bool
}

// Connected reports whether the player is in a voice channel. Someone who is
// not connected cannot be moved, muted or deafened.
func (o Observed) Connected() bool {
	return o.ChannelID != ""
}

// Change is the minimal edit that brings one player to their desired state.
//
// Every field is a pointer so that only what actually differs is sent. Discord
// applies a member edit as a whole, and resending a value the player already
// carries is a write nobody needs and a rate limit slot wasted.
type Change struct {
	UserID string
	// MoveTo is the channel to move the player into, or nil when they are
	// already there. It is never a pointer to an empty string: Discord treats an
	// empty channel id as "disconnect from voice", so an unset policy target
	// must never reach the API as a move.
	MoveTo *string
	// Muted and Deafened are set only when the server mute or deafen differs.
	Muted    *bool
	Deafened *bool
}

// Empty reports whether the change asks for nothing.
func (c Change) Empty() bool {
	return c.MoveTo == nil && c.Muted == nil && c.Deafened == nil
}

// Diff returns the changes needed to bring the observed voice states in line
// with what the policy wants, and nothing else.
//
// Applying the result and observing again yields an empty diff. That is what
// makes reconciliation idempotent, and it is also what stops move loops: a
// player who is already where the policy wants them produces no action, so a
// voice state update caused by our own move cannot trigger another one.
//
// Players are skipped when:
//   - the policy has no opinion about them, so they are absent from desired;
//   - they are not connected to voice, because there is nothing to move.
//
// Results are ordered by user id so a reconciliation is reproducible and its
// log readable.
func Diff(observed map[string]Observed, desired map[string]DesiredVoiceState) []Change {
	changes := make([]Change, 0, len(desired))

	for userID, want := range desired {
		have, seen := observed[userID]
		if !seen || !have.Connected() {
			continue
		}

		change := Change{UserID: userID}

		// An empty target means the policy has no opinion, and sending it would
		// disconnect the player rather than leave them alone.
		if want.TargetChannelID != "" && want.TargetChannelID != have.ChannelID {
			target := want.TargetChannelID
			change.MoveTo = &target
		}
		if want.Muted != have.Muted {
			muted := want.Muted
			change.Muted = &muted
		}
		if want.Deafened != have.Deafened {
			deafened := want.Deafened
			change.Deafened = &deafened
		}

		if !change.Empty() {
			changes = append(changes, change)
		}
	}

	// Moves go first, and this is a correctness requirement rather than tidiness.
	//
	// When a player dies at the moment a meeting starts, one reconciliation
	// carries both "undeafen the living" and "move the dead to the ghost
	// channel". Relaxing the living first lets them hear a corpse still sitting
	// in the main channel, which gives the round away. Moving people into the
	// right room before changing who can hear is the ordering that cannot leak.
	//
	// Within each group the order is by user id so a reconciliation stays
	// reproducible and its log readable.
	sort.Slice(changes, func(i, j int) bool {
		iMoves, jMoves := changes[i].MoveTo != nil, changes[j].MoveTo != nil
		if iMoves != jMoves {
			return iMoves
		}
		return changes[i].UserID < changes[j].UserID
	})
	return changes
}

// Applier performs one member edit against Discord. It exists so the reconciler
// can be tested without a connection.
type Applier interface {
	Apply(guildID string, change Change) error
}

// Reconciler applies changes and serializes the work per guild.
//
// Voice state updates arrive asynchronously, so two goroutines can easily be
// reconciling the same guild from slightly different observations. Serializing
// per guild means the later one sees the result of the earlier one instead of
// racing it, while separate guilds still proceed independently.
type Reconciler struct {
	applier Applier

	mu     sync.Mutex
	guilds map[string]*sync.Mutex
}

func NewReconciler(applier Applier) *Reconciler {
	return &Reconciler{applier: applier, guilds: map[string]*sync.Mutex{}}
}

// lockFor returns the mutex guarding one guild, creating it on first use.
func (r *Reconciler) lockFor(guildID string) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()

	lock, ok := r.guilds[guildID]
	if !ok {
		lock = &sync.Mutex{}
		r.guilds[guildID] = lock
	}
	return lock
}

// Reconcile applies every difference between the observed and desired states.
//
// A failure on one player does not abandon the rest: the others are still
// brought into line and the errors are reported together. Giving up halfway
// would leave a round in a state that is neither the old one nor the new one.
func (r *Reconciler) Reconcile(guildID string, observed map[string]Observed, desired map[string]DesiredVoiceState) error {
	lock := r.lockFor(guildID)
	lock.Lock()
	defer lock.Unlock()

	var failures []error
	for _, change := range Diff(observed, desired) {
		if err := r.applier.Apply(guildID, change); err != nil {
			failures = append(failures, fmt.Errorf("user %s: %w", change.UserID, err))
		}
	}

	switch len(failures) {
	case 0:
		return nil
	case 1:
		return failures[0]
	default:
		return fmt.Errorf("%d of the voice changes failed: %w", len(failures), failures[0])
	}
}
