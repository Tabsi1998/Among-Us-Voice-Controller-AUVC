package command

import (
	"github.com/automuteus/automuteus/v8/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type NewStatus int

const (
	NewSuccess NewStatus = iota
	NewNoVoiceChannel
)

type NewInfo struct {
	Hyperlink    string
	MinimalURL   string
	ApiHyperlink string
	ConnectCode  string
	ActiveGames  int64
}

var New = discordgo.ApplicationCommand{
	Name:        "new",
	Description: "Start a new game",
}

func NewResponse(status NewStatus, info NewInfo, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	var content string
	var embeds []*discordgo.MessageEmbed
	flags := discordgo.MessageFlagsEphemeral // private message by default

	switch status {
	case NewSuccess:
		content = sett.LocalizeMessage(&i18n.Message{
			ID: "commands.new.success",
			Other: "Paste this link into your web browser:\n <{{.hyperlink}}>\n" +
				"or click [here]({{.apiHyperlink}})\n\n" +
				"If the URL doesn't work, you may need to run the capture program first, and then try again.\n\n" +
				"Don't have the capture installed? Latest version [here]({{.downloadURL}})\n\nTo link your capture manually:",
		},
			map[string]interface{}{
				"hyperlink":    info.Hyperlink,
				"apiHyperlink": info.ApiHyperlink,
				"downloadURL":  CaptureDownloadURL,
			})
		embeds = []*discordgo.MessageEmbed{
			{
				Fields: []*discordgo.MessageEmbedField{
					{
						Name: sett.LocalizeMessage(&i18n.Message{
							ID:    "commands.new.success.url",
							Other: "URL",
						}),
						Value:  info.MinimalURL,
						Inline: true,
					},
					{
						Name: sett.LocalizeMessage(&i18n.Message{
							ID:    "commands.new.success.code",
							Other: "Code",
						}),
						Value:  info.ConnectCode,
						Inline: true,
					},
				},
			},
		}
	case NewNoVoiceChannel:
		content = sett.LocalizeMessage(&i18n.Message{
			ID:    "commands.new.nochannel",
			Other: "Please join a voice channel before starting a match!",
		})
	}
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   flags,
			Content: content,
			Embeds:  embeds,
		},
	}
}
