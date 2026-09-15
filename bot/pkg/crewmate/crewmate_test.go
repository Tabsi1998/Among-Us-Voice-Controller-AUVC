package crewmate

import (
	"encoding/base64"
	"regexp"
	"strings"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/session"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
	"github.com/bwmarrin/discordgo"
)

var uploaded = Emojis{
	"auvc_red":       "100",
	"auvc_red_dead":  "101",
	"auvc_blue":      "200",
	"auvc_blue_dead": "201",
}

func menuOf(t *testing.T, board Board) discordgo.SelectMenu {
	t.Helper()

	if len(board.Components) != 1 {
		t.Fatalf("expected one row, got %d", len(board.Components))
	}
	row, ok := board.Components[0].(discordgo.ActionsRow)
	if !ok || len(row.Components) != 1 {
		t.Fatalf("expected one menu in the row, got %+v", board.Components[0])
	}
	menu, ok := row.Components[0].(discordgo.SelectMenu)
	if !ok {
		t.Fatalf("expected a select menu, got %T", row.Components[0])
	}
	return menu
}

// Discord refuses an emoji picture over 256 KB or a name outside its alphabet,
// and a colour without a picture would show a gap in the menu.
func TestEveryColourHasAnAliveAndADeadPicture(t *testing.T) {
	images, err := Images()
	if err != nil {
		t.Fatalf("Images: %v", err)
	}
	if len(images) != 2*len(game.ColorStrings) {
		t.Fatalf("got %d pictures, want %d", len(images), 2*len(game.ColorStrings))
	}

	valid := regexp.MustCompile(`^[a-z0-9_]{2,32}$`)
	seen := map[string]bool{}
	for _, image := range images {
		if !valid.MatchString(image.Name) {
			t.Errorf("%q is not a valid emoji name", image.Name)
		}
		if seen[image.Name] {
			t.Errorf("%q appears twice", image.Name)
		}
		seen[image.Name] = true

		encoded, ok := strings.CutPrefix(image.DataURI, "data:image/png;base64,")
		if !ok {
			t.Fatalf("%s is not a PNG data URI", image.Name)
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("%s: %v", image.Name, err)
		}
		if len(data) > MaxImageBytes || !strings.HasPrefix(string(data), "\x89PNG") {
			t.Errorf("%s is not a PNG Discord accepts (%d bytes)", image.Name, len(data))
		}
	}

	for name, color := range game.ColorStrings {
		for _, dead := range []bool{false, true} {
			if !seen[EmojiName(color, dead)] {
				t.Errorf("%s (dead: %t) has no picture", name, dead)
			}
		}
	}
}

// The board is public. A figure that turned into a ghost during tasks would
// tell the whole channel who was killed before any meeting announced it.
func TestAnUnannouncedDeathLooksExactlyLikeALivingPlayer(t *testing.T) {
	owners := map[string]string{"Alice": "user-a"}

	alive := Render(text.English, Round{}, []session.GamePlayer{
		{Name: "Alice", Color: game.Red, Alive: true},
		{Name: "Bob", Color: game.Blue, Alive: true},
	}, owners, uploaded)
	killed := Render(text.English, Round{}, []session.GamePlayer{
		{Name: "Alice", Color: game.Red, Alive: false, Revealed: false},
		{Name: "Bob", Color: game.Blue, Alive: true},
	}, owners, uploaded)

	if alive.Key() != killed.Key() {
		t.Fatalf("an unannounced death changed the board:\nalive:  %s\nkilled: %s", alive.Key(), killed.Key())
	}
}

func TestAnAnnouncedDeathShowsTheGhost(t *testing.T) {
	board := Render(text.English, Round{}, []session.GamePlayer{
		{Name: "Alice", Color: game.Red, Alive: false, Revealed: true},
	}, nil, uploaded)

	if !strings.Contains(board.Embed.Description, "<:auvc_red_dead:101>") {
		t.Errorf("the ghost is not shown: %s", board.Embed.Description)
	}
	if emoji := menuOf(t, board).Options[0].Emoji; emoji == nil || emoji.ID != "101" {
		t.Errorf("the menu does not show the ghost: %+v", emoji)
	}
}

func TestTheBoardSaysWhoIsWhoAndWhatIsFree(t *testing.T) {
	board := Render(text.English, Round{}, []session.GamePlayer{
		{Name: "Bob", Color: game.Blue, Alive: true},
		{Name: "Alice", Color: game.Red, Alive: true},
	}, map[string]string{"Alice": "user-a"}, uploaded)

	description := board.Embed.Description
	if !strings.Contains(description, "<:auvc_red:100> **Alice** · Red · <@user-a>") {
		t.Errorf("Alice's owner is missing: %s", description)
	}
	if !strings.Contains(description, "<:auvc_blue:200> **Bob** · Blue · *free*") {
		t.Errorf("Bob is not shown as free: %s", description)
	}

	options := menuOf(t, board).Options
	// Red comes before Blue in the game's own colour order.
	if len(options) != 3 || options[0].Label != "Alice" || options[1].Label != "Bob" {
		t.Fatalf("unexpected options: %+v", options)
	}
	if options[0].Description != "Red · taken" || options[1].Description != "Blue · free" {
		t.Errorf("unexpected descriptions: %q, %q", options[0].Description, options[1].Description)
	}
	if options[2].Label != "Unlink me" {
		t.Errorf("the last option should unlink: %+v", options[2])
	}
}

func TestADisconnectedPlayerIsNotOffered(t *testing.T) {
	board := Render(text.English, Round{}, []session.GamePlayer{
		{Name: "Alice", Color: game.Red, Alive: true},
		{Name: "Gone", Color: game.Blue, Alive: true, Disconnected: true},
	}, nil, uploaded)

	if strings.Contains(board.Embed.Description, "Gone") {
		t.Errorf("a disconnected player is listed: %s", board.Embed.Description)
	}
	for _, option := range menuOf(t, board).Options {
		if option.Label == "Gone" {
			t.Error("a disconnected player is offered")
		}
	}
}

func TestAnEmptyLobbyHasNoMenu(t *testing.T) {
	board := Render(text.English, Round{}, nil, nil, uploaded)

	if len(board.Components) != 0 {
		t.Errorf("Discord refuses a menu without options, got %+v", board.Components)
	}
	if board.Embed == nil || !strings.Contains(board.Embed.Description, "Waiting for players") {
		t.Errorf("an empty lobby should say what to do: %+v", board.Embed)
	}
}

// The pictures are uploaded in the background. Until they are, the board must
// still work rather than refer to emojis Discord does not know.
func TestTheBoardWorksBeforeThePicturesAreUploaded(t *testing.T) {
	board := Render(text.English, Round{}, []session.GamePlayer{{Name: "Alice", Color: game.Red, Alive: true}}, nil, nil)

	if strings.Contains(board.Embed.Description, "<:") {
		t.Errorf("the board refers to an emoji that does not exist yet: %s", board.Embed.Description)
	}
	if emoji := menuOf(t, board).Options[0].Emoji; emoji != nil {
		t.Errorf("the option refers to an emoji that does not exist yet: %+v", emoji)
	}
}

func TestTheMenuStaysWithinDiscordsLimit(t *testing.T) {
	var players []session.GamePlayer
	for i := 0; i < 40; i++ {
		players = append(players, session.GamePlayer{Name: strings.Repeat("x", i+1), Color: i % 18, Alive: true})
	}

	options := menuOf(t, Render(text.English, Round{}, players, nil, nil)).Options
	if len(options) != maxOptions {
		t.Fatalf("got %d options, want %d", len(options), maxOptions)
	}
	if options[len(options)-1].Value != unlinkValue {
		t.Error("unlinking must stay possible in a full lobby")
	}
}

func TestEveryOfferedChoiceReadsBackAsThatPlayer(t *testing.T) {
	board := Render(text.English, Round{}, []session.GamePlayer{
		{Name: "Alice", Color: game.Red, Alive: true},
		{Name: "player:odd", Color: game.Blue, Alive: true},
	}, nil, nil)

	wanted := map[string]bool{"Alice": true, "player:odd": true}
	for _, option := range menuOf(t, board).Options {
		choice, ok := ParseChoice([]string{option.Value})
		if !ok {
			t.Fatalf("%q does not read back", option.Value)
		}
		if option.Value == unlinkValue {
			if !choice.Unlink {
				t.Error("unlink does not read back as unlink")
			}
			continue
		}
		if !wanted[choice.Player] || choice.Player != option.Label {
			t.Errorf("%q read back as %+v", option.Value, choice)
		}
	}

	for _, bad := range [][]string{nil, {}, {"player:"}, {"Alice"}, {"unlink", "unlink"}} {
		if _, ok := ParseChoice(bad); ok {
			t.Errorf("%q should be refused", bad)
		}
	}
}

func TestANameCannotFormatTheBoard(t *testing.T) {
	board := Render(text.English, Round{}, []session.GamePlayer{{Name: "**@everyone**", Color: game.Red, Alive: true}}, nil, nil)

	if strings.Contains(board.Embed.Description, "**@everyone**") {
		t.Errorf("the name was not escaped: %s", board.Embed.Description)
	}
}

func TestAColourTheGameDoesNotKnowHasNoPicture(t *testing.T) {
	if name := EmojiName(99, false); name != "" {
		t.Errorf("got %q for an unknown colour", name)
	}
	if name := EmojiName(-1, true); name != "" {
		t.Errorf("got %q for a negative colour", name)
	}
}
