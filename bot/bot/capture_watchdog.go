package bot

import (
	"fmt"
	"log"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/voice"
)

// watchdogInterval is how often the timeout is checked.
//
// The configured timeout is ten seconds at the shortest, so checking once a
// second finds a stall within roughly one tick of when it happened without
// waking up for no reason.
const watchdogInterval = time.Second

// Stalled reports the guilds whose capture has gone quiet for longer than
// their configured timeout.
//
// A session that is not running is never stalled. Nothing is being applied, so
// there is nothing to fail open from, and reporting it would mean warning a
// guild about a capture it has not asked to use.
func (c *CaptureSessions) Stalled(now time.Time, timeout func(guildID string) time.Duration) []string {
	c.mu.Lock()
	guilds := make([]string, 0, len(c.sessions))
	for guildID := range c.sessions {
		guilds = append(guilds, guildID)
	}
	c.mu.Unlock()

	var stalled []string
	for _, guildID := range guilds {
		guild := c.forGuild(guildID)

		guild.mu.Lock()
		quiet := guild.mode == Running && !guild.stalled &&
			!guild.lastSeen.IsZero() && now.Sub(guild.lastSeen) > timeout(guildID)
		guild.mu.Unlock()

		if quiet {
			stalled = append(stalled, guildID)
		}
	}
	return stalled
}

// markStalled records that the capture timeout interrupted a guild, and reports
// whether this call is the one that did it.
//
// The report matters: the watchdog runs every second and a capture stays gone
// for as long as it is gone, so without it a single crash would warn a channel
// once a second until somebody noticed.
func (c *CaptureSessions) markStalled(guildID string) bool {
	guild := c.forGuild(guildID)

	guild.mu.Lock()
	defer guild.mu.Unlock()

	if guild.stalled {
		return false
	}
	guild.stalled = true
	guild.stalledFrom = guild.mode
	guild.mode = Paused
	return true
}

// recovered clears a stall and returns the mode the session should go back to,
// or Stopped and false when the session was not stalled.
func (c *CaptureSessions) recovered(guildID string) (Mode, bool) {
	guild := c.forGuild(guildID)

	guild.mu.Lock()
	defer guild.mu.Unlock()

	if !guild.stalled {
		return Stopped, false
	}
	guild.stalled = false
	guild.mode = guild.stalledFrom
	return guild.stalledFrom, true
}

// seen records that capture spoke.
func (c *CaptureSessions) seen(guildID string, at time.Time) {
	guild := c.forGuild(guildID)

	guild.mu.Lock()
	defer guild.mu.Unlock()

	guild.lastSeen = at
}

// WatchCapture runs the capture timeout until the returned function is called.
//
// The fail-safe exists because a capture that dies mid-round leaves every
// living player server-muted and deafened, with no way out that does not
// involve an administrator. Nothing in Discord expires a server mute on its
// own, so the bot has to notice and undo it.
func (bot *Bot) WatchCapture() func() {
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(watchdogInterval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case now := <-ticker.C:
				bot.checkCaptureTimeouts(now)
			}
		}
	}()

	return func() { close(done) }
}

// checkCaptureTimeouts runs one pass of the fail-safe.
func (bot *Bot) checkCaptureTimeouts(now time.Time) {
	for _, guildID := range bot.CaptureSessions.Stalled(now, bot.captureTimeout) {
		if !bot.CaptureSessions.markStalled(guildID) {
			continue
		}
		bot.failSafe(guildID)
	}
}

// captureTimeout reads a guild's configured timeout.
//
// An unreadable configuration falls back to the documented default rather than
// to no timeout at all. A fail-safe that switches itself off when a database
// blinks is not one.
func (bot *Bot) captureTimeout(guildID string) time.Duration {
	config, err := bot.AUVC.GuildConfig(guildID)
	if err != nil || config.CaptureTimeoutSeconds <= 0 {
		return time.Duration(au.DefaultCaptureTimeoutSeconds) * time.Second
	}
	return time.Duration(config.CaptureTimeoutSeconds) * time.Second
}

// failSafe applies what a guild asked for when its capture goes quiet.
func (bot *Bot) failSafe(guildID string) {
	action := au.CaptureTimeoutFailOpen
	if config, err := bot.AUVC.GuildConfig(guildID); err == nil && config.CaptureTimeoutAction != "" {
		action = config.CaptureTimeoutAction
	}

	log.Printf("capture for guild %s has gone quiet; applying %s", guildID, action)

	warning := "⚠️ The capture app stopped responding."
	switch action {
	case au.CaptureTimeoutPause:
		warning += " The session is paused and players have been left exactly as they are, " +
			"because this server is configured with `capture_timeout_action: pause`. " +
			"Run `/au session stop` to release everyone."
	default:
		// Fail open. Leaving a round muted because the bot lost its eyes is
		// the worst outcome available: nothing in Discord expires a server
		// mute, so the players would stay stuck until somebody noticed.
		if err := bot.releaseEveryone(guildID); err != nil {
			log.Printf("could not release players for guild %s: %v", guildID, err)
			warning += " AUVC could not unmute everyone automatically: " + err.Error()
		} else {
			warning += " Everyone has been unmuted and returned to the main channel."
		}
		warning += " The session is paused and resumes on its own when capture comes back."
	}

	bot.warnGuild(guildID, warning)
}

// captureReturned restores a session the timeout had interrupted.
func (bot *Bot) captureReturned(guildID string) {
	restored, wasStalled := bot.CaptureSessions.recovered(guildID)
	if !wasStalled {
		return
	}

	log.Printf("capture for guild %s is back; restoring %s", guildID, restored)
	bot.warnGuild(guildID, "✅ The capture app is back. AUVC is following the game again.")
}

// warnGuild sends a message to the configured control channel.
//
// A guild that has not configured one gets nothing, which is the right failure:
// posting a warning into whatever channel happens to be available is how a bot
// ends up announcing itself somewhere it is not wanted.
func (bot *Bot) warnGuild(guildID, message string) {
	config, err := bot.AUVC.GuildConfig(guildID)
	if err != nil || config.ControlTextChannelID == "" {
		return
	}

	if _, err := bot.PrimarySession.ChannelMessageSend(config.ControlTextChannelID, message); err != nil {
		log.Printf("could not warn guild %s: %v", guildID, err)
	}
}

// releaseEveryone unmutes, undeafens and returns every managed player to the
// main channel.
//
// It runs through the ordinary voice policy with the phase forced to Menu
// rather than through a path that unmutes people directly. The policy already
// knows what "no round is running" looks like, and a second implementation of
// that is a second thing that can disagree with the first. /au session stop
// takes the same route.
func (bot *Bot) releaseEveryone(guildID string) error {
	config, ready := bot.voicePolicyConfig(guildID)
	if !ready {
		return nil
	}

	resolve, err := bot.linkResolver(guildID)
	if err != nil {
		return err
	}

	guild := bot.CaptureSessions.forGuild(guildID)
	guild.mu.Lock()
	state := guild.live.Project(resolve)
	guild.mu.Unlock()
	state.Phase = game.MENU

	discordGuild, err := bot.PrimarySession.State.Guild(guildID)
	if err != nil || discordGuild == nil {
		return fmt.Errorf("discord guild %s is unavailable: %w", guildID, err)
	}

	return bot.Reconciler.Reconcile(guildID, observeVoiceStates(discordGuild), voice.Desired(state, config))
}
