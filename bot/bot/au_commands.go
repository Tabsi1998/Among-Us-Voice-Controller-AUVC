package bot

import (
	"errors"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
	"github.com/bwmarrin/discordgo"
)

// handleAUCommand is the Discord adapter around the transport-independent AUVC
// application service. It only translates Discord values and response flags;
// authorization, validation and persistence stay testable in pkg/au.
func (bot *Bot) handleAUCommand(s *discordgo.Session, interaction *discordgo.InteractionCreate) *discordgo.InteractionResponse {
	// Every /au reply is private, so it is written in the Discord language of
	// the one member who reads it.
	language := text.FromDiscord(string(interaction.Locale))

	if interaction.GuildID == "" || interaction.Member == nil || interaction.Member.User == nil {
		return auPrivateResponse(language.Say(text.NotInServer))
	}
	if bot.AUVC == nil {
		return auPrivateResponse(language.Say(text.StorageUnavailable))
	}

	group, command, values, err := au.ParsePathAndValues(interaction.ApplicationCommandData().Options)
	if err != nil {
		return auPrivateResponse(language.Say(text.InvalidCommand, err))
	}

	invoker, err := invokerOf(s, interaction)
	if err != nil {
		return auPrivateResponse(language.Say(text.ServerUnavailable))
	}

	// /au link on its own offers the crewmate menu, rather than asking for a
	// name whose exact spelling nobody remembers.
	if group == "" && command == au.Link {
		_, named := values.String(au.OptionPlayer)
		_, forSomebodyElse := values.String(au.OptionUser)
		if !named && !forSomebodyElse {
			return bot.crewmatePicker(interaction.GuildID, language)
		}
	}

	request := au.Request{
		GuildID:  interaction.GuildID,
		Group:    group,
		Command:  command,
		Invoker:  invoker,
		Values:   values,
		Language: language,
	}

	content, err := bot.AUVC.Handle(request)
	if errors.Is(err, au.ErrUnauthorized) {
		return auPrivateResponse(language.Say(text.NotAuthorized))
	}
	if err != nil {
		return auPrivateResponse(language.Say(text.CommandFailed, err))
	}

	switch {
	case group == "" && (command == au.Link || command == au.Unlink):
		go bot.LinksChanged(interaction.GuildID)
	case group == au.GroupSetup || (group == au.GroupSettings && command == au.SettingsLanguage):
		// A new text channel takes the crewmate board with it, and a new
		// language rewrites it.
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
