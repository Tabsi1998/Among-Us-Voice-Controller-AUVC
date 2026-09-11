package bot

import (
	"context"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/permission"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/settings"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/task"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/voice"
	"github.com/bsm/redislock"
	"github.com/bwmarrin/discordgo"
	"log"
	"strconv"
	"time"
)

type HandlePriority int

const (
	NoPriority    HandlePriority = 0
	AlivePriority HandlePriority = 1
	DeadPriority  HandlePriority = 2
)

func (bot *Bot) applyToSingle(dgs *GameState, userID string, mute, deaf bool) error {
	uid, _ := strconv.ParseUint(userID, 10, 64)
	req := task.UserModifyRequest{
		Users: []task.UserModify{
			{
				UserID: uid,
				Mute:   mute,
				Deaf:   deaf,
			},
		},
	}
	// nil lock because this is an override; we don't care about legitimately obtaining the lock
	return bot.TokenProvider.ModifyUsers(dgs.GuildID, dgs.ConnectCode, req, nil)
}

func (bot *Bot) applyToAll(dgs *GameState, mute, deaf bool) error {
	g, err := bot.PrimarySession.State.Guild(dgs.GuildID)
	if err != nil {
		return err
	}

	var users []task.UserModify

	for _, voiceState := range g.VoiceStates {
		userData, err := dgs.GetUser(voiceState.UserID)
		if err != nil {
			// the User doesn't exist in our userdata cache; add them
			added := false
			userData, added = dgs.checkCacheAndAddUser(g, bot.PrimarySession, voiceState.UserID)
			if !added {
				continue
			}
		}

		tracked := voiceState.ChannelID != "" && dgs.VoiceChannel == voiceState.ChannelID

		_, linked := dgs.GameData.GetByName(userData.InGameName)
		// only actually tracked if we're in a tracked channel AND linked to a player
		tracked = tracked && linked

		if tracked {
			uid, _ := strconv.ParseUint(userData.User.UserID, 10, 64)
			users = append(users, task.UserModify{
				UserID: uid,
				Mute:   mute,
				Deaf:   deaf,
			})
			log.Println("Forcibly applying mute/deaf to " + userData.User.UserID)
		}
	}
	if len(users) > 0 {
		req := task.UserModifyRequest{
			Users: users,
		}
		// nil lock because this is an override; we don't care about legitimately obtaining the lock
		return bot.TokenProvider.ModifyUsers(dgs.GuildID, dgs.ConnectCode, req, nil)
	}
	return nil
}

// handleTrackedMembers moves/mutes players according to the current game state
func (bot *Bot) handleTrackedMembers(sess *discordgo.Session, sett *settings.GuildSettings, delay int, handlePriority HandlePriority, gsr GameStateRequest) {

	lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	for lock == nil {
		lock, dgs = bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
	}

	g, err := sess.State.Guild(dgs.GuildID)

	if err != nil || g == nil {
		lock.Release(ctx)
		return
	}

	if bot.reconcileVoice(g, dgs, lock, gsr, delay) {
		return
	}

	var users []task.UserModify

	priorityRequests := 0
	for _, voiceState := range g.VoiceStates {
		userData, err := dgs.GetUser(voiceState.UserID)
		if err != nil {
			// the User doesn't exist in our userdata cache; add them
			added := false
			userData, added = dgs.checkCacheAndAddUser(g, sess, voiceState.UserID)
			if !added {
				continue
			}
		}

		tracked := voiceState.ChannelID != "" && dgs.VoiceChannel == voiceState.ChannelID

		auData, found := dgs.GameData.GetByName(userData.InGameName)
		// only actually tracked if we're in a tracked channel AND linked to a player
		var isAlive bool

		// only actually tracked if we're in a tracked channel AND linked to a player
		if !sett.GetMuteSpectator() {
			tracked = tracked && found
			isAlive = auData.IsAlive
		} else {
			if !found {
				// we just assume the spectator is dead
				isAlive = false
			} else {
				isAlive = auData.IsAlive
			}
		}
		shouldMute, shouldDeaf := sett.GetVoiceState(isAlive, tracked, dgs.GameData.GetPhase())

		incorrectMuteDeafenState := shouldMute != userData.ShouldBeMute || shouldDeaf != userData.ShouldBeDeaf

		// only issue a change if the User isn't in the right state already
		// nicksmatch can only be false if the in-game data is != nil, so the reference to .audata below is safe
		// check the userdata is linked here to not accidentally undeafen music bots, for example
		if incorrectMuteDeafenState && (found || sett.GetMuteSpectator()) {
			uid, _ := strconv.ParseUint(userData.User.UserID, 10, 64)
			userModify := task.UserModify{
				UserID: uid,
				Mute:   shouldMute,
				Deaf:   shouldDeaf,
			}

			if handlePriority != NoPriority && ((handlePriority == AlivePriority && isAlive) || (handlePriority == DeadPriority && !isAlive)) {
				users = append([]task.UserModify{userModify}, users...)
				priorityRequests++ // counter of how many elements on the front of the arr should be sent first
			} else {
				users = append(users, userModify)
			}
			userData.SetShouldBeMuteDeaf(shouldMute, shouldDeaf)
			dgs.UpdateUserData(userData.User.UserID, userData)
		}
	}

	// we relinquish the lock while we wait
	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	voiceLock := bot.RedisInterface.LockVoiceChanges(dgs.ConnectCode, time.Second*time.Duration(delay+1))

	if delay > 0 {
		log.Printf("Sleeping for %d seconds before applying changes to users\n", delay)
		time.Sleep(time.Second * time.Duration(delay))
	}

	if dgs.Running && len(users) > 0 {

		if priorityRequests > 0 {
			req := task.UserModifyRequest{
				Users: users[:priorityRequests],
			}
			// no lock; we're not done yet
			err := bot.issueMutesAndRecord(dgs.GuildID, dgs.ConnectCode, req, nil)
			if err != nil {
				log.Println(err)
			} else {
				log.Println("Successfully finished issuing high priority mutes")
			}
			rem := users[priorityRequests:]
			if len(rem) > 0 {
				req = task.UserModifyRequest{
					Users: rem,
				}
				err := bot.issueMutesAndRecord(dgs.GuildID, dgs.ConnectCode, req, voiceLock)
				if err != nil {
					log.Println(err)
				}
			} else if voiceLock != nil {
				voiceLock.Release(context.Background())
			}
		} else {
			// no priority; issue all at once
			log.Println("Issuing mutes/deafens with no particular priority")
			req := task.UserModifyRequest{
				Users: users,
			}
			err := bot.issueMutesAndRecord(dgs.GuildID, dgs.ConnectCode, req, voiceLock)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func (bot *Bot) issueMutesAndRecord(guildID, connectCode string, req task.UserModifyRequest, lock *redislock.Lock) error {
	return bot.TokenProvider.ModifyUsers(guildID, connectCode, req, lock)
}

// voicePolicyConfig reports a guild's voice configuration and whether AUVC
// should take charge of voice at all.
//
// Taking over requires an administrator to have configured AUVC: a guild that
// has not run /au setup channels has no ghost channel to move anyone into, so
// the legacy voice rules stay in charge rather than the bot silently doing
// nothing. That fallback disappears with the legacy settings in phase 14.
//
// A configuration that cannot be read is treated the same way. Voice is the
// visible half of a round, so an unreachable database must degrade to the old
// behaviour instead of leaving players muted with no way out.
func (bot *Bot) voicePolicyConfig(guildID string) (voice.Config, bool) {
	if bot.Reconciler == nil {
		return voice.Config{}, false
	}

	config, ready, err := bot.AUVC.VoiceConfig(guildID)
	if err != nil {
		log.Println("Could not read the AUVC configuration, using the legacy voice rules:", err)
		return voice.Config{}, false
	}
	return config, ready
}

// reconcileVoice applies the AUVC voice policy and reports whether it handled
// the guild. A false result means the caller has to fall back to the legacy
// voice rules, and that the game state lock is still held.
//
// Unlike the legacy path, this compares against what Discord reports rather
// than against the intent recorded on the last run, so a mute that failed or a
// player somebody unmuted by hand is corrected on the next reconciliation.
//
// Changes go out over the primary session rather than the token provider. The
// provider exists to spread Discord rate limits across several bot tokens,
// which is part of the hosted AutoMuteUs setup that phase 14 removes; a
// self-hosted AUVC runs one token and has nothing to spread.
func (bot *Bot) reconcileVoice(g *discordgo.Guild, dgs *GameState, lock *redislock.Lock, gsr GameStateRequest, delay int) bool {
	config, ready := bot.voicePolicyConfig(dgs.GuildID)
	if !ready {
		return false
	}

	// Read everything needed from the game state before releasing it.
	running := dgs.Running
	state := dgs.SessionState()

	// Release the game state before sleeping or touching Discord: holding it
	// across an API call blocks every other handler for this guild.
	bot.RedisInterface.SetDiscordGameState(dgs, lock)

	if !running {
		return true
	}

	if delay > 0 {
		log.Printf("Sleeping for %d seconds before applying voice changes\n", delay)
		time.Sleep(time.Second * time.Duration(delay))

		// The delay gives players a moment, it does not freeze the round. A
		// death or a phase change during it is handled by its own call, and
		// applying the state from before the sleep would undo that work. Reading
		// the state again is safe precisely because the reconciler is
		// idempotent: whatever the other call already applied produces no change
		// here.
		if current := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr); current != nil {
			if !current.Running {
				return true
			}
			state = current.SessionState()
		}
	}

	if err := bot.Reconciler.Reconcile(g.ID, observeVoiceStates(g), voice.Desired(state, config)); err != nil {
		log.Println("Applying voice changes failed:", err)

		// A permission problem surfaces as an opaque API error per player.
		// Naming it turns that into something an administrator can act on.
		if summary := permission.Summary(permission.Audit(
			bot.voiceChannelPermissions(config.MainChannelID, config.GhostChannelID)...,
		)); summary != "" {
			log.Print(summary)
		}
	}
	return true
}
