package command

import (
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
)

func TestAllRegistersAUExactlyOnce(t *testing.T) {
	count := 0
	for _, registered := range All {
		if registered.Name == au.Name {
			count++
		}
	}
	if count != 1 {
		t.Errorf("/au registration count = %d, want 1", count)
	}
}
