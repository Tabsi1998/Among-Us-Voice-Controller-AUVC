package crewmate

import (
	"strings"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/session"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
)

// Everybody in the text channel reads the board, so all of it is in the
// server's language, the colour names included.
func TestTheBoardIsWrittenInTheServerLanguage(t *testing.T) {
	board := Render(text.German, []session.GamePlayer{
		{Name: "Alice", Color: game.Red, Alive: true},
		{Name: "Bob", Color: game.Lime, Alive: true},
	}, map[string]string{"Alice": "user-a"}, nil)

	if board.Embed.Title != text.German.Say(text.BoardTitle) {
		t.Errorf("title %q", board.Embed.Title)
	}
	for _, expected := range []string{"**Alice** · Rot · <@user-a>", "**Bob** · Hellgrün · *frei*"} {
		if !strings.Contains(board.Embed.Description, expected) {
			t.Errorf("the German board is missing %q: %s", expected, board.Embed.Description)
		}
	}

	menu := menuOf(t, board)
	if menu.Placeholder != text.German.Say(text.BoardPlaceholder) {
		t.Errorf("placeholder %q", menu.Placeholder)
	}
	if got := menu.Options[0].Description; got != "Rot · vergeben" {
		t.Errorf("first option %q", got)
	}
	unlink := menu.Options[len(menu.Options)-1]
	if unlink.Label != text.German.Say(text.BoardUnlink) || !strings.Contains(board.Embed.Footer.Text, unlink.Label) {
		t.Errorf("the unlink option %q and the footer %q do not agree", unlink.Label, board.Embed.Footer.Text)
	}

	empty := Render(text.German, nil, nil, nil)
	if empty.Embed.Description != text.German.Say(text.BoardWaiting) {
		t.Errorf("an empty German board reads %q", empty.Embed.Description)
	}
}

// Every colour the game defines has a name in every language, so no crewmate is
// listed as an unknown colour.
func TestEveryColourHasAName(t *testing.T) {
	for name, color := range game.ColorStrings {
		if _, ok := colorNames[color]; !ok {
			t.Errorf("%s (%d) has no name", name, color)
		}
	}
}
