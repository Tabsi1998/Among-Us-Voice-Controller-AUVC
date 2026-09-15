package bot

import (
	"errors"
	"log"
	"strings"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/voice"
	"github.com/bwmarrin/discordgo"
)

// errWouldDisconnect is the answer to a move into no channel. Discord reads an
// empty channel id as "remove this member from voice", which AUVC never wants.
var errWouldDisconnect = errors.New("refused a move to an empty channel: Discord would remove the member from voice")

// refuseDisconnect stops a change that would throw a member out of voice.
//
// The policy and the reconciler already never produce one. This is the last
// place before Discord, so a mistake anywhere upstream ends here as an error in
// the log instead of a player dropped from the call (#160).
func refuseDisconnect(change voice.Change) error {
	if change.MoveTo != nil && *change.MoveTo == "" {
		return errWouldDisconnect
	}
	return nil
}

// logVoiceChange writes down what AUVC is about to ask Discord for, so the bot
// log shows whether AUVC touched a member at the moment something happened to
// them.
func logVoiceChange(guildID string, change voice.Change) {
	log.Printf("Voice change for member %s in guild %s: %s", change.UserID, guildID, describeChange(change))
}

// describeChange says in words what one member edit asks for.
func describeChange(change voice.Change) string {
	var parts []string
	if change.MoveTo != nil {
		parts = append(parts, "move to channel "+*change.MoveTo)
	}
	if change.Muted != nil {
		if *change.Muted {
			parts = append(parts, "server mute")
		} else {
			parts = append(parts, "lift server mute")
		}
	}
	if change.Deafened != nil {
		if *change.Deafened {
			parts = append(parts, "server deafen")
		} else {
			parts = append(parts, "lift server deafen")
		}
	}
	if len(parts) == 0 {
		return "nothing"
	}
	return strings.Join(parts, ", ")
}

// describeVoiceMove says how a member's voice channel changed, and whether that
// is worth a log line: only a join, a leave or a switch that involves AUVC's
// main or ghost channel is. A change of mute or deafen within one channel is
// not a move, and AUVC's own edits are logged where they are made.
func describeVoiceMove(before, after string, config voice.Config) (string, bool) {
	ours := func(channelID string) bool {
		return channelID != "" && (channelID == config.MainChannelID || channelID == config.GhostChannelID)
	}
	if before == after || (!ours(before) && !ours(after)) {
		return "", false
	}
	switch {
	case before == "":
		return "joined voice channel " + after, true
	case after == "":
		return "left voice channel " + before, true
	default:
		return "switched from voice channel " + before + " to " + after, true
	}
}

// logVoiceMoves writes every join, leave and switch in AUVC's channels to the
// bot log. Together with logVoiceChange it answers the question #160 raised:
// was a player who dropped out of voice moved by AUVC, or did they leave
// without AUVC doing anything?
func (bot *Bot) logVoiceMoves() {
	bot.PrimarySession.AddHandler(func(_ *discordgo.Session, update *discordgo.VoiceStateUpdate) {
		if update.VoiceState == nil {
			return
		}
		before := ""
		if update.BeforeUpdate != nil {
			before = update.BeforeUpdate.ChannelID
		}
		if what, worth := describeVoiceMove(before, update.ChannelID, bot.voiceChannels(update.GuildID)); worth {
			log.Printf("Member %s %s in guild %s", update.UserID, what, update.GuildID)
		}
	})
}
