package bot

import (
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/amongus"
	"github.com/bwmarrin/discordgo"
)

// User struct
type User struct {
	Nick     string `json:"Nick"`
	UserID   string `json:"UserID"`
	UserName string `json:"UserName"`
	// IsBot marks Discord bot accounts, including music bots. The voice policy
	// and channel enforcement must never manage them. Records written before
	// this field existed deserialize as false and correct themselves the next
	// time the member is cached.
	IsBot bool `json:"IsBot"`
}

// UserData struct
type UserData struct {
	User         User   `json:"User"`
	ShouldBeMute bool   `json:"ShouldBeMute"`
	ShouldBeDeaf bool   `json:"ShouldBeDeaf"`
	InGameName   string `json:"PlayerName"`
}

func MakeUserDataFromDiscordUser(dUser *discordgo.User, nick string) UserData {
	return UserData{
		User: User{
			Nick:     nick,
			UserID:   dUser.ID,
			UserName: dUser.Username,
			IsBot:    dUser.Bot,
		},
		ShouldBeDeaf: false,
		ShouldBeMute: false,
		InGameName:   amongus.UnlinkedPlayerName,
	}
}

func (user *UserData) GetNickName() string {
	return user.User.Nick
}

func (user *UserData) SetShouldBeMuteDeaf(mute, deaf bool) {
	user.ShouldBeMute = mute
	user.ShouldBeDeaf = deaf
}

func (user *UserData) GetUserName() string {
	return user.User.UserName
}

func (user *UserData) GetID() string {
	return user.User.UserID
}

func (user *UserData) GetPlayerName() string {
	return user.InGameName
}

func (user *UserData) Link(player amongus.PlayerData) {
	user.InGameName = player.Name
}
