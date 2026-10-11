package bot

import (
	"strings"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
)

// The post names the winning side, how they won, and the impostors.
func TestTheRoundResultNamesWinnersAndImpostors(t *testing.T) {
	notice := roundResultNotice(text.German, protocol.RoundResult{
		Winner: protocol.WinnerImpostors, Reason: protocol.ReasonKill, Impostors: []string{"Leon", "Mira"},
	})

	for _, want := range []string{
		text.German.Say(text.RoundWonImpostors), text.German.Say(text.RoundByKill),
		text.German.Say(text.RoundImpostors, "**Leon**, **Mira**"),
	} {
		if !strings.Contains(notice, want) {
			t.Errorf("the post does not say %q:\n%s", want, notice)
		}
	}
}

func TestCrewmatesWinningIsSaidAsSuch(t *testing.T) {
	notice := roundResultNotice(text.English, protocol.RoundResult{Winner: protocol.WinnerCrewmates, Reason: protocol.ReasonTasks})

	if !strings.HasPrefix(notice, text.English.Say(text.RoundWonCrewmates)+" "+text.English.Say(text.RoundByTasks)) {
		t.Errorf("crewmates winning by tasks reads: %s", notice)
	}
	if !strings.Contains(notice, text.English.Say(text.RoundImpostorsNone)) {
		t.Errorf("no impostors read should say so: %s", notice)
	}
}

// A newer capture may know a way to end a round this bot does not: the post
// leaves it out instead of naming a wrong one.
func TestAnUnknownReasonIsLeftOut(t *testing.T) {
	notice := roundResultNotice(text.English, protocol.RoundResult{Winner: protocol.WinnerCrewmates, Reason: "meteor"})

	if first := strings.SplitN(notice, "\n", 2)[0]; first != text.English.Say(text.RoundWonCrewmates) {
		t.Errorf("first line: %q", first)
	}
}

// A name is the player's own: it must not format the post or break out of bold.
func TestANameCannotFormatThePost(t *testing.T) {
	notice := roundResultNotice(text.English, protocol.RoundResult{Winner: protocol.WinnerImpostors, Impostors: []string{"**x**", "a_b"}})

	if !strings.Contains(notice, `**\*\*x\*\***, **a\_b**`) {
		t.Errorf("names are not escaped: %s", notice)
	}
}

// Without a control channel, or before Discord is up, nothing is posted and
// nothing breaks.
func TestNoChannelNoPost(t *testing.T) {
	(&Bot{}).postRoundResult("guild", protocol.RoundResult{Winner: protocol.WinnerCrewmates})
}
