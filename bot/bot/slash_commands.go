package bot

import (
	"encoding/json"
	"errors"
	"log"
	"regexp"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/bot/command"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/bot/setting"
	redis_common "github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/common"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/settings"
	"github.com/bwmarrin/discordgo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

var MatchIDRegex = regexp.MustCompile(`^[A-Z0-9]{8}:[0-9]+$`)

var DownloadPermissions = []int64{
	discordgo.PermissionAttachFiles,
}

var RequiredPermissions = []int64{
	discordgo.PermissionViewChannel, discordgo.PermissionSendMessages,
	discordgo.PermissionManageMessages, discordgo.PermissionEmbedLinks,
	discordgo.PermissionUseExternalEmojis,
}

var VoicePermissions = []int64{
	discordgo.PermissionVoiceMuteMembers, discordgo.PermissionVoiceDeafenMembers,
}

const (
	resetUserConfirmedID          = "reset-user-confirmed"
	resetUserCanceledID           = "reset-user-canceled"
	resetGuildConfirmedID         = "reset-guild-confirmed"
	resetGuildCanceledID          = "reset-guild-canceled"
	downloadGuildConfirmedID      = "download-guild-confirmed"
	downloadUsersConfirmedID      = "download-users-confirmed"
	downloadUsersGamesConfirmedID = "download-users-games-confirmed"
	downloadGamesConfirmedID      = "download-games-confirmed"
	downloadGameEventsConfirmedID = "download-game-events-confirmed"
	downloadCanceledID            = "download-canceled"
)

func (bot *Bot) handleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	respondChan := make(chan *discordgo.InteractionResponse)
	ticker := time.NewTicker(time.Second * 2)
	var followUpMsg *discordgo.Message
	var err error

	// get the result in the background
	go func() {
		respondChan <- bot.slashCommandHandler(s, i)
	}()

	for {
		select {
		case <-ticker.C:
			// only followup the first time
			if followUpMsg == nil {
				err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Flags:   1 << 6,
						Content: "Give me just a little bit to make a proper response :)",
					},
				})
				if err != nil {
					log.Println("err issuing wait response ", err)
				}
				followUpMsg, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: Hourglass,
				})
				if err != nil {
					log.Println("Error creating followup message: ", err)
				}
			}
			// don't return here

		case resp := <-respondChan:
			if followUpMsg != nil {
				if resp != nil && resp.Data != nil {
					content := resp.Data.Content
					if content == "" {
						content = "\u200b"
					}
					followUpMsg, err = s.FollowupMessageEdit(i.Interaction, followUpMsg.ID, &discordgo.WebhookEdit{
						Content:    &content,
						Components: &resp.Data.Components,
						Embeds:     &resp.Data.Embeds,
					})
				} else {
					//TODO if this shows up in logs regularly, print more context
					log.Println("received a nil response, or resp.data was nil")
				}
			} else if resp != nil {
				err = s.InteractionRespond(i.Interaction, resp)
				if err != nil {
					log.Println("error issuing interaction response: ", err)
					iBytes, err := json.Marshal(i.Interaction)
					if err != nil {
						log.Println(err)
					} else {
						log.Println(string(iBytes))
					}
				}
			}
			ticker.Stop()
			// no matter what we get back, return
			return
		}
	}
}

func (bot *Bot) slashCommandHandler(s *discordgo.Session, i *discordgo.InteractionCreate) *discordgo.InteractionResponse {
	if i.Type == discordgo.InteractionApplicationCommand && i.ApplicationCommandData().Name == au.Name {
		return bot.handleAUCommand(s, i)
	}

	if i.Member != nil && i.Member.User != nil {
		if redis_common.IsUserBanned(bot.RedisInterface.client, i.Member.User.ID) {
			return nil
		}
	}

	// lock this particular interaction message so no other shard tries to process it
	interactionLock := bot.RedisInterface.LockSnowflake(i.ID)
	// couldn't obtain lock; bail bail bail!
	if interactionLock == nil {
		return nil
	}
	defer interactionLock.Release(ctx)

	sett := bot.StorageInterface.GetGuildSettings(i.GuildID)

	// TODO respond properly for commands that *can* be performed in DMs. Such as minimal stats queries, help, info, etc
	// NOTE: difference between i.Member.User (Server/Guild chat) vs i.User (DMs)
	if i.GuildID == "" || i.Member == nil || i.Member.User == nil {
		return command.DmResponse(sett)
	}

	if redis_common.IsUserRateLimitedGeneral(bot.RedisInterface.client, i.Member.User.ID) {
		banned := redis_common.IncrementRateLimitExceed(bot.RedisInterface.client, i.Member.User.ID)
		return softbanResponse(banned, sett)
	}

	g, err := s.State.Guild(i.GuildID)
	if err != nil {
		log.Println(err)
		return command.PrivateErrorResponse("get-guild", err, sett)
	}
	perm, err := bot.PrimarySession.State.UserChannelPermissions(s.State.User.ID, i.ChannelID)
	if err != nil {
		log.Println(err)
		return command.PrivateErrorResponse("get-permissions", err, sett)
	}
	missingPerms := checkPermissions(perm, RequiredPermissions)
	if missingPerms > 0 {
		return command.ReinviteMeResponse(missingPerms, i.ChannelID, sett)
	}

	isAdmin, isPermissioned := false, false
	if g.OwnerID == i.Member.User.ID || (len(sett.AdminUserIDs) == 0 && len(sett.PermissionRoleIDs) == 0) {
		// the guild owner should always have both permissions
		// or if both permissions are still empty, everyone gets both
		isAdmin = true
		isPermissioned = true
	} else {
		// if we have no admins, then we MUST have mods as per the check above. So ensure this user is a mod
		if len(sett.AdminUserIDs) == 0 {
			isAdmin = sett.HasRolePerms(i.Member)
		} else {
			// we have admins; make sure user is one
			isAdmin = sett.HasAdminPerms(i.Member.User)
		}
		// even if we have admins, we can grant mod if the moderators role is empty; it is lesser permissions
		isPermissioned = len(sett.PermissionRoleIDs) == 0 || sett.HasRolePerms(i.Member)
	}

	// common gsr, but not necessarily used by all commands
	gsr := GameStateRequest{
		GuildID:     i.GuildID,
		TextChannel: i.ChannelID,
	}

	if i.Type == discordgo.InteractionApplicationCommand {
		if redis_common.IsUserRateLimitedSpecific(bot.RedisInterface.client, i.Member.User.ID, i.ApplicationCommandData().Name) {
			banned := redis_common.IncrementRateLimitExceed(bot.RedisInterface.client, i.Member.User.ID)
			return softbanResponse(banned, sett)
		}
		var cmdRatelimitTimeout = redis_common.GlobalUserRateLimitDuration
		// /new has a longer ratelimit window than other commands (it's an expensive operation)
		if i.ApplicationCommandData().Name == command.New.Name {
			cmdRatelimitTimeout = redis_common.NewGameRateLimitDuration
		}
		redis_common.MarkUserRateLimit(bot.RedisInterface.client, i.Member.User.ID, i.ApplicationCommandData().Name, cmdRatelimitTimeout)
		switch i.ApplicationCommandData().Name {
		case command.Help.Name:
			return command.HelpResponse(sett, i.ApplicationCommandData().Options)

		case command.Info.Name:
			botInfo := bot.getInfo()
			return command.InfoResponse(botInfo, i.GuildID, sett)

		case command.Link.Name:
			if !isPermissioned {
				return command.InsufficientPermissionsResponse(sett)
			}
			userID, color := command.GetLinkParams(s, i.ApplicationCommandData().Options)

			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLockRetries(gsr, 5)
			if lock == nil {
				log.Printf("No lock could be obtained when linking for guild %s, channel %s\n", i.GuildID, i.ChannelID)
				return command.DeadlockGameStateResponse(command.Link.Name, sett)
			}
			resp, success := bot.linkOrUnlinkAndRespond(dgs, userID, color, sett)
			if success {
				bot.RedisInterface.SetDiscordGameState(dgs, lock)
				bot.DispatchRefreshOrEdit(dgs, gsr, sett)
			} else {
				// release the lock
				bot.RedisInterface.SetDiscordGameState(nil, lock)
			}
			return resp

		case command.Unlink.Name:
			if !isPermissioned {
				return command.InsufficientPermissionsResponse(sett)
			}
			userID := command.GetUnlinkParams(s, i.ApplicationCommandData().Options)

			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLock(gsr)
			if lock == nil {
				log.Printf("No lock could be obtained when unlinking for guild %s, channel %s\n", i.GuildID, i.ChannelID)
				return command.DeadlockGameStateResponse(command.Unlink.Name, sett)
			}
			resp, success := bot.linkOrUnlinkAndRespond(dgs, userID, "", sett)
			if success {
				bot.RedisInterface.SetDiscordGameState(dgs, lock)
				bot.DispatchRefreshOrEdit(dgs, gsr, sett)
			} else {
				// release the lock
				bot.RedisInterface.SetDiscordGameState(nil, lock)
			}
			return resp

		case command.Settings.Name:
			if !isAdmin {
				return command.InsufficientPermissionsResponse(sett)
			}
			setting, args := command.GetSettingsParams(i.ApplicationCommandData().Options)
			msg := bot.HandleSettingsCommand(i.GuildID, sett, setting, args)
			return command.SettingsResponse(msg)

		case command.New.Name:
			if !isPermissioned {
				return command.InsufficientPermissionsResponse(sett)
			}

			voiceChannelID := getTrackingChannel(g, i.Member.User.ID)
			if voiceChannelID == "" {
				return command.NewResponse(command.NewNoVoiceChannel, command.NewInfo{}, sett)
			}

			perm, err = bot.PrimarySession.State.UserChannelPermissions(s.State.User.ID, voiceChannelID)
			missingPerms = checkPermissions(perm, VoicePermissions)
			if missingPerms > 0 {
				return command.ReinviteMeResponse(missingPerms, voiceChannelID, sett)
			}

			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLockRetries(gsr, 5)
			if lock == nil {
				log.Printf("No lock could be obtained when making a new game for guild %s, channel %s\n", i.GuildID, i.ChannelID)
				return command.DeadlockGameStateResponse(command.New.Name, sett)
			}

			status, activeGames := bot.newGame(dgs)
			if status == command.NewSuccess {
				// release the lock
				bot.RedisInterface.SetDiscordGameState(dgs, lock)

				bot.RedisInterface.RefreshActiveGame(dgs.GuildID, dgs.ConnectCode)

				killChan := make(chan EndGameMessage)

				go bot.SubscribeToGameByConnectCode(i.GuildID, dgs.ConnectCode, killChan)

				bot.ChannelsMapLock.Lock()
				bot.EndGameChannels[dgs.ConnectCode] = killChan
				bot.ChannelsMapLock.Unlock()

				hyperlink, apiHyperlink, minimalURL := formCaptureURL(bot.url, dgs.ConnectCode)

				bot.handleGameStartMessage(i.GuildID, i.ChannelID, voiceChannelID, i.Member.User.ID, sett, g, dgs.ConnectCode)

				return command.NewResponse(status, command.NewInfo{
					Hyperlink:    hyperlink,
					ApiHyperlink: apiHyperlink,
					MinimalURL:   minimalURL,
					ConnectCode:  dgs.ConnectCode,
					ActiveGames:  activeGames, // not actually needed for Success messages
				}, sett)
			} else {
				// release the lock
				bot.RedisInterface.SetDiscordGameState(nil, lock)
				return command.NewResponse(status, command.NewInfo{
					ActiveGames: activeGames, // only field we need for success messages
				}, sett)
			}
		case command.Refresh.Name:
			if bot.RefreshGameStateMessage(gsr, sett) {
				return command.PrivateResponse(ThumbsUp)
			} else {
				return command.NoGameResponse(sett)
			}

		case command.Pause.Name:
			if !isPermissioned {
				return command.InsufficientPermissionsResponse(sett)
			}
			lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLockRetries(gsr, 5)
			if lock == nil {
				log.Printf("No lock could be obtained when pausing game for guild %s, channel %s\n", i.GuildID, i.ChannelID)
				return command.DeadlockGameStateResponse(command.Pause.Name, sett)
			}
			if !dgs.GameStateMsg.Exists() {
				bot.RedisInterface.SetDiscordGameState(nil, lock)
				return command.NoGameResponse(sett)
			}

			dgs.Running = !dgs.Running

			bot.RedisInterface.SetDiscordGameState(dgs, lock)
			// if we paused the game, unmute/undeafen all players
			if !dgs.Running {
				err = bot.applyToAll(dgs, false, false)
			}
			bot.DispatchRefreshOrEdit(dgs, gsr, sett)
			if err != nil {
				return command.PrivateErrorResponse(command.Pause.Name, err, sett)
			}
			return command.PrivateResponse(ThumbsUp)

		case command.End.Name:
			if !isPermissioned {
				return command.InsufficientPermissionsResponse(sett)
			}
			dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
			if dgs != nil {
				if !dgs.GameStateMsg.Exists() {
					return command.NoGameResponse(sett)
				}

				if v, ok := bot.EndGameChannels[dgs.ConnectCode]; ok {
					v <- true
				}
				delete(bot.EndGameChannels, dgs.ConnectCode)

				err = bot.applyToAll(dgs, false, false)
				if err != nil {
					return command.PrivateErrorResponse(command.End.Name, err, sett)
				}
				return command.PrivateResponse(ThumbsUp)
			}
			return command.DeadlockGameStateResponse(command.End.Name, sett)

		case command.Map.Name:
			mapType, detailed := command.GetMapParams(i.ApplicationCommandData().Options)
			return command.MapResponse(mapType, detailed, sett)

		case command.Debug.Name:
			action, opType, id := command.GetDebugParams(bot.PrimarySession, i.Member.User.ID, i.ApplicationCommandData().Options)
			if action == setting.View {
				if opType == command.User {
					cached, err := bot.RedisInterface.GetUsernameOrUserIDMappings(i.GuildID, id)
					log.Println("View user cache")
					return command.DebugResponse(setting.View, cached, nil, id, err, sett)
				} else if opType == command.GameState {
					state := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
					if state != nil {
						jBytes, err := json.MarshalIndent(state, "", "  ")
						return command.DebugResponse(setting.View, nil, jBytes, id, err, sett)
					} else {
						return command.DeadlockGameStateResponse(command.Debug.Name, sett)
					}
				}
			} else if action == setting.Clear {
				if opType == command.User {
					if id != i.Member.User.ID {
						if !isAdmin {
							return command.InsufficientPermissionsResponse(sett)
						}
					}
					err := bot.RedisInterface.DeleteLinksByUserID(i.GuildID, id)
					return command.DebugResponse(setting.Clear, nil, nil, id, err, sett)
				}
			} else if action == command.Unmute {
				// prob shouldn't be constructing the GameState explicitly like this... okay so long as we don't reuse it
				dgs := GameState{
					GuildID:     i.GuildID,
					ConnectCode: i.GuildID + "-unmute",
				}
				// admins can always unmute no matter what
				if isAdmin {
					err = bot.applyToSingle(&dgs, id, false, false)
					if err != nil {
						return command.PrivateErrorResponse(command.Unmute, err, sett)
					}
					return command.PrivateResponse(ThumbsUp)
				}

				for _, v := range g.VoiceStates {
					if v.UserID == id {
						// fetch the game state purely by the voice channel ID
						gsr.TextChannel = ""
						gsr.VoiceChannel = v.ChannelID
						log.Println("fetching game by id ", v.ChannelID)

						// no game is happening in this voice channel, so we're safe to unmute
						if bot.RedisInterface.getDiscordGameStateKey(gsr) == "" {
							err = bot.applyToSingle(&dgs, id, false, false)
							if err != nil {
								return command.PrivateErrorResponse(command.Unmute, err, sett)
							}
							return command.PrivateResponse(ThumbsUp)
						} else {
							// there's a game happening, so we can't unmute
							return command.DebugResponse(command.Unmute, nil, nil, id, errors.New(""), sett)
						}
					}
				}
				return command.PrivateErrorResponse(command.Unmute, errors.New("user is not in a voice channel"), sett)
			} else if action == command.UnmuteAll {
				dgs := bot.RedisInterface.GetReadOnlyDiscordGameState(gsr)
				if dgs != nil {
					err = bot.applyToAll(dgs, false, false)
					if err != nil {
						return command.PrivateErrorResponse(command.UnmuteAll, err, sett)
					}
					return command.PrivateResponse(ThumbsUp)
				}
				return command.DeadlockGameStateResponse(command.UnmuteAll, sett)
			}
		}

	} else if i.Type == discordgo.InteractionMessageComponent {
		if redis_common.IsUserRateLimitedSpecific(bot.RedisInterface.client, i.Member.User.ID, i.MessageComponentData().CustomID) {
			banned := redis_common.IncrementRateLimitExceed(bot.RedisInterface.client, i.Member.User.ID)
			return softbanResponse(banned, sett)
		}
		redis_common.MarkUserRateLimit(bot.RedisInterface.client, i.Member.User.ID, i.MessageComponentData().CustomID, redis_common.GlobalUserRateLimitDuration)

		switch i.MessageComponentData().CustomID {
		case colorSelectID:
			if len(i.MessageComponentData().Values) > 0 {
				value := i.MessageComponentData().Values[0]
				lock, dgs := bot.RedisInterface.GetDiscordGameStateAndLockRetries(gsr, 5)
				if lock == nil {
					log.Printf("No lock could be obtained when linking for guild %s, channel %s\n", i.GuildID, i.ChannelID)
					return command.DeadlockGameStateResponse(command.Link.Name, sett)
				}
				if value == UnlinkEmojiName {
					value = ""
				}
				resp, success := bot.linkOrUnlinkAndRespond(dgs, i.Member.User.ID, value, sett)
				if success {
					bot.RedisInterface.SetDiscordGameState(dgs, lock)
					bot.DispatchRefreshOrEdit(dgs, gsr, sett)
				} else {
					// only release the lock; no changes
					bot.RedisInterface.SetDiscordGameState(nil, lock)
				}
				return resp
			}

		case resetUserCanceledID:
			fallthrough
		case resetGuildCanceledID:
			if i.Message.MessageReference != nil {
				bot.deleteComponentInParentMessage(s, i)
			}
			return resetCancelResponse(sett)
		}
	}

	// no command or handler matched somehow
	return nil
}

func downloadErrorResponse(sett *settings.GuildSettings, err error) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Flags: 1 << 6, //private message
			Content: sett.LocalizeMessage(&i18n.Message{
				ID:    "commands.download.guild.error",
				Other: "I encountered an error fetching your stats for download: {{.Error}}",
			}, map[string]interface{}{
				"Error": err.Error(),
			}),
		},
	}
}

func (bot *Bot) linkOrUnlinkAndRespond(dgs *GameState, userID, testValue string, sett *settings.GuildSettings) (*discordgo.InteractionResponse, bool) {
	if testValue != "" {
		// don't care if it's successful, just always unlink before linking
		unlinkPlayer(dgs, userID)
		status, err := linkPlayer(bot.RedisInterface, dgs, userID, testValue)
		if err != nil {
			log.Println(err)
		}
		return command.LinkResponse(status, userID, testValue, sett), status == command.LinkSuccess
	} else {
		status := unlinkPlayer(dgs, userID)
		return command.UnlinkResponse(status, userID, sett), status == command.UnlinkSuccess
	}
}

// deleteComponentInParentMessage deletes any components from parent messages.
// this is required for safety. if the resetting process takes over 2 seconds,
// since RESET/Cancel buttons remain forever once the button has been clicked.
func (bot *Bot) deleteComponentInParentMessage(s *discordgo.Session, i *discordgo.InteractionCreate) {
	me := discordgo.NewMessageEdit(i.ChannelID, i.Message.ID)
	me.Components = &[]discordgo.MessageComponent{}
	_, err := s.ChannelMessageEditComplex(me)
	if err != nil {
		log.Println("Error when attempting to edit complex message", err)
	}
}

func confirmationComponents(confirmedID string, canceledID string, sett *settings.GuildSettings) []discordgo.MessageComponent {
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					CustomID: confirmedID,
					Style:    discordgo.DangerButton,
					Label: sett.LocalizeMessage(&i18n.Message{
						ID:    "commands.stats.reset.button.proceed",
						Other: "Confirm",
					}),
					Emoji: &discordgo.ComponentEmoji{Name: ThumbsUp},
				},
				discordgo.Button{
					CustomID: canceledID,
					Style:    discordgo.SecondaryButton,
					Label: sett.LocalizeMessage(&i18n.Message{
						ID:    "commands.stats.reset.button.cancel",
						Other: "Cancel",
					}),
					Emoji: &discordgo.ComponentEmoji{Name: X},
				},
			},
		},
	}
}

func resetCancelResponse(sett *settings.GuildSettings) *discordgo.InteractionResponse {
	content := sett.LocalizeMessage(&i18n.Message{
		ID:    "commands.stats.reset.canceled",
		Other: "Operation has been canceled",
	})
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Flags:      1 << 6, //private message
			Content:    content,
			Components: []discordgo.MessageComponent{},
		},
	}
}

func softbanResponse(banned bool, sett *settings.GuildSettings) *discordgo.InteractionResponse {
	if banned {
		return &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: 1 << 6, //private message
				Content: sett.LocalizeMessage(&i18n.Message{
					ID:    "softban.ignoring",
					Other: "I'm ignoring you for the next 5 minutes, stop spamming",
				}),
			},
		}
	}
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: 1 << 6, //private message
			Content: sett.LocalizeMessage(&i18n.Message{
				ID:    "softban.warning",
				Other: "Please stop spamming commands",
			}),
		},
	}
}

func checkPermissions(perm int64, perms []int64) (a int64) {
	for _, v := range perms {
		if v&perm != v {
			a |= v
		}
	}
	return
}
