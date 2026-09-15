package bot

import (
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
)

// guildLanguage is the language for what everybody in a guild reads: the
// crewmate message and the notices in the text channel.
//
// A server that picked one with /au settings language gets that one. Every
// other server gets the server language set in Discord, which a server without
// the Community feature cannot change and which is then English. Replies only
// one member sees do not come through here: they use that member's language.
func (bot *Bot) guildLanguage(guildID string) text.Language {
	if bot.AUVC != nil {
		if config, err := bot.AUVC.GuildConfig(guildID); err == nil {
			if chosen, ok := text.Parse(config.Language); ok {
				return chosen
			}
		}
	}
	if bot.PrimarySession != nil && bot.PrimarySession.State != nil {
		if guild, err := bot.PrimarySession.State.Guild(guildID); err == nil && guild != nil {
			return text.FromDiscord(string(guild.PreferredLocale))
		}
	}
	return text.English
}
