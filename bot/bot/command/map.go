package command

import (
	"bytes"
	"os"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

var Map = discordgo.ApplicationCommand{
	Name:        "map",
	Description: "View Among Us game maps",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "map_name",
			Description: "Map to display",
			Choices:     mapsToCommandChoices(),
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        "detailed",
			Description: "View detailed map",
			Required:    false,
		},
	},
}

func GetMapParams(options []*discordgo.ApplicationCommandInteractionDataOption) (_ game.PlayMap, detailed bool) {
	if len(options) > 1 {
		detailed = options[1].BoolValue()
	}
	return game.PlayMap(options[0].IntValue()), detailed
}

// MapResponse answers /map. When an operator configured BASE_MAP_URL the image
// is linked from there; otherwise the bundled image is attached, so the command
// works without reaching any host AUVC does not control.
func MapResponse(mapType game.PlayMap, detailed bool, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	if url := game.FormMapUrl(os.Getenv("BASE_MAP_URL"), mapType, detailed); url != "" {
		return &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: url,
			},
		}
	}

	name, image, ok := game.MapImage(mapType, detailed)
	if !ok {
		// Not reachable through the command choices, which only offer real maps.
		return &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6,
				Content: sett.LocalizeMessage(&i18n.Message{
					ID:    "commands.map.noImage",
					Other: "No image is available for that map.",
				}),
			},
		}
	}

	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Files: []*discordgo.File{
				{
					Name:        name,
					ContentType: "image/png",
					Reader:      bytes.NewReader(image),
				},
			},
		},
	}
}
