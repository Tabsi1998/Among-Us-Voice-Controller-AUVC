package bot

import (
	"errors"
	"fmt"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/bwmarrin/discordgo"
)

// handleAUCommand is the Discord adapter around the transport-independent AUVC
// application service. It only translates Discord values and response flags;
// authorization, validation and persistence stay testable in pkg/au.
func (bot *Bot) handleAUCommand(s *discordgo.Session, interaction *discordgo.InteractionCreate) *discordgo.InteractionResponse {
	if interaction.GuildID == "" || interaction.Member == nil || interaction.Member.User == nil {
		return auPrivateResponse("❌ `/au` commands can only be used inside a Discord server.")
	}
	if bot.AUVC == nil {
		return auPrivateResponse("❌ AUVC configuration storage is unavailable.")
	}

	group, command, values, err := au.ParsePathAndValues(interaction.ApplicationCommandData().Options)
	if err != nil {
		return auPrivateResponse("❌ Invalid `/au` command: " + err.Error())
	}

	guild, err := s.State.Guild(interaction.GuildID)
	if err != nil {
		return auPrivateResponse("❌ Discord guild information is unavailable. Please try again.")
	}

	request := au.Request{
		GuildID: interaction.GuildID,
		Group:   group,
		Command: command,
		Invoker: au.Invoker{
			UserID:           interaction.Member.User.ID,
			RoleIDs:          interaction.Member.Roles,
			IsGuildOwner:     guild.OwnerID == interaction.Member.User.ID,
			HasAdministrator: interaction.Member.Permissions&discordgo.PermissionAdministrator != 0,
		},
		Values: values,
	}

	content, err := bot.AUVC.Handle(request)
	if errors.Is(err, au.ErrUnauthorized) {
		return auPrivateResponse("❌ You need the configured AUVC admin role or Discord Administrator permission for this command.")
	}
	if err != nil {
		return auPrivateResponse(fmt.Sprintf("❌ AUVC could not complete this command: %v", err))
	}
	return auPrivateResponse(content)
}

func auPrivateResponse(content string) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   discordgo.MessageFlagsEphemeral,
			Content: content,
		},
	}
}
