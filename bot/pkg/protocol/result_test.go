package protocol

import "testing"

// The names end up in a Discord channel, so a result that is not a lobby's is
// refused; one without a result, from an older capture, is fine.
func TestTheRoundResultIsChecked(t *testing.T) {
	refuse := func(reason string) *Error { return NewError("s", 1, CodeMalformed, reason) }
	tooMany := make([]string, MaxImpostors+1)
	for i := range tooMany {
		tooMany[i] = "x"
	}

	for name, situation := range map[string]struct {
		result *RoundResult
		ok     bool
	}{
		"none":           {nil, true},
		"crewmates":      {&RoundResult{Winner: WinnerCrewmates}, true},
		"unknown reason": {&RoundResult{Winner: WinnerImpostors, Reason: "meteor", Impostors: []string{"Mira"}}, true},
		"unknown winner": {&RoundResult{Winner: "nobody"}, false},
		"nameless":       {&RoundResult{Winner: WinnerImpostors, Impostors: []string{""}}, false},
		"too many":       {&RoundResult{Winner: WinnerImpostors, Impostors: tooMany}, false},
	} {
		if refused := validateResult(situation.result, refuse) != nil; refused == situation.ok {
			t.Errorf("%s: refused=%v", name, refused)
		}
	}
}
