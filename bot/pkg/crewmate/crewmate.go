// Package crewmate builds the menu where players choose which Among Us figure
// they are, instead of typing their in-game name.
//
// Everything here is a pure function over the session and the player links, so
// what a board shows, and above all what it must not show, is testable without
// Discord. Posting, editing and uploading live in the bot package.
package crewmate

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/assets"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/session"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
	"github.com/bwmarrin/discordgo"
)

// SelectID is the custom id of the menu, which is how the bot recognises a
// choice when Discord sends it back.
const SelectID = "auvc:crewmate"

const (
	playerPrefix = "player:"
	unlinkValue  = "unlink"

	// Discord allows 25 options in a menu and 100 characters in a value.
	maxOptions     = 25
	maxValueLength = 100

	// MaxImageBytes is Discord's limit for one emoji picture.
	MaxImageBytes = 256 * 1024

	embedColor = 0xC51111
)

// colorNames names every colour the game defines, by its number.
var colorNames = map[int]text.Key{
	game.Red:    text.ColorRed,
	game.Blue:   text.ColorBlue,
	game.Green:  text.ColorGreen,
	game.Pink:   text.ColorPink,
	game.Orange: text.ColorOrange,
	game.Yellow: text.ColorYellow,
	game.Black:  text.ColorBlack,
	game.White:  text.ColorWhite,
	game.Purple: text.ColorPurple,
	game.Brown:  text.ColorBrown,
	game.Cyan:   text.ColorCyan,
	game.Lime:   text.ColorLime,
	game.Maroon: text.ColorMaroon,
	game.Rose:   text.ColorRose,
	game.Banana: text.ColorBanana,
	game.Gray:   text.ColorGray,
	game.Tan:    text.ColorTan,
	game.Coral:  text.ColorCoral,
}

// Image is one crewmate picture, ready to upload as an application emoji.
type Image struct {
	Name    string
	DataURI string
}

// Emojis maps an emoji name to the id Discord gave it when it was uploaded.
type Emojis map[string]string

// EmojiName is the name a crewmate picture is uploaded under, or "" for a
// colour the game does not define.
func EmojiName(color int, dead bool) string {
	name := game.GetColorStringForInt(color)
	if name == "" || !game.IsColorString(name) {
		return ""
	}
	if dead {
		return "auvc_" + name + "_dead"
	}
	return "auvc_" + name
}

// Images returns every crewmate picture, alive and dead, ordered by name.
func Images() ([]Image, error) {
	colors := make([]string, 0, len(game.ColorStrings))
	for name := range game.ColorStrings {
		colors = append(colors, name)
	}
	sort.Strings(colors)

	images := make([]Image, 0, 2*len(colors))
	for _, name := range colors {
		for _, dead := range []bool{false, true} {
			file := "emojis/au" + name + ".png"
			if dead {
				file = "emojis/au" + name + "dead.png"
			}

			data, err := assets.Emojis.ReadFile(file)
			if err != nil {
				return nil, fmt.Errorf("crewmate picture %s: %w", file, err)
			}
			if len(data) > MaxImageBytes {
				return nil, fmt.Errorf("crewmate picture %s is %d bytes, more than Discord accepts", file, len(data))
			}

			images = append(images, Image{
				Name:    EmojiName(game.ColorStrings[name], dead),
				DataURI: "data:image/png;base64," + base64.StdEncoding.EncodeToString(data),
			})
		}
	}
	return images, nil
}

// Board is what the crewmate message shows.
type Board struct {
	Embed      *discordgo.MessageEmbed
	Components []discordgo.MessageComponent
}

// Key identifies what a board looks like. Two boards with the same key look
// the same in Discord, so the bot can skip an edit that would change nothing.
func (b Board) Key() string {
	data, err := json.Marshal(struct {
		Embed      *discordgo.MessageEmbed      `json:"embed"`
		Components []discordgo.MessageComponent `json:"components"`
	}{b.Embed, b.Components})
	if err != nil {
		// Nothing in a board can fail to marshal. Should that ever change,
		// an empty key only means one edit too many, never a stale board.
		return ""
	}
	return string(data)
}

// Render builds the board for the players in a session, in a language.
//
// owners maps an in-game name to the Discord user linked to it. emojis may be
// empty, for instance while the pictures are still being uploaded; the board
// then shows names and colours only.
//
// A death nobody has been told about yet looks exactly like a living player.
// The board is public, and a figure that turned into a ghost during tasks
// would announce the kill to everyone reading the channel.
func Render(language text.Language, players []session.GamePlayer, owners map[string]string, emojis Emojis) Board {
	shown := visible(players)

	embed := &discordgo.MessageEmbed{
		Title: language.Say(text.BoardTitle),
		Color: embedColor,
	}
	if len(shown) == 0 {
		embed.Description = language.Say(text.BoardWaiting)
		return Board{Embed: embed}
	}

	lines := []string{language.Say(text.BoardIntro), ""}
	options := make([]discordgo.SelectMenuOption, 0, len(shown)+1)

	for _, player := range shown {
		dead := !player.Alive && player.Revealed
		emojiName := EmojiName(player.Color, dead)
		emojiID := emojis[emojiName]
		color := colorLabel(player.Color, language)

		owner, taken := owners[player.Name]
		ownerText, state := "*"+language.Say(text.BoardFree)+"*", language.Say(text.BoardFree)
		if taken {
			ownerText, state = "<@"+owner+">", language.Say(text.BoardTaken)
		}

		prefix := ""
		if emojiID != "" {
			prefix = "<:" + emojiName + ":" + emojiID + "> "
		}
		lines = append(lines, fmt.Sprintf("%s**%s** · %s · %s", prefix, escape(player.Name), color, ownerText))

		value := playerPrefix + player.Name
		if len(value) > maxValueLength || len(options) == maxOptions-1 {
			continue
		}
		option := discordgo.SelectMenuOption{
			Label:       truncate(player.Name, maxValueLength),
			Value:       value,
			Description: color + " · " + state,
		}
		if emojiID != "" {
			option.Emoji = &discordgo.ComponentEmoji{Name: emojiName, ID: emojiID}
		}
		options = append(options, option)
	}

	unlink := language.Say(text.BoardUnlink)
	embed.Description = strings.Join(lines, "\n")
	embed.Footer = &discordgo.MessageEmbedFooter{Text: language.Say(text.BoardFooter, unlink)}

	options = append(options, discordgo.SelectMenuOption{
		Label:       unlink,
		Value:       unlinkValue,
		Description: language.Say(text.BoardUnlinkHint),
		Emoji:       &discordgo.ComponentEmoji{Name: "✖️"},
	})

	return Board{
		Embed: embed,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					MenuType:    discordgo.StringSelectMenu,
					CustomID:    SelectID,
					Placeholder: language.Say(text.BoardPlaceholder),
					MaxValues:   1,
					Options:     options,
				},
			}},
		},
	}
}

// Choice is what somebody picked in the menu.
type Choice struct {
	Player string
	Unlink bool
}

// ParseChoice reads the values Discord sends back for the menu.
func ParseChoice(values []string) (Choice, bool) {
	if len(values) != 1 {
		return Choice{}, false
	}
	if values[0] == unlinkValue {
		return Choice{Unlink: true}, true
	}
	name, ok := strings.CutPrefix(values[0], playerPrefix)
	if !ok || name == "" {
		return Choice{}, false
	}
	return Choice{Player: name}, true
}

// visible returns the players a board offers, ordered the way the game lists
// colours. A player who disconnected is not in the round any more.
func visible(players []session.GamePlayer) []session.GamePlayer {
	shown := make([]session.GamePlayer, 0, len(players))
	for _, player := range players {
		if player.Name == "" || player.Disconnected {
			continue
		}
		shown = append(shown, player)
	}
	sort.SliceStable(shown, func(i, j int) bool {
		if shown[i].Color != shown[j].Color {
			return shown[i].Color < shown[j].Color
		}
		return shown[i].Name < shown[j].Name
	})
	return shown
}

func colorLabel(color int, language text.Language) string {
	if key, ok := colorNames[color]; ok {
		return language.Say(key)
	}
	return language.Say(text.ColorUnknown)
}

// escape keeps an in-game name from being read as Discord formatting.
func escape(name string) string {
	var builder strings.Builder
	for _, r := range name {
		if strings.ContainsRune("\\*_~`|>[]()<:@#", r) {
			builder.WriteRune('\\')
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func truncate(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit])
}
