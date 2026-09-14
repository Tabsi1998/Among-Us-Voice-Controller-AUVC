package main

import (
	"errors"
	"fmt"
	"log"
	"sort"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/bot"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/localcontrol"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/pairing"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/transport"
	"github.com/bwmarrin/discordgo"
)

// localControl builds the interface the AUVC Windows app sets this bot up
// through, or nothing when the app did not start this bot.
func localControl(secret string, backend localcontrol.Backend) (*localcontrol.Server, error) {
	if secret == "" {
		return nil, nil
	}

	server, err := localcontrol.New(secret, backend)
	if err != nil {
		return nil, err
	}
	log.Println("Local control for the AUVC app is enabled, for this computer only")
	return server, nil
}

// localBackend answers the Windows app from the running bot.
type localBackend struct {
	controller *bot.Bot
	pairing    *pairing.Service
	capture    *transport.Server
	doctor     *bot.Doctor
	version    string
	stop       func()
}

func (l *localBackend) Status() localcontrol.Status {
	status := localcontrol.Status{Version: l.version}

	session := l.controller.PrimarySession
	if session == nil || session.State == nil {
		return status
	}

	session.State.RLock()
	defer session.State.RUnlock()

	if user := session.State.User; user != nil {
		status.Connected = true
		status.BotID = user.ID
		status.BotName = user.Username
	}
	for _, guild := range session.State.Guilds {
		status.Guilds = append(status.Guilds, localcontrol.GuildSummary{ID: guild.ID, Name: guild.Name})
	}
	sort.Slice(status.Guilds, func(i, j int) bool { return status.Guilds[i].Name < status.Guilds[j].Name })

	return status
}

func (l *localBackend) Channels(guildID string) ([]localcontrol.Channel, error) {
	guild, err := l.discordGuild(guildID)
	if err != nil {
		return nil, err
	}

	state := l.controller.PrimarySession.State
	state.RLock()
	defer state.RUnlock()

	channels := make([]localcontrol.Channel, 0, len(guild.Channels))
	for _, channel := range guild.Channels {
		kind := channelKind(channel.Type)
		if kind == "" {
			continue
		}
		channels = append(channels, localcontrol.Channel{
			ID: channel.ID, Name: channel.Name, Kind: kind, Position: channel.Position,
		})
	}

	// Voice channels first, each kind in the order Discord shows them, so the
	// app's lists read like the channel list the person is looking at.
	sort.SliceStable(channels, func(i, j int) bool {
		if channels[i].Kind != channels[j].Kind {
			return channels[i].Kind == localcontrol.KindVoice
		}
		return channels[i].Position < channels[j].Position
	})
	return channels, nil
}

func (l *localBackend) Guild(guildID string) (localcontrol.Guild, error) {
	guild, err := l.discordGuild(guildID)
	if err != nil {
		return localcontrol.Guild{}, err
	}

	config, err := l.controller.AUVC.GuildConfig(guildID)
	if err != nil {
		return localcontrol.Guild{}, err
	}

	state := l.controller.PrimarySession.State
	state.RLock()
	name := guild.Name
	state.RUnlock()

	result := localcontrol.Guild{
		ID:                   guildID,
		Name:                 name,
		MainVoiceChannelID:   config.MainVoiceChannelID,
		GhostVoiceChannelID:  config.GhostVoiceChannelID,
		ControlTextChannelID: config.ControlTextChannelID,
		AutoStart:            config.AutoStart,
		Checks:               []localcontrol.Check{},
	}
	if l.capture != nil {
		result.CaptureConnections = l.capture.Connections(guildID)
	}
	if l.doctor != nil {
		for _, check := range l.doctor.Report(guildID) {
			result.Checks = append(result.Checks, localcontrol.Check{
				Name: check.Name, Level: check.Level.String(), Detail: check.Detail, Fix: check.Fix,
			})
		}
	}
	return result, nil
}

// Configure saves a setup, after checking that every channel is one of this
// server's and of the right kind.
//
// Discord checks this for /au setup channels by offering only voice channels
// for a voice option. The app sends plain ids, so the same check happens here.
func (l *localBackend) Configure(guildID string, setup localcontrol.Setup) error {
	guild, err := l.discordGuild(guildID)
	if err != nil {
		return err
	}

	state := l.controller.PrimarySession.State
	state.RLock()
	kinds := make(map[string]string, len(guild.Channels))
	for _, channel := range guild.Channels {
		kinds[channel.ID] = channelKind(channel.Type)
	}
	state.RUnlock()

	if kinds[setup.MainVoiceChannelID] != localcontrol.KindVoice {
		return fmt.Errorf("%w: the main channel must be a voice channel in this server", localcontrol.ErrInvalidSetup)
	}
	if kinds[setup.GhostVoiceChannelID] != localcontrol.KindVoice {
		return fmt.Errorf("%w: the ghost channel must be a voice channel in this server", localcontrol.ErrInvalidSetup)
	}
	if setup.ControlTextChannelID != "" && kinds[setup.ControlTextChannelID] != localcontrol.KindText {
		return fmt.Errorf("%w: the control channel must be a text channel in this server", localcontrol.ErrInvalidSetup)
	}

	_, err = l.controller.AUVC.ConfigureFromApp(guildID, au.AppSetup{
		MainVoiceChannelID:   setup.MainVoiceChannelID,
		GhostVoiceChannelID:  setup.GhostVoiceChannelID,
		ControlTextChannelID: setup.ControlTextChannelID,
		AutoStart:            setup.AutoStart,
	})
	if errors.Is(err, au.ErrInvalidInput) {
		return fmt.Errorf("%w: %v", localcontrol.ErrInvalidSetup, err)
	}
	return err
}

func (l *localBackend) IssueCredential(guildID string) (string, error) {
	if _, err := l.discordGuild(guildID); err != nil {
		return "", err
	}

	issued, err := l.pairing.Issue(guildID, "auvc-app")
	if err != nil {
		return "", err
	}

	// The credential itself is returned to the app and never written down.
	log.Printf("Issued a capture credential to the AUVC app for guild %s", guildID)
	return issued.Token().Reveal(), nil
}

func (l *localBackend) Shutdown() {
	l.stop()
}

// discordGuild finds a server the bot is in.
func (l *localBackend) discordGuild(guildID string) (*discordgo.Guild, error) {
	session := l.controller.PrimarySession
	if session == nil || session.State == nil || guildID == "" {
		return nil, localcontrol.ErrUnknownGuild
	}

	guild, err := session.State.Guild(guildID)
	if err != nil || guild == nil {
		return nil, localcontrol.ErrUnknownGuild
	}
	return guild, nil
}

// channelKind says what a channel can be used for, or nothing. Stage channels
// are left out: players cannot talk freely in one, so it is no place for a
// round.
func channelKind(channelType discordgo.ChannelType) string {
	switch channelType {
	case discordgo.ChannelTypeGuildVoice:
		return localcontrol.KindVoice
	case discordgo.ChannelTypeGuildText:
		return localcontrol.KindText
	default:
		return ""
	}
}
