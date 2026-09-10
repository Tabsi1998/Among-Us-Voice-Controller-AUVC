package bot

import (
	"encoding/json"
	"fmt"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/bot/setting"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/settings"
	"log"
)

func (bot *Bot) HandleSettingsCommand(guildID string, sett *settings.GuildSettings, settType string, args []string) interface{} {
	var sendMsg interface{}
	// if command invalid, no need to reapply changes to json file
	isValid := false

	switch settType {
	case setting.Language:
		sendMsg, isValid = setting.FnLanguage(sett, args)
	case setting.AdminUserIDs:
		sendMsg, isValid = setting.FnAdminUserIDs(sett, args)
	case setting.RoleIDs:
		sendMsg, isValid = setting.FnPermissionRoleIDs(sett, args)
	case setting.UnmuteDead:
		sendMsg, isValid = setting.FnUnmuteDeadDuringTasks(sett, args)
	case setting.MapVersion:
		sendMsg, isValid = setting.FnMapVersion(sett, args)
	case setting.Delays:
		sendMsg, isValid = setting.FnDelays(sett, args)
	case setting.VoiceRules:
		sendMsg, isValid = setting.FnVoiceRules(sett, args)
	case setting.MatchSummary:
		sendMsg, isValid = setting.FnMatchSummary(sett, args)
	case setting.MatchSummaryChannel:
		sendMsg, isValid = setting.FnMatchSummaryChannel(sett, args)
	case setting.AutoRefresh:
		sendMsg, isValid = setting.FnAutoRefresh(sett, args)
	case setting.LeaderboardMention:
		sendMsg, isValid = setting.FnLeaderboardNameMention(sett, args)
	case setting.LeaderboardSize:
		sendMsg, isValid = setting.FnLeaderboardSize(sett, args)
	case setting.LeaderboardMin:
		sendMsg, isValid = setting.FnLeaderboardMin(sett, args)
	case setting.MuteSpectators:
		sendMsg, isValid = setting.FnMuteSpectators(sett, args)
	case setting.DisplayRoomCode:
		sendMsg, isValid = setting.FnDisplayRoomCode(sett, args)
	case setting.Show:
		jBytes, err := json.MarshalIndent(sett, "", "  ")
		if err != nil {
			log.Println(err)
			return err
		}
		// TODO need to consider if the settings are too long? Is that possible?
		return fmt.Sprintf("```JSON\n%s\n```", jBytes)
	case setting.Reset:
		sett = settings.MakeGuildSettings()
		sendMsg = "Resetting guild settings to default values"
		isValid = true
	case setting.List:
		fallthrough
	default:
		return settingResponse(setting.AllSettings, sett)
	}

	if isValid {
		err := bot.StorageInterface.SetGuildSettings(guildID, sett)
		if err != nil {
			log.Println(err)
		}
	}
	return sendMsg
}
