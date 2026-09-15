package bot

import (
	"errors"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/voice"
)

// A move into no channel removes the member from voice. The applier refuses it
// before Discord is asked, so the zero applier here, which has no session,
// must return the refusal rather than reach for one (#160).
func TestDiscordApplierRefusesAMoveIntoNoChannel(t *testing.T) {
	nowhere := ""

	err := discordApplier{}.Apply("guild", voice.Change{UserID: "console", MoveTo: &nowhere})

	if !errors.Is(err, errWouldDisconnect) {
		t.Fatalf("a move to an empty channel must be refused, got %v", err)
	}
}

func TestRefuseDisconnectLetsRealChangesThrough(t *testing.T) {
	main := "main"
	muted := true
	for _, change := range []voice.Change{
		{UserID: "a", MoveTo: &main},
		{UserID: "a", Muted: &muted},
		{UserID: "a"},
	} {
		if err := refuseDisconnect(change); err != nil {
			t.Errorf("%+v must not be refused: %v", change, err)
		}
	}
}

func TestDescribeChangeNamesWhatIsAsked(t *testing.T) {
	ghost := "ghost"
	on, off := true, false

	cases := []struct {
		change voice.Change
		want   string
	}{
		{voice.Change{MoveTo: &ghost}, "move to channel ghost"},
		{voice.Change{Muted: &on, Deafened: &on}, "server mute, server deafen"},
		{voice.Change{Muted: &off, Deafened: &off}, "lift server mute, lift server deafen"},
		{voice.Change{MoveTo: &ghost, Muted: &off}, "move to channel ghost, lift server mute"},
		{voice.Change{}, "nothing"},
	}
	for _, c := range cases {
		if got := describeChange(c.change); got != c.want {
			t.Errorf("describeChange(%+v) = %q, want %q", c.change, got, c.want)
		}
	}
}

func TestDescribeVoiceMoveLogsOnlyMovesInAUVCsChannels(t *testing.T) {
	config := voice.Config{MainChannelID: "main", GhostChannelID: "ghost"}

	cases := []struct {
		name          string
		before, after string
		want          string
		worth         bool
	}{
		{"joins the main channel", "", "main", "joined voice channel main", true},
		{"leaves the main channel", "main", "", "left voice channel main", true},
		{"leaves the ghost channel", "ghost", "", "left voice channel ghost", true},
		{"switches into the ghost channel", "main", "ghost", "switched from voice channel main to ghost", true},
		{"comes from another channel", "lounge", "main", "switched from voice channel lounge to main", true},
		{"stays in the channel", "main", "main", "", false},
		{"moves between other channels", "lounge", "music", "", false},
		{"joins another channel", "", "lounge", "", false},
	}
	for _, c := range cases {
		got, worth := describeVoiceMove(c.before, c.after, config)
		if got != c.want || worth != c.worth {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", c.name, got, worth, c.want, c.worth)
		}
	}
}

// A guild without channels has nothing of AUVC's to watch, and an empty
// channel id must never count as one of them.
func TestDescribeVoiceMoveWithoutChannelsLogsNothing(t *testing.T) {
	if what, worth := describeVoiceMove("", "main", voice.Config{}); worth {
		t.Errorf("no channels are configured, yet it logged %q", what)
	}
}
