package bot

import (
	"log"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/voice"
)

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
