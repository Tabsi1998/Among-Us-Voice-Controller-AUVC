package bot

import (
	"log"
	"sort"
)

// RunningGuilds returns the guilds whose session is managing voice right now,
// in a stable order.
func (c *CaptureSessions) RunningGuilds() []string {
	c.mu.Lock()
	sessions := make(map[string]*guildSession, len(c.sessions))
	for guildID, guild := range c.sessions {
		sessions[guildID] = guild
	}
	c.mu.Unlock()

	var running []string
	for guildID, guild := range sessions {
		guild.mu.Lock()
		mode := guild.mode
		guild.mu.Unlock()

		if mode == Running {
			running = append(running, guildID)
		}
	}

	sort.Strings(running)
	return running
}

// ReleaseAll unmutes, undeafens and returns to the main channel every player AUVC
// manages, in every guild with a running session.
//
// It is what a stopping bot owes the people in voice. Nothing in Discord ever
// lifts a server mute on its own, so a bot that shut down mid-round would leave
// the living muted until an administrator noticed. That matters most for the
// Windows app, where closing the app is how the bot stops.
func (bot *Bot) ReleaseAll() {
	for _, guildID := range bot.CaptureSessions.RunningGuilds() {
		if err := bot.releaseEveryone(guildID); err != nil {
			log.Printf("could not release players in guild %s while stopping: %v", guildID, err)
		}
	}
}
