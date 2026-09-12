package voice

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/session"
)

func inMain() Observed { return Observed{ChannelID: main} }

// Reconciling a state that already matches must do nothing. Without this,
// every voice state update our own move produces would trigger another move,
// and players would be dragged between channels indefinitely.
func TestNoChangesWhenObservationAlreadyMatches(t *testing.T) {
	observed := map[string]Observed{
		"a": {ChannelID: main, Muted: true, Deafened: true},
		"b": {ChannelID: ghost},
	}
	desired := map[string]DesiredVoiceState{
		"a": {TargetChannelID: main, Muted: true, Deafened: true},
		"b": {TargetChannelID: ghost},
	}

	if changes := Diff(observed, desired); len(changes) != 0 {
		t.Errorf("expected no changes, got %+v", changes)
	}
}

// Applying a diff and observing the result must leave nothing to do. This is
// the property that makes reconciliation safe to repeat.
func TestDiffIsIdempotent(t *testing.T) {
	observed := map[string]Observed{"a": inMain()}
	desired := map[string]DesiredVoiceState{
		"a": {TargetChannelID: ghost, Muted: true},
	}

	first := Diff(observed, desired)
	if len(first) != 1 {
		t.Fatalf("expected one change, got %+v", first)
	}

	// Pretend the change was applied.
	observed["a"] = Observed{ChannelID: ghost, Muted: true}

	if second := Diff(observed, desired); len(second) != 0 {
		t.Errorf("expected nothing left after applying, got %+v", second)
	}
}

// Only what differs is sent. Resending a value the player already carries is a
// write nobody needs and a rate limit slot wasted.
func TestOnlyDifferingFieldsAreSet(t *testing.T) {
	observed := map[string]Observed{"a": {ChannelID: main, Muted: true}}
	desired := map[string]DesiredVoiceState{
		"a": {TargetChannelID: main, Muted: true, Deafened: true},
	}

	changes := Diff(observed, desired)
	if len(changes) != 1 {
		t.Fatalf("expected one change, got %+v", changes)
	}

	change := changes[0]
	if change.MoveTo != nil {
		t.Errorf("player is already in the target channel; no move should be sent: %v", *change.MoveTo)
	}
	if change.Muted != nil {
		t.Errorf("mute already matches; it should not be resent")
	}
	if change.Deafened == nil || !*change.Deafened {
		t.Errorf("deafen differs and must be sent, got %v", change.Deafened)
	}
}

// Discord treats an empty channel id as "disconnect from voice". A policy that
// has no opinion about the channel must never reach the API as a move, or the
// bot would kick players out of voice entirely.
func TestAnEmptyTargetNeverBecomesAMove(t *testing.T) {
	observed := map[string]Observed{"a": inMain()}
	desired := map[string]DesiredVoiceState{"a": {Muted: true}}

	changes := Diff(observed, desired)
	if len(changes) != 1 {
		t.Fatalf("expected the mute change, got %+v", changes)
	}
	if changes[0].MoveTo != nil {
		t.Fatalf("an unset target produced a move to %q", *changes[0].MoveTo)
	}
}

// Somebody who is not in a voice channel cannot be moved, muted or deafened.
// Trying anyway produces an API error per player and nothing useful.
func TestPlayersNotInVoiceAreSkipped(t *testing.T) {
	observed := map[string]Observed{
		"offline":      {},
		"disconnected": {ChannelID: ""},
	}
	desired := map[string]DesiredVoiceState{
		"offline":      {TargetChannelID: ghost, Muted: true},
		"disconnected": {TargetChannelID: ghost},
		"unknown":      {TargetChannelID: ghost},
	}

	if changes := Diff(observed, desired); len(changes) != 0 {
		t.Errorf("expected nobody to be touched, got %+v", changes)
	}
}

// A player the policy has no opinion about must never be acted on, even when
// Discord reports them sitting in a channel the bot manages.
func TestObservedPlayersWithoutADesiredStateAreLeftAlone(t *testing.T) {
	observed := map[string]Observed{
		"managed":   inMain(),
		"bystander": {ChannelID: ghost, Muted: true},
	}
	desired := map[string]DesiredVoiceState{"managed": {TargetChannelID: ghost}}

	changes := Diff(observed, desired)
	if len(changes) != 1 || changes[0].UserID != "managed" {
		t.Errorf("only the managed player may be changed, got %+v", changes)
	}
}

func TestChangesAreOrderedByUserID(t *testing.T) {
	observed := map[string]Observed{"c": inMain(), "a": inMain(), "b": inMain()}
	desired := map[string]DesiredVoiceState{
		"c": {TargetChannelID: ghost},
		"a": {TargetChannelID: ghost},
		"b": {TargetChannelID: ghost},
	}

	changes := Diff(observed, desired)
	for i, want := range []string{"a", "b", "c"} {
		if changes[i].UserID != want {
			t.Fatalf("change %d is %q, want %q", i, changes[i].UserID, want)
		}
	}
}

type recorder struct {
	mu      sync.Mutex
	applied []Change
	failOn  map[string]error
	// during counts how many Apply calls are in flight at once.
	during  int
	maxSeen int
}

func (r *recorder) Apply(_ string, change Change) error {
	r.mu.Lock()
	r.during++
	if r.during > r.maxSeen {
		r.maxSeen = r.during
	}
	r.applied = append(r.applied, change)
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.during--
		r.mu.Unlock()
	}()

	if err, ok := r.failOn[change.UserID]; ok {
		return err
	}
	return nil
}

func TestReconcileAppliesEveryDifference(t *testing.T) {
	applier := &recorder{}
	reconciler := NewReconciler(applier)

	observed := map[string]Observed{"a": inMain(), "b": inMain()}
	desired := map[string]DesiredVoiceState{
		"a": {TargetChannelID: ghost},
		"b": {TargetChannelID: main, Muted: true},
	}

	if err := reconciler.Reconcile("guild", observed, desired); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(applier.applied) != 2 {
		t.Errorf("expected two edits, got %+v", applier.applied)
	}
}

// One player failing must not abandon the others. Stopping halfway leaves a
// round in a state that is neither the old one nor the new one.
func TestOneFailureDoesNotAbandonTheRest(t *testing.T) {
	applier := &recorder{failOn: map[string]error{"a": errors.New("rate limited")}}
	reconciler := NewReconciler(applier)

	observed := map[string]Observed{"a": inMain(), "b": inMain(), "c": inMain()}
	desired := map[string]DesiredVoiceState{
		"a": {TargetChannelID: ghost},
		"b": {TargetChannelID: ghost},
		"c": {TargetChannelID: ghost},
	}

	err := reconciler.Reconcile("guild", observed, desired)
	if err == nil {
		t.Fatal("expected the failure to be reported")
	}
	if len(applier.applied) != 3 {
		t.Errorf("every player should still have been attempted, got %d", len(applier.applied))
	}
}

func TestReconcileReportsHowManyFailed(t *testing.T) {
	applier := &recorder{failOn: map[string]error{
		"a": errors.New("boom"),
		"b": errors.New("boom"),
	}}
	reconciler := NewReconciler(applier)

	observed := map[string]Observed{"a": inMain(), "b": inMain()}
	desired := map[string]DesiredVoiceState{
		"a": {TargetChannelID: ghost},
		"b": {TargetChannelID: ghost},
	}

	err := reconciler.Reconcile("guild", observed, desired)
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); got == "" || !strings.Contains(got, "2 of the voice changes failed") {
		t.Errorf("error should say how many failed, got %q", got)
	}
}

// Voice state updates arrive asynchronously, so the same guild can be
// reconciled from two goroutines at once. Serializing per guild means the later
// one sees the result of the earlier one instead of racing it.
func TestReconcileSerializesPerGuild(t *testing.T) {
	applier := &recorder{}
	reconciler := NewReconciler(applier)

	observed := map[string]Observed{"a": inMain()}
	desired := map[string]DesiredVoiceState{"a": {TargetChannelID: ghost}}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = reconciler.Reconcile("guild", observed, desired)
		}()
	}
	wg.Wait()

	if applier.maxSeen > 1 {
		t.Errorf("saw %d concurrent edits in one guild; work must be serialized", applier.maxSeen)
	}
}

// The policy and the reconciler have to agree end to end: a session in tasks
// must produce exactly the moves and mutes the ghost-chat table describes.
func TestPolicyAndReconcilerAgreeDuringTasks(t *testing.T) {
	state := session.State{Phase: game.TASKS, Players: []session.PlayerState{
		{UserID: "alive", InGameName: "Red", Alive: true},
		{UserID: "ghost-player", InGameName: "Blue", Revealed: true},
	}}
	observed := map[string]Observed{"alive": inMain(), "ghost-player": inMain()}

	changes := Diff(observed, Desired(state, ghostChat()))
	if len(changes) != 2 {
		t.Fatalf("expected both players to change, got %+v", changes)
	}

	byUser := map[string]Change{}
	for _, change := range changes {
		byUser[change.UserID] = change
	}

	alive := byUser["alive"]
	if alive.MoveTo != nil {
		t.Error("a living player in main must not be moved during tasks")
	}
	if alive.Muted == nil || !*alive.Muted || alive.Deafened == nil || !*alive.Deafened {
		t.Errorf("a living player must be muted and deafened during tasks, got %+v", alive)
	}

	dead := byUser["ghost-player"]
	if dead.MoveTo == nil || *dead.MoveTo != ghost {
		t.Errorf("a dead player must be moved to the ghost channel, got %+v", dead)
	}
	if dead.Muted != nil || dead.Deafened != nil {
		t.Errorf("a dead player was already open and must not be re-muted, got %+v", dead)
	}
}

// A player who dies exactly as a meeting starts produces one reconciliation
// carrying both "undeafen the living" and "move the dead to ghost". Relaxing
// the living first lets them hear a corpse still in the main channel, which
// gives the round away. Moves therefore have to be applied first.
func TestMovesAreAppliedBeforeRelaxingTheLiving(t *testing.T) {
	// zzz sorts last by user id, so only the move-first rule can put it first.
	observed := map[string]Observed{
		"aaa-living": {ChannelID: main, Muted: true, Deafened: true},
		"zzz-dead":   {ChannelID: main},
	}
	desired := map[string]DesiredVoiceState{
		"aaa-living": {TargetChannelID: main},
		"zzz-dead":   {TargetChannelID: ghost},
	}

	changes := Diff(observed, desired)
	if len(changes) != 2 {
		t.Fatalf("expected both players to change, got %+v", changes)
	}

	if changes[0].UserID != "zzz-dead" {
		t.Errorf("the move must be applied first, got %q", changes[0].UserID)
	}
	if changes[0].MoveTo == nil || *changes[0].MoveTo != ghost {
		t.Errorf("expected the ghost move first, got %+v", changes[0])
	}
	if changes[1].UserID != "aaa-living" {
		t.Errorf("the living player must be relaxed last, got %q", changes[1].UserID)
	}
}

// Within the moves, and within the rest, the order stays by user id so a
// reconciliation is reproducible.
func TestOrderingIsStableWithinEachGroup(t *testing.T) {
	observed := map[string]Observed{
		"m-b": {ChannelID: main},
		"m-a": {ChannelID: main},
		"v-b": {ChannelID: main, Muted: true},
		"v-a": {ChannelID: main, Muted: true},
	}
	desired := map[string]DesiredVoiceState{
		"m-b": {TargetChannelID: ghost},
		"m-a": {TargetChannelID: ghost},
		"v-b": {TargetChannelID: main},
		"v-a": {TargetChannelID: main},
	}

	changes := Diff(observed, desired)
	got := []string{}
	for _, change := range changes {
		got = append(got, change.UserID)
	}

	want := []string{"m-a", "m-b", "v-a", "v-b"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order is %v, want %v", got, want)
		}
	}
}
