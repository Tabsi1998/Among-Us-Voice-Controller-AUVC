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

	invoker, err := invokerOf(s, interaction)
	if err != nil {
		return auPrivateResponse("❌ Discord guild information is unavailable. Please try again.")
	}

	// /au link on its own offers the crewmate menu, rather than asking for a
	// name whose exact spelling nobody remembers.
	if group == "" && command == au.Link {
		_, named := values.String(au.OptionPlayer)
		_, forSomebodyElse := values.String(au.OptionUser)
		if !named && !forSomebodyElse {
			return bot.crewmatePicker(interaction.GuildID)
		}
	}

	request := au.Request{
		GuildID: interaction.GuildID,
		Group:   group,
		Command: command,
		Invoker: invoker,
		Values:  values,
	}

	content, err := bot.AUVC.Handle(request)
	if errors.Is(err, au.ErrUnauthorized) {
		return auPrivateResponse("❌ You need the configured AUVC admin role or Discord Administrator permission for this command.")
	}
	if err != nil {
		return auPrivateResponse(fmt.Sprintf("❌ AUVC could not complete this command: %v", err))
	}

	switch {
	case group == "" && (command == au.Link || command == au.Unlink):
		go bot.LinksChanged(interaction.GuildID)
	case group == au.GroupSetup:
		// A new text channel takes the crewmate board with it.
		bot.RefreshCrewmates(interaction.GuildID)
	}
	return auPrivateResponse(content)
}

// invokerOf describes who used an interaction, for authorization.
func invokerOf(s *discordgo.Session, interaction *discordgo.InteractionCreate) (au.Invoker, error) {
	guild, err := s.State.Guild(interaction.GuildID)
	if err != nil {
		return au.Invoker{}, err
	}

	return au.Invoker{
		UserID:           interaction.Member.User.ID,
		RoleIDs:          interaction.Member.Roles,
		IsGuildOwner:     guild.OwnerID == interaction.Member.User.ID,
		HasAdministrator: interaction.Member.Permissions&discordgo.PermissionAdministrator != 0,
	}, nil
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
