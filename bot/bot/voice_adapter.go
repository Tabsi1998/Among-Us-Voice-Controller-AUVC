package bot

import (
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/permission"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/voice"
	"github.com/bwmarrin/discordgo"
)

// observeVoiceStates maps a guild's voice states onto the view the reconciler
// compares against.
//
// Only the server mute and deafen are read. Discord also reports SelfMute and
// SelfDeaf, which are the member's own choice: treating those as the observed
// state would make the bot fight anyone who muted themselves, unmuting them
// every time it reconciled.
//
// Members who are not in a voice channel are absent from g.VoiceStates and
// therefore absent here, which is exactly what the reconciler expects.
func observeVoiceStates(guild *discordgo.Guild) map[string]voice.Observed {
	if guild == nil {
		return nil
	}

	observed := make(map[string]voice.Observed, len(guild.VoiceStates))
	for _, state := range guild.VoiceStates {
		if state == nil || state.UserID == "" {
			continue
		}
		observed[state.UserID] = voice.Observed{
			ChannelID: state.ChannelID,
			Muted:     state.Mute,
			Deafened:  state.Deaf,
		}
	}
	return observed
}

// memberParams turns one change into the member edit Discord expects.
//
// Every field stays nil unless the change asks for it, so a member edit never
// carries a value the player already has. That matters beyond tidiness: sending
// an empty ChannelID would disconnect the member from voice entirely.
func memberParams(change voice.Change) *discordgo.GuildMemberParams {
	return &discordgo.GuildMemberParams{
		ChannelID: change.MoveTo,
		Mute:      change.Muted,
		Deaf:      change.Deafened,
	}
}

// discordApplier applies reconciler changes through the Discord API.
type discordApplier struct {
	session *discordgo.Session
}

func (a discordApplier) Apply(guildID string, change voice.Change) error {
	if err := refuseDisconnect(change); err != nil {
		return err
	}
	logVoiceChange(guildID, change)
	_, err := a.session.GuildMemberEdit(guildID, change.UserID, memberParams(change))
	return err
}

// voiceChannelPermissions collects what AUVC may actually do on the configured
// channels, so pkg/permission can report anything missing before a round starts
// rather than failing per player in the middle of one.
//
// UserChannelPermissions resolves roles and channel overwrites, which is what
// the requirements mean by effective permissions. A channel that cannot be
// resolved is reported with no permissions rather than skipped: an
// unreachable channel is a real problem, and silence would hide it.
func (bot *Bot) voiceChannelPermissions(mainChannelID, ghostChannelID string) []permission.Channel {
	describe := []struct {
		purpose   text.Key
		channelID string
	}{
		{text.PurposeMainChannel, mainChannelID},
		{text.PurposeGhostChannel, ghostChannelID},
	}

	channels := make([]permission.Channel, 0, len(describe))
	for _, entry := range describe {
		if entry.channelID == "" {
			continue
		}

		effective, err := bot.PrimarySession.UserChannelPermissions(
			bot.PrimarySession.State.User.ID, entry.channelID)
		if err != nil {
			effective = 0
		}

		channels = append(channels, permission.Channel{
			Purpose:   entry.purpose,
			ID:        entry.channelID,
			Effective: effective,
		})
	}
	return channels
}
