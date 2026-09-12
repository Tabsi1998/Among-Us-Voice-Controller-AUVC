// Package doctor turns what AUVC knows about itself into a report a person can
// act on.
//
// It holds no connections and asks nothing: the caller gathers the facts and
// this decides how they read. That split is what lets every line of the report
// be tested, and it is why the rendering never has to guard against a Discord
// call failing halfway through.
//
// Nothing here ever receives a secret. The checks describe whether something
// works, not what it was configured with, so there is no credential for a
// report to leak into a Discord channel.
package doctor

import (
	"fmt"
	"sort"
	"strings"
)

// Level is how a check turned out.
type Level int

const (
	// OK means the check passed and nothing is needed.
	OK Level = iota
	// Warn means AUVC works but something is missing or stale. A guild that has
	// not finished setting up lives here, and so does a capture that has not
	// connected yet: neither is broken, both are incomplete.
	Warn
	// Fail means this stops AUVC from doing its job.
	Fail
)

func (l Level) icon() string {
	switch l {
	case Fail:
		return "❌"
	case Warn:
		return "⚠️"
	default:
		return "✅"
	}
}

func (l Level) String() string {
	switch l {
	case Fail:
		return "fail"
	case Warn:
		return "warn"
	default:
		return "ok"
	}
}

// Check is one thing the doctor looked at.
type Check struct {
	// Name is the subject, short enough to scan a column of them.
	Name string
	// Level is the verdict.
	Level Level
	// Detail says what was found, in words rather than codes.
	Detail string
	// Fix is what to do about it, and is empty when there is nothing to do.
	// A warning without a next step makes a reader feel worse without helping.
	Fix string
}

// Report is the whole diagnosis, in the order the checks were added.
type Report []Check

// Add appends a check.
func (r *Report) Add(check Check) {
	*r = append(*r, check)
}

// Worst returns the most serious level in the report.
func (r Report) Worst() Level {
	worst := OK
	for _, check := range r {
		if check.Level > worst {
			worst = check.Level
		}
	}
	return worst
}

// Failing returns the checks that are not OK, worst first.
//
// Ties keep the order they were added, so the report reads in the order AUVC
// actually depends on things: no Discord connection is reported before the
// channels that need it.
func (r Report) Failing() []Check {
	failing := make([]Check, 0, len(r))
	for _, check := range r {
		if check.Level != OK {
			failing = append(failing, check)
		}
	}

	sort.SliceStable(failing, func(i, j int) bool {
		return failing[i].Level > failing[j].Level
	})
	return failing
}

// Render writes the report for Discord.
//
// Every check is listed, passing ones included. A diagnosis that only shows
// problems leaves the reader unsure whether the rest was checked or skipped,
// which is the question they ran the command to answer.
func (r Report) Render() string {
	if len(r) == 0 {
		return "⚠️ Nothing was checked."
	}

	var builder strings.Builder
	builder.WriteString(r.headline())
	builder.WriteString("\n")

	for _, check := range r {
		builder.WriteString(fmt.Sprintf("\n%s **%s**: %s", check.Level.icon(), check.Name, check.Detail))
		if check.Fix != "" {
			builder.WriteString("\n   → " + check.Fix)
		}
	}
	return builder.String()
}

// headline says in one line whether anything needs attention, so a reader knows
// before scrolling.
func (r Report) headline() string {
	var failed, warned int
	for _, check := range r {
		switch check.Level {
		case Fail:
			failed++
		case Warn:
			warned++
		}
	}

	switch {
	case failed > 0 && warned > 0:
		return fmt.Sprintf("❌ %d problem(s) and %d warning(s).", failed, warned)
	case failed > 0:
		return fmt.Sprintf("❌ %d problem(s).", failed)
	case warned > 0:
		return fmt.Sprintf("⚠️ %d warning(s); AUVC can still run.", warned)
	default:
		return "✅ Everything checks out."
	}
}
