package permission

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

// The crewmate menu is an embed with a menu, posted by the bot. Each of these
// three permissions alone is enough to stop it from appearing.
func TestTheControlChannelNeedsToBeSeenPostedInAndEmbeddedIn(t *testing.T) {
	granted := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)

	if missing := Missing(granted, ControlChannelNeeds); len(missing) != 0 {
		t.Fatalf("nothing should be missing, got %+v", missing)
	}
	for _, need := range ControlChannelNeeds {
		missing := Missing(granted&^need.Bit, ControlChannelNeeds)
		if len(missing) != 1 || missing[0].Bit != need.Bit {
			t.Errorf("without %s: got %+v", need.Name, missing)
		}
		if need.Consequence == "" {
			t.Errorf("%s does not say what breaks", need.Name)
		}
	}
}
