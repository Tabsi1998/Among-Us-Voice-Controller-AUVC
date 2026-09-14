package permission

import (
	"strings"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
	"github.com/bwmarrin/discordgo"
)

const all = discordgo.PermissionViewChannel |
	discordgo.PermissionVoiceConnect |
	discordgo.PermissionVoiceMoveMembers |
	discordgo.PermissionVoiceMuteMembers |
	discordgo.PermissionVoiceDeafenMembers

func TestNothingMissingWhenEverythingIsGranted(t *testing.T) {
	if missing := Missing(all, VoiceChannelNeeds); len(missing) != 0 {
		t.Errorf("expected nothing missing, got %+v", missing)
	}
}

func TestEveryNeedIsDetectedOnItsOwn(t *testing.T) {
	for _, need := range VoiceChannelNeeds {
		missing := Missing(all&^need.Bit, VoiceChannelNeeds)

		if len(missing) != 1 {
			t.Fatalf("removing %s reported %d problems, want 1: %+v", need.Name, len(missing), missing)
		}
		if missing[0].Name != need.Name {
			t.Errorf("removing %s reported %s", need.Name, missing[0].Name)
		}
	}
}

func TestAllNeedsMissingWhenNothingIsGranted(t *testing.T) {
	if missing := Missing(0, VoiceChannelNeeds); len(missing) != len(VoiceChannelNeeds) {
		t.Errorf("expected all %d needs, got %d", len(VoiceChannelNeeds), len(missing))
	}
}

// Discord treats Administrator as granting everything. Reporting a permission
// as missing when the bot can in fact use it sends an administrator hunting for
// a problem that is not there.
func TestAdministratorCoversEverything(t *testing.T) {
	if missing := Missing(discordgo.PermissionAdministrator, VoiceChannelNeeds); len(missing) != 0 {
		t.Errorf("Administrator must cover every need, got %+v", missing)
	}
}

// Every need has to explain itself in words an administrator can act on. A list
// of permission names without consequences is a puzzle, not a report.
func TestEveryNeedExplainsWhatBreaks(t *testing.T) {
	for _, need := range VoiceChannelNeeds {
		if need.Name == "" {
			t.Error("a need has no name")
		}
		if need.Consequence == "" {
			t.Errorf("%s does not say what breaks without it", need.Name)
		}
		if need.Bit == 0 {
			t.Errorf("%s has no permission bit", need.Name)
		}
	}
}

func TestAuditReportsPerChannel(t *testing.T) {
	reports := Audit(
		Channel{Purpose: text.PurposeMainChannel, ID: "main", Effective: all},
		Channel{Purpose: text.PurposeGhostChannel, ID: "ghost", Effective: all &^ discordgo.PermissionVoiceMoveMembers},
	)

	if len(reports) != 1 {
		t.Fatalf("expected only the ghost channel to be reported, got %+v", reports)
	}
	if reports[0].Channel.ID != "ghost" {
		t.Errorf("wrong channel reported: %+v", reports[0].Channel)
	}
	if len(reports[0].Missing) != 1 || reports[0].Missing[0].Name != MoveMembers.Name {
		t.Errorf("expected Move Members to be missing, got %+v", reports[0].Missing)
	}
}

// A channel that was never configured is a setup question, not a permission
// one. Reporting five missing permissions on a channel that does not exist
// would bury the actual advice, which is to run /au setup channels.
func TestAuditSkipsUnconfiguredChannels(t *testing.T) {
	reports := Audit(
		Channel{Purpose: text.PurposeMainChannel, ID: "", Effective: 0},
		Channel{Purpose: text.PurposeGhostChannel, ID: "", Effective: 0},
	)

	if len(reports) != 0 {
		t.Errorf("an unconfigured channel must not produce a permission report, got %+v", reports)
	}
}

func TestSummaryIsEmptyWhenNothingIsMissing(t *testing.T) {
	if got := Summary(text.English, nil); got != "" {
		t.Errorf("expected an empty summary, got %q", got)
	}
	if got := Summary(text.English, Audit(Channel{Purpose: "main", ID: "main", Effective: all})); got != "" {
		t.Errorf("expected an empty summary, got %q", got)
	}
}

func TestSummaryNamesTheConsequence(t *testing.T) {
	reports := Audit(Channel{
		Purpose:   text.PurposeGhostChannel,
		ID:        "ghost",
		Effective: all &^ discordgo.PermissionVoiceMoveMembers,
	})

	summary := Summary(text.English, reports)
	if !strings.Contains(summary, MoveMembers.Name) {
		t.Errorf("summary does not name the permission: %q", summary)
	}
	if !strings.Contains(summary, "ghost voice channel") {
		t.Errorf("summary does not name the channel: %q", summary)
	}
	if !strings.Contains(summary, "ghost channel") {
		t.Errorf("summary does not explain what breaks: %q", summary)
	}
}

// An administrator fixes a permission once, usually on the role. Explaining the
// same missing permission separately per channel makes the report look like two
// problems.
func TestSummaryGroupsAChannelPairUnderOnePermission(t *testing.T) {
	reports := Audit(
		Channel{Purpose: text.PurposeMainChannel, ID: "main", Effective: all &^ discordgo.PermissionVoiceMoveMembers},
		Channel{Purpose: text.PurposeGhostChannel, ID: "ghost", Effective: all &^ discordgo.PermissionVoiceMoveMembers},
	)

	summary := Summary(text.English, reports)
	if got := strings.Count(summary, MoveMembers.Name); got != 1 {
		t.Errorf("Move Members should be explained once, appeared %d times:\n%s", got, summary)
	}
	if !strings.Contains(summary, "main voice channel") || !strings.Contains(summary, "ghost voice channel") {
		t.Errorf("both channels should be named:\n%s", summary)
	}
}

func TestSummaryListsEveryMissingPermission(t *testing.T) {
	summary := Summary(text.English, Audit(Channel{Purpose: text.PurposeMainChannel, ID: "main", Effective: 0}))

	for _, need := range VoiceChannelNeeds {
		if !strings.Contains(summary, need.Name) {
			t.Errorf("%s is missing from the summary:\n%s", need.Name, summary)
		}
	}
}

// A German administrator reads the whole summary in German; only Discord's own
// permission names stay as Discord documents them.
func TestSummaryIsWrittenInTheLanguageAskedFor(t *testing.T) {
	summary := Summary(text.German, Audit(Channel{
		Purpose:   text.PurposeGhostChannel,
		ID:        "ghost",
		Effective: all &^ discordgo.PermissionVoiceMoveMembers,
	}))

	for _, expected := range []string{
		text.German.Say(text.PermissionsMissing),
		MoveMembers.Name,
		text.German.Say(text.PurposeGhostChannel),
		text.German.Say(text.WithoutMoveMembers),
	} {
		if !strings.Contains(summary, expected) {
			t.Errorf("the German summary is missing %q:\n%s", expected, summary)
		}
	}
}

// The order must be stable, or the same problem produces a different message on
// every call and nobody can tell whether anything changed.
func TestSummaryIsStable(t *testing.T) {
	channels := []Channel{
		{Purpose: text.PurposeMainChannel, ID: "main", Effective: 0},
		{Purpose: text.PurposeGhostChannel, ID: "ghost", Effective: 0},
	}

	first := Summary(text.English, Audit(channels...))
	for i := 0; i < 5; i++ {
		if again := Summary(text.English, Audit(channels...)); again != first {
			t.Fatalf("summary changed between calls:\n%s\n---\n%s", first, again)
		}
	}
}
