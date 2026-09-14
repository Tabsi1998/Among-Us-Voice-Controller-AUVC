package bot

import (
	"errors"
	"sync"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/voice"
)

// holdRecorder stands in for Discord and notes what was on record at the moment
// each change reached it.
type holdRecorder struct {
	mu      sync.Mutex
	store   *sqlite.DB
	fail    error
	changes []voice.Change
	seen    []sqlite.VoiceHold
}

func (r *holdRecorder) Apply(guildID string, change voice.Change) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.store != nil {
		hold, _ := r.store.VoiceHold(guildID, change.UserID)
		r.seen = append(r.seen, hold)
	}
	if r.fail != nil {
		return r.fail
	}
	r.changes = append(r.changes, change)
	return nil
}

type unreadableHolds struct{}

func (unreadableHolds) VoiceHold(string, string) (sqlite.VoiceHold, error) {
	return sqlite.VoiceHold{}, errors.New("database is locked")
}
func (unreadableHolds) SaveVoiceHold(sqlite.VoiceHold) error { return nil }
func (unreadableHolds) VoiceHolds() ([]sqlite.VoiceHold, error) {
	return nil, nil
}

func holdFlag(value bool) *bool    { return &value }
func holdTarget(id string) *string { return &id }

func holdingFor(db *sqlite.DB, inner voice.Applier) holdingApplier {
	return holdingApplier{inner: inner, store: db, ghostChannel: func(string) string { return "ghost" }}
}

// holdBot is a bot restarted over whatever db holds, with main and ghost
// channels configured.
func holdBot(t *testing.T, db *sqlite.DB, recorder *holdRecorder) *Bot {
	t.Helper()

	config := sqlite.DefaultGuildConfig(crewGuild)
	config.MainVoiceChannelID = "main"
	config.GhostVoiceChannelID = "ghost"
	if err := db.SaveGuildConfig(config); err != nil {
		t.Fatalf("save configuration: %v", err)
	}
	holds, err := db.VoiceHolds()
	if err != nil {
		t.Fatalf("read holds: %v", err)
	}

	bot := &Bot{
		AUVC:            au.NewService(db, "test", "test"),
		CaptureSessions: NewCaptureSessions(),
		holdStore:       db,
		holds:           newHoldRecovery(holds),
		voiceApplier:    recorder,
	}
	bot.Reconciler = voice.NewReconciler(holdingApplier{
		inner: recorder, store: db, claim: bot.holds.claim,
		ghostChannel: func(guildID string) string { return bot.voiceChannels(guildID).GhostChannelID },
	})
	return bot
}

func TestAMuteIsWrittenDownBeforeItIsMade(t *testing.T) {
	db := openCrewDB(t)
	recorder := &holdRecorder{store: db}

	err := holdingFor(db, recorder).Apply(crewGuild, voice.Change{UserID: "member", Muted: holdFlag(true), Deafened: holdFlag(true)})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	if len(recorder.seen) != 1 || !recorder.seen[0].Muted || !recorder.seen[0].Deafened {
		t.Fatalf("the mute reached Discord before it was written down: %+v", recorder.seen)
	}
	if hold, _ := db.VoiceHold(crewGuild, "member"); !hold.Muted || !hold.Deafened {
		t.Errorf("the mute is not on record: %+v", hold)
	}
}

func TestLiftingEverythingForgetsTheMember(t *testing.T) {
	db := openCrewDB(t)
	applier := holdingFor(db, &holdRecorder{})

	if err := applier.Apply(crewGuild, voice.Change{UserID: "member", Muted: holdFlag(true), Deafened: holdFlag(true)}); err != nil {
		t.Fatalf("mute: %v", err)
	}
	if err := applier.Apply(crewGuild, voice.Change{UserID: "member", Muted: holdFlag(false), Deafened: holdFlag(false)}); err != nil {
		t.Fatalf("unmute: %v", err)
	}

	if holds, _ := db.VoiceHolds(); len(holds) != 0 {
		t.Errorf("a lifted mute is still on record: %+v", holds)
	}
}

// If Discord refuses the unmute, the member is still muted, and a crash now
// must still find them.
func TestAFailedUnmuteStaysOnRecord(t *testing.T) {
	db := openCrewDB(t)
	recorder := &holdRecorder{}
	applier := holdingFor(db, recorder)

	if err := applier.Apply(crewGuild, voice.Change{UserID: "member", Muted: holdFlag(true)}); err != nil {
		t.Fatalf("mute: %v", err)
	}
	recorder.fail = errors.New("rate limited")
	if err := applier.Apply(crewGuild, voice.Change{UserID: "member", Muted: holdFlag(false)}); err == nil {
		t.Fatal("the failed unmute was not reported")
	}

	if hold, _ := db.VoiceHold(crewGuild, "member"); !hold.Muted {
		t.Errorf("a member Discord did not unmute was forgotten: %+v", hold)
	}
}

func TestAMoveIntoTheGhostChannelIsWrittenDownAndAMoveBackForgotten(t *testing.T) {
	db := openCrewDB(t)
	applier := holdingFor(db, &holdRecorder{})

	if err := applier.Apply(crewGuild, voice.Change{UserID: "member", MoveTo: holdTarget("ghost")}); err != nil {
		t.Fatalf("move to ghost: %v", err)
	}
	if hold, _ := db.VoiceHold(crewGuild, "member"); hold.GhostChannelID != "ghost" {
		t.Fatalf("the move into the ghost channel is not on record: %+v", hold)
	}

	if err := applier.Apply(crewGuild, voice.Change{UserID: "member", MoveTo: holdTarget("main")}); err != nil {
		t.Fatalf("move back: %v", err)
	}
	if holds, _ := db.VoiceHolds(); len(holds) != 0 {
		t.Errorf("a member moved back is still on record: %+v", holds)
	}
}

func TestAChangeThatHoldsNothingLeavesNoRecord(t *testing.T) {
	db := openCrewDB(t)
	recorder := &holdRecorder{}

	if err := holdingFor(db, recorder).Apply(crewGuild, voice.Change{UserID: "member", MoveTo: holdTarget("main")}); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if len(recorder.changes) != 1 {
		t.Fatalf("the move was not made: %+v", recorder.changes)
	}
	if holds, _ := db.VoiceHolds(); len(holds) != 0 {
		t.Errorf("a plain move was recorded: %+v", holds)
	}
}

// Muting somebody without a record could strand them after a crash. Not muting
// them harms nobody.
func TestAChangeThatCannotBeWrittenDownIsNotMade(t *testing.T) {
	recorder := &holdRecorder{}
	applier := holdingApplier{inner: recorder, store: unreadableHolds{}, ghostChannel: func(string) string { return "ghost" }}

	if err := applier.Apply(crewGuild, voice.Change{UserID: "member", Muted: holdFlag(true)}); err == nil {
		t.Fatal("the missing record was not reported")
	}
	if len(recorder.changes) != 0 {
		t.Errorf("a mute was made without a record: %+v", recorder.changes)
	}
}

func TestReleasingLiftsOnlyWhatAUVCSet(t *testing.T) {
	for name, test := range map[string]struct {
		hold     sqlite.VoiceHold
		observed voice.Observed
		main     string
		ready    bool
		muted    *bool
		deafened *bool
		moveTo   string
	}{
		"muted and deafened by AUVC": {
			hold:     sqlite.VoiceHold{UserID: "m", Muted: true, Deafened: true},
			observed: voice.Observed{ChannelID: "main", Muted: true, Deafened: true},
			main:     "main", ready: true, muted: holdFlag(false), deafened: holdFlag(false),
		},
		"deafened by AUVC, muted by an administrator": {
			hold:     sqlite.VoiceHold{UserID: "m", Deafened: true},
			observed: voice.Observed{ChannelID: "main", Muted: true, Deafened: true},
			main:     "main", ready: true, deafened: holdFlag(false),
		},
		"already unmuted by somebody": {
			hold:     sqlite.VoiceHold{UserID: "m", Muted: true},
			observed: voice.Observed{ChannelID: "main"},
			main:     "main", ready: true,
		},
		"still in the ghost channel": {
			hold:     sqlite.VoiceHold{UserID: "m", GhostChannelID: "ghost"},
			observed: voice.Observed{ChannelID: "ghost"},
			main:     "main", ready: true, moveTo: "main",
		},
		"left the ghost channel on their own": {
			hold:     sqlite.VoiceHold{UserID: "m", GhostChannelID: "ghost"},
			observed: voice.Observed{ChannelID: "elsewhere"},
			main:     "main", ready: true,
		},
		"no main channel to return to": {
			hold:     sqlite.VoiceHold{UserID: "m", GhostChannelID: "ghost"},
			observed: voice.Observed{ChannelID: "ghost"},
			ready:    true,
		},
		"not in voice": {
			hold:     sqlite.VoiceHold{UserID: "m", Muted: true},
			observed: voice.Observed{},
			main:     "main", ready: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			change, ready := releaseChange(test.hold, test.observed, test.main)

			if ready != test.ready {
				t.Fatalf("ready = %t, want %t", ready, test.ready)
			}
			if (change.Muted == nil) != (test.muted == nil) || (change.Muted != nil && *change.Muted != *test.muted) {
				t.Errorf("muted = %v, want %v", change.Muted, test.muted)
			}
			if (change.Deafened == nil) != (test.deafened == nil) || (change.Deafened != nil && *change.Deafened != *test.deafened) {
				t.Errorf("deafened = %v, want %v", change.Deafened, test.deafened)
			}
			moved := ""
			if change.MoveTo != nil {
				moved = *change.MoveTo
			}
			if moved != test.moveTo {
				t.Errorf("move to %q, want %q", moved, test.moveTo)
			}
		})
	}
}

func TestWhatAPreviousRunLeftIsReleasedWhenTheMemberIsSeen(t *testing.T) {
	db := openCrewDB(t)
	if err := db.SaveVoiceHold(sqlite.VoiceHold{GuildID: crewGuild, UserID: "member",
		Muted: true, Deafened: true, GhostChannelID: "ghost"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	recorder := &holdRecorder{}
	bot := holdBot(t, db, recorder)

	bot.releaseStaleHolds(crewGuild, map[string]voice.Observed{
		"member": {ChannelID: "ghost", Muted: true, Deafened: true},
	})

	if len(recorder.changes) != 1 {
		t.Fatalf("want one release, got %+v", recorder.changes)
	}
	change := recorder.changes[0]
	if change.MoveTo == nil || *change.MoveTo != "main" || change.Muted == nil || *change.Muted ||
		change.Deafened == nil || *change.Deafened {
		t.Errorf("the release does not lift everything AUVC set: %+v", change)
	}
	if holds, _ := db.VoiceHolds(); len(holds) != 0 {
		t.Errorf("the released member is still on record: %+v", holds)
	}
}

func TestAMemberNotInVoiceIsReleasedWhenTheyJoin(t *testing.T) {
	db := openCrewDB(t)
	if err := db.SaveVoiceHold(sqlite.VoiceHold{GuildID: crewGuild, UserID: "member", Muted: true}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	recorder := &holdRecorder{}
	bot := holdBot(t, db, recorder)

	bot.releaseStaleHolds(crewGuild, map[string]voice.Observed{"member": {}})
	if len(recorder.changes) != 0 || len(bot.holds.pending(crewGuild)) != 1 {
		t.Fatalf("a member outside voice cannot be released yet: %+v", recorder.changes)
	}

	bot.releaseStaleHolds(crewGuild, map[string]voice.Observed{"member": {ChannelID: "main", Muted: true}})
	if len(recorder.changes) != 1 || recorder.changes[0].Muted == nil || *recorder.changes[0].Muted {
		t.Errorf("the member was not released on joining: %+v", recorder.changes)
	}
}

func TestAMemberAnAdministratorMutedIsLeftAlone(t *testing.T) {
	db := openCrewDB(t)
	recorder := &holdRecorder{}
	bot := holdBot(t, db, recorder)

	bot.releaseStaleHolds(crewGuild, map[string]voice.Observed{"member": {ChannelID: "main", Muted: true, Deafened: true}})

	if len(recorder.changes) != 0 {
		t.Errorf("a member AUVC never muted was changed: %+v", recorder.changes)
	}
}

// A restart in the middle of a round: the session keeps a living player muted,
// and nothing about them changes, so no voice edit ever claims them.
func TestTheRunningSessionKeepsItsPlayersEvenWhenNothingChanges(t *testing.T) {
	db := openCrewDB(t)
	if err := db.SaveVoiceHold(sqlite.VoiceHold{GuildID: crewGuild, UserID: "member", Muted: true, Deafened: true}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	recorder := &holdRecorder{}
	bot := holdBot(t, db, recorder)

	bot.claimManaged(crewGuild, map[string]voice.DesiredVoiceState{"member": {TargetChannelID: "main", Muted: true, Deafened: true}})
	bot.releaseStaleHolds(crewGuild, map[string]voice.Observed{"member": {ChannelID: "main", Muted: true, Deafened: true}})

	if len(recorder.changes) != 0 {
		t.Errorf("the recovery unmuted a player the running session keeps muted: %+v", recorder.changes)
	}
	if hold, _ := db.VoiceHold(crewGuild, "member"); !hold.Muted {
		t.Errorf("the session's record was dropped: %+v", hold)
	}
}

func TestAFailedReleaseIsTriedAgain(t *testing.T) {
	db := openCrewDB(t)
	if err := db.SaveVoiceHold(sqlite.VoiceHold{GuildID: crewGuild, UserID: "member", Muted: true}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	recorder := &holdRecorder{fail: errors.New("missing permissions")}
	bot := holdBot(t, db, recorder)
	seen := map[string]voice.Observed{"member": {ChannelID: "main", Muted: true}}

	bot.releaseStaleHolds(crewGuild, seen)
	if hold, _ := db.VoiceHold(crewGuild, "member"); !hold.Muted || len(bot.holds.pending(crewGuild)) != 1 {
		t.Fatalf("a failed release was given up: %+v", hold)
	}

	recorder.fail = nil
	bot.releaseStaleHolds(crewGuild, seen)
	if holds, _ := db.VoiceHolds(); len(holds) != 0 || len(recorder.changes) != 1 {
		t.Errorf("the second attempt did not release: holds %+v, changes %+v", holds, recorder.changes)
	}
}
