package bot

import (
	"log"
	"strings"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/session"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
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
			names = append(names, "**"+au.EscapeMarkdown(name)+"**")
		}
		lines = append(lines, language.Say(text.RoundImpostors, strings.Join(names, ", ")))
	}
	return strings.Join(lines, "\n")
}

// roundStats is every player's part in the round: who won follows from the
// winning side and whether the player was among the impostors. A player who
// left before the end did not finish the round and is not counted.
func roundStats(result protocol.RoundResult, players []session.GamePlayer) []sqlite.RoundPlayer {
	impostors := map[string]bool{}
	for _, name := range result.Impostors {
		impostors[strings.ToLower(name)] = true
	}
	counted := make([]sqlite.RoundPlayer, 0, len(players))
	for _, player := range players {
		if player.Disconnected || player.Name == "" {
			continue
		}
		impostor := impostors[strings.ToLower(player.Name)]
		counted = append(counted, sqlite.RoundPlayer{
			Name:     player.Name,
			Impostor: impostor,
			Won:      impostor == (result.Winner == protocol.WinnerImpostors),
		})
	}
	return counted
}

// finishRound counts the round for /au stats and, unless the server switched it
// off, posts its result.
func (bot *Bot) finishRound(guildID string, result protocol.RoundResult, players []session.GamePlayer) {
	if bot.AUVC == nil {
		return
	}
	if err := bot.AUVC.RecordRound(guildID, roundStats(result, players)); err != nil {
		log.Printf("could not count the round for guild %s: %v", guildID, err)
	}
	bot.postRoundResult(guildID, result)
}

// postRoundResult sends the round's result to the control channel. Names are
// the players' own, so nobody is mentioned, whatever a name contains.
func (bot *Bot) postRoundResult(guildID string, result protocol.RoundResult) {
	if bot.AUVC == nil || bot.PrimarySession == nil {
		return
	}
	config, err := bot.AUVC.GuildConfig(guildID)
	if err != nil || config.ControlTextChannelID == "" || !config.PostRoundResults {
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
