package doctor

import (
	"strings"
	"testing"
)

func TestAnEmptyReportSaysSoRatherThanLookingHealthy(t *testing.T) {
	rendered := Report{}.Render()

	if strings.Contains(rendered, "✅") {
		t.Errorf("a report with no checks must not look like a pass: %q", rendered)
	}
}

func TestTheWorstLevelWins(t *testing.T) {
	cases := map[string]struct {
		report Report
		want   Level
	}{
		"all fine":      {Report{{Level: OK}, {Level: OK}}, OK},
		"one warning":   {Report{{Level: OK}, {Level: Warn}}, Warn},
		"one failure":   {Report{{Level: OK}, {Level: Warn}, {Level: Fail}}, Fail},
		"nothing there": {Report{}, OK},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := c.report.Worst(); got != c.want {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}

// The reader has to know before scrolling whether anything needs them.
func TestTheHeadlineSaysWhetherAnythingIsWrong(t *testing.T) {
	healthy := Report{{Name: "A", Level: OK, Detail: "fine"}}.Render()
	if !strings.HasPrefix(healthy, "✅") {
		t.Errorf("a healthy report should open with a pass: %q", healthy)
	}

	warned := Report{{Name: "A", Level: Warn, Detail: "incomplete"}}.Render()
	if !strings.HasPrefix(warned, "⚠️") {
		t.Errorf("a warning report should open with a warning: %q", warned)
	}
	if !strings.Contains(warned, "can still run") {
		t.Errorf("a warning should say AUVC still works: %q", warned)
	}

	broken := Report{{Name: "A", Level: Fail, Detail: "broken"}}.Render()
	if !strings.HasPrefix(broken, "❌") {
		t.Errorf("a failing report should open with a failure: %q", broken)
	}
}

func TestTheHeadlineCountsBothKinds(t *testing.T) {
	rendered := Report{
		{Name: "A", Level: Fail, Detail: "broken"},
		{Name: "B", Level: Warn, Detail: "incomplete"},
		{Name: "C", Level: OK, Detail: "fine"},
	}.Render()

	first := strings.SplitN(rendered, "\n", 2)[0]
	if !strings.Contains(first, "1 problem") || !strings.Contains(first, "1 warning") {
		t.Errorf("the headline should count both: %q", first)
	}
}

// A diagnosis that only shows problems leaves the reader unsure whether the
// rest was checked or skipped, which is the question they ran it to answer.
func TestPassingChecksAreListedToo(t *testing.T) {
	rendered := Report{
		{Name: "Discord", Level: OK, Detail: "connected"},
		{Name: "Capture", Level: Fail, Detail: "gone"},
	}.Render()

	if !strings.Contains(rendered, "Discord") {
		t.Errorf("a passing check was left out: %q", rendered)
	}
	if !strings.Contains(rendered, "Capture") {
		t.Errorf("a failing check was left out: %q", rendered)
	}
}

// A warning without a next step makes a reader feel worse without helping.
func TestAFixIsShownWhenThereIsOne(t *testing.T) {
	rendered := Report{
		{Name: "Capture", Level: Warn, Detail: "not paired", Fix: "Run `/au capture pair`."},
	}.Render()

	if !strings.Contains(rendered, "/au capture pair") {
		t.Errorf("the fix was not shown: %q", rendered)
	}
}

func TestNoArrowWhenThereIsNothingToDo(t *testing.T) {
	rendered := Report{{Name: "Discord", Level: OK, Detail: "connected"}}.Render()

	if strings.Contains(rendered, "→") {
		t.Errorf("a passing check should not suggest a fix: %q", rendered)
	}
}

// Failures come first, and checks of the same level keep the order they were
// added, so the report reads in the order AUVC depends on things.
func TestFailingChecksComeWorstFirstAndOtherwiseInOrder(t *testing.T) {
	failing := Report{
		{Name: "first-warning", Level: Warn},
		{Name: "ok", Level: OK},
		{Name: "first-failure", Level: Fail},
		{Name: "second-warning", Level: Warn},
		{Name: "second-failure", Level: Fail},
	}.Failing()

	var names []string
	for _, check := range failing {
		names = append(names, check.Name)
	}

	want := []string{"first-failure", "second-failure", "first-warning", "second-warning"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", names, want)
	}
}

func TestEveryLevelHasAnIconAndAName(t *testing.T) {
	for _, level := range []Level{OK, Warn, Fail} {
		if level.icon() == "" {
			t.Errorf("%s has no icon", level)
		}
		if level.String() == "" {
			t.Errorf("a level has no name")
		}
	}
}
