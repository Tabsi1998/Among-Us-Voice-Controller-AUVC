package bot

import (
	"reflect"
	"testing"
)

// Only a running session has put anybody in a state they need releasing from.
// A paused one was deliberately left as it is, and a stopped one already
// released everyone.
func TestOnlyRunningSessionsAreReleasedOnShutdown(t *testing.T) {
	sessions := NewCaptureSessions()
	sessions.SetMode("guild-c", Running)
	sessions.SetMode("guild-b", Paused)
	sessions.SetMode("guild-a", Running)
	sessions.SetMode("guild-d", Stopped)

	if got, want := sessions.RunningGuilds(), []string{"guild-a", "guild-c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("running guilds are %v, want %v", got, want)
	}
}

func TestNoSessionsMeansNothingToRelease(t *testing.T) {
	if got := NewCaptureSessions().RunningGuilds(); len(got) != 0 {
		t.Errorf("a fresh bot reported running guilds %v", got)
	}
}
