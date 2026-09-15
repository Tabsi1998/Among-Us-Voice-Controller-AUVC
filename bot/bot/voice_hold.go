package bot

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/voice"
	"github.com/bwmarrin/discordgo"
)

// HoldStore remembers the voice changes AUVC made that are still in effect.
type HoldStore interface {
	VoiceHold(guildID, userID string) (sqlite.VoiceHold, error)
	SaveVoiceHold(hold sqlite.VoiceHold) error
	VoiceHolds() ([]sqlite.VoiceHold, error)
}

// holdingApplier writes down a mute, a deafen or a move into the ghost channel
// before making it, and forgets it once it has been lifted.
//
// The order is the point. A crash between Discord applying a mute and the
// record being written would leave exactly the member nobody can find again. A
// record without the mute behind it costs one unnecessary release at most.
type holdingApplier struct {
	inner        voice.Applier
	store        HoldStore
	ghostChannel func(guildID string) string
	// claim tells the recovery that this run now manages the member, so a
	// record from before the restart is no longer its business.
	claim func(guildID, userID string)
}

func (a holdingApplier) Apply(guildID string, change voice.Change) error {
	if a.claim != nil {
		a.claim(guildID, change.UserID)
	}

	hold, err := a.store.VoiceHold(guildID, change.UserID)
	if err != nil {
		// Without a record a crash could strand the member, so the change is
		// not made. Not muting somebody is the failure that harms nobody.
		return fmt.Errorf("the voice change could not be recorded first: %w", err)
	}

	intended := hold
	if change.Muted != nil && *change.Muted {
		intended.Muted = true
	}
	if change.Deafened != nil && *change.Deafened {
		intended.Deafened = true
	}
	if ghost := a.ghostChannel(guildID); change.MoveTo != nil && ghost != "" && *change.MoveTo == ghost {
		intended.GhostChannelID = ghost
	}
	if intended != hold {
		if err := a.store.SaveVoiceHold(intended); err != nil {
			return fmt.Errorf("the voice change could not be recorded first: %w", err)
		}
	}

	if err := a.inner.Apply(guildID, change); err != nil {
		return err
	}

	lifted := intended
	if change.Muted != nil && !*change.Muted {
		lifted.Muted = false
	}
	if change.Deafened != nil && !*change.Deafened {
		lifted.Deafened = false
	}
	if change.MoveTo != nil && *change.MoveTo != intended.GhostChannelID {
		lifted.GhostChannelID = ""
	}
	if lifted != intended {
		if err := a.store.SaveVoiceHold(lifted); err != nil {
			// The change was made. A stale record means one release too many
			// after the next restart, never a member left muted.
			log.Printf("Could not forget a lifted voice change for %s in guild %s: %v", change.UserID, guildID, err)
		}
	}
	return nil
}

// holdRecovery tracks the records a previous run left behind.
type holdRecovery struct {
	mu    sync.Mutex
	stale map[string]map[string]sqlite.VoiceHold
}

func newHoldRecovery(holds []sqlite.VoiceHold) *holdRecovery {
	recovery := &holdRecovery{stale: map[string]map[string]sqlite.VoiceHold{}}
	for _, hold := range holds {
		recovery.giveBack(hold)
	}
	return recovery
}

// claim removes a member from the recovery: the running session manages them.
func (r *holdRecovery) claim(guildID, userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.stale[guildID], userID)
}

func (r *holdRecovery) take(guildID, userID string) (sqlite.VoiceHold, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	hold, ok := r.stale[guildID][userID]
	if ok {
		delete(r.stale[guildID], userID)
	}
	return hold, ok
}

func (r *holdRecovery) giveBack(hold sqlite.VoiceHold) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stale[hold.GuildID] == nil {
		r.stale[hold.GuildID] = map[string]sqlite.VoiceHold{}
	}
	r.stale[hold.GuildID][hold.UserID] = hold
}

func (r *holdRecovery) pending(guildID string) []sqlite.VoiceHold {
	r.mu.Lock()
	defer r.mu.Unlock()
	holds := make([]sqlite.VoiceHold, 0, len(r.stale[guildID]))
	for _, hold := range r.stale[guildID] {
		holds = append(holds, hold)
	}
	return holds
}

// releaseChange is what lifts a record from a previous run, given the member as
// Discord reports them now, and whether it can be made at all.
//
// Only what AUVC set is lifted. A member AUVC deafened but an administrator
// muted keeps the mute. Discord refuses to change the voice of somebody who is
// not in a voice channel, so such a member waits until they join one.
func releaseChange(hold sqlite.VoiceHold, observed voice.Observed, mainChannelID string) (voice.Change, bool) {
	change := voice.Change{UserID: hold.UserID}
	if !observed.Connected() {
		return change, false
	}

	if hold.Muted && observed.Muted {
		unmuted := false
		change.Muted = &unmuted
	}
	if hold.Deafened && observed.Deafened {
		undeafened := false
		change.Deafened = &undeafened
	}
	if hold.GhostChannelID != "" && observed.ChannelID == hold.GhostChannelID &&
		mainChannelID != "" && mainChannelID != observed.ChannelID {
		target := mainChannelID
		change.MoveTo = &target
	}
	return change, true
}

// AttachVoiceHolds records AUVC's voice changes from now on and releases what a
// previous run left behind, for every member as soon as Discord shows them in a
// voice channel.
func (bot *Bot) AttachVoiceHolds(store HoldStore) error {
	holds, err := store.VoiceHolds()
	if err != nil {
		return fmt.Errorf("read the voice changes a previous run left in place: %w", err)
	}

	bot.holdStore = store
	bot.holds = newHoldRecovery(holds)
	bot.voiceApplier = discordApplier{session: bot.PrimarySession}
	bot.Reconciler = voice.NewReconciler(holdingApplier{
		inner:        bot.voiceApplier,
		store:        store,
		ghostChannel: func(guildID string) string { return bot.voiceChannels(guildID).GhostChannelID },
		claim:        bot.holds.claim,
	})

	if len(holds) > 0 {
		log.Printf("AUVC did not stop cleanly last time: releasing %d member(s) it left muted, deafened "+
			"or in the ghost channel as they are seen in voice", len(holds))
	}

	bot.PrimarySession.AddHandler(func(_ *discordgo.Session, created *discordgo.GuildCreate) {
		if created.Guild != nil {
			bot.releaseStaleHolds(created.Guild.ID, observeVoiceStates(created.Guild))
		}
	})
	bot.PrimarySession.AddHandler(func(_ *discordgo.Session, update *discordgo.VoiceStateUpdate) {
		if update.VoiceState == nil {
			return
		}
		bot.releaseStaleHolds(update.GuildID, map[string]voice.Observed{
			update.UserID: {ChannelID: update.ChannelID, Muted: update.Mute, Deafened: update.Deaf},
		})
	})

	bot.PrimarySession.State.RLock()
	guilds := append([]*discordgo.Guild(nil), bot.PrimarySession.State.Guilds...)
	bot.PrimarySession.State.RUnlock()
	for _, guild := range guilds {
		bot.releaseStaleHolds(guild.ID, observeVoiceStates(guild))
	}
	return nil
}

// releaseStaleHolds releases the records a previous run left for the members
// seen here.
func (bot *Bot) releaseStaleHolds(guildID string, observed map[string]voice.Observed) {
	if bot.holds == nil || bot.Reconciler == nil || guildID == "" {
		return
	}
	for _, hold := range bot.holds.pending(guildID) {
		have, seen := observed[hold.UserID]
		if !seen {
			continue
		}
		bot.Reconciler.Exclusive(guildID, func() { bot.releaseStaleHold(hold.GuildID, hold.UserID, have) })
	}
}

// releaseStaleHold lifts one record. The caller holds the guild's
// reconciliation lock, so the running session cannot claim the member halfway.
func (bot *Bot) releaseStaleHold(guildID, userID string, observed voice.Observed) {
	hold, ok := bot.holds.take(guildID, userID)
	if !ok {
		// The running session took the member over, or another event was faster.
		return
	}

	change, ready := releaseChange(hold, observed, bot.voiceChannels(guildID).MainChannelID)
	if !ready {
		bot.holds.giveBack(hold)
		return
	}

	if !change.Empty() {
		if err := bot.voiceApplier.Apply(guildID, change); err != nil {
			log.Printf("Could not release member %s in guild %s after AUVC restarted; retrying when they are seen again: %v",
				userID, guildID, err)
			bot.holds.giveBack(hold)
			return
		}
		log.Printf("Released member %s in guild %s, whom AUVC had left %s when it last stopped", userID, guildID, describeHold(hold))
	}

	if err := bot.holdStore.SaveVoiceHold(sqlite.VoiceHold{GuildID: guildID, UserID: userID}); err != nil {
		log.Printf("Could not forget the released voice change for %s in guild %s: %v", userID, guildID, err)
	}
}

// claimManaged tells the recovery that the running session manages these
// members, including those whose voice already matches and so see no change.
// Without it, a restart in the middle of a round could briefly unmute a living
// player the session correctly keeps muted.
func (bot *Bot) claimManaged(guildID string, desired map[string]voice.DesiredVoiceState) {
	if bot.holds == nil {
		return
	}
	for userID := range desired {
		bot.holds.claim(guildID, userID)
	}
}

// voiceChannels reads a guild's channels for the recovery. An unreadable
// configuration yields none, which lifts mutes but moves nobody.
func (bot *Bot) voiceChannels(guildID string) voice.Config {
	if bot.AUVC == nil {
		return voice.Config{}
	}
	config, _, err := bot.AUVC.VoiceConfig(guildID)
	if err != nil {
		return voice.Config{}
	}
	return config
}

// VoiceHoldOf reports what AUVC holds on a member right now: a server mute, a
// server deafen, a move into the ghost channel. It is empty for a member AUVC
// has left alone, and for a bot that does not record holds.
func (bot *Bot) VoiceHoldOf(guildID, userID string) sqlite.VoiceHold {
	empty := sqlite.VoiceHold{GuildID: guildID, UserID: userID}
	if bot.holdStore == nil || userID == "" {
		return empty
	}
	hold, err := bot.holdStore.VoiceHold(guildID, userID)
	if err != nil {
		log.Printf("Could not read what AUVC holds on member %s in guild %s: %v", userID, guildID, err)
		return empty
	}
	return hold
}

func describeHold(hold sqlite.VoiceHold) string {
	var parts []string
	if hold.Muted {
		parts = append(parts, "muted")
	}
	if hold.Deafened {
		parts = append(parts, "deafened")
	}
	if hold.GhostChannelID != "" {
		parts = append(parts, "in the ghost channel")
	}
	return strings.Join(parts, ", ")
}
