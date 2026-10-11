package bot

import (
	"log"
	"strings"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
	"github.com/bwmarrin/discordgo"
)

// roundReasons names how a round ended. A reason this build does not know is
// left out of the post rather than guessed.
var roundReasons = map[string]text.Key{
	protocol.ReasonVote:       text.RoundByVote,
	protocol.ReasonTasks:      text.RoundByTasks,
	protocol.ReasonKill:       text.RoundByKill,
	protocol.ReasonSabotage:   text.RoundBySabotage,
	protocol.ReasonDisconnect: text.RoundByDisconnect,
	protocol.ReasonTimer:      text.RoundByTimer,
}

// roundResultNotice is what the control channel reads once a round is over: who
// won, how, and who the impostors were. It is only ever posted after the end,
// so it gives nothing away.
func roundResultNotice(language text.Language, result protocol.RoundResult) string {
	lines := []string{language.Say(text.RoundWonCrewmates)}
	if result.Winner == protocol.WinnerImpostors {
		lines[0] = language.Say(text.RoundWonImpostors)
	}
	if key, ok := roundReasons[result.Reason]; ok {
		lines[0] += " " + language.Say(key)
	}

	if len(result.Impostors) == 0 {
		lines = append(lines, language.Say(text.RoundImpostorsNone))
	} else {
		names := make([]string, 0, len(result.Impostors))
		for _, name := range result.Impostors {
			names = append(names, "**"+escapeMarkdown(name)+"**")
		}
		lines = append(lines, language.Say(text.RoundImpostors, strings.Join(names, ", ")))
	}
	return strings.Join(lines, "\n")
}

// escapeMarkdown keeps a player's name from formatting the post. Among Us
// allows most characters in a name, Discord reads several of them as markup.
func escapeMarkdown(name string) string {
	var escaped strings.Builder
	for _, r := range name {
		if strings.ContainsRune("\\*_~`|>#[]()-", r) {
			escaped.WriteRune('\\')
		}
		escaped.WriteRune(r)
	}
	return escaped.String()
}

// postRoundResult sends the round's result to the control channel. Names are
// the players' own, so nobody is mentioned, whatever a name contains.
func (bot *Bot) postRoundResult(guildID string, result protocol.RoundResult) {
	if bot.AUVC == nil || bot.PrimarySession == nil {
		return
	}
	config, err := bot.AUVC.GuildConfig(guildID)
	if err != nil || config.ControlTextChannelID == "" {
		return
	}

	message := &discordgo.MessageSend{
		Content:         roundResultNotice(bot.guildLanguage(guildID), result),
		AllowedMentions: &discordgo.MessageAllowedMentions{Parse: []discordgo.AllowedMentionType{}},
	}
	if _, err := bot.PrimarySession.ChannelMessageSendComplex(config.ControlTextChannelID, message); err != nil {
		log.Printf("could not post the round result for guild %s: %v", guildID, err)
	}
}
