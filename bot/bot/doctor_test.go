package bot

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/pairing"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
)

// doctorBot builds a bot with real storage and no Discord connection, which is
// the worst case the report has to survive: it has to say what is wrong rather
// than fail while trying to find out.
func doctorBot(t *testing.T) (*Doctor, *sqlite.DB) {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "auvc.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	service := au.NewServiceWithPairing(db, pairing.NewService(db), "v1.2.3-test", "abcdef")
	controller := &Bot{AUVC: service, CaptureSessions: NewCaptureSessions(), AUVCLinks: db}
	return NewDoctor(controller), db
}

func diagnose(t *testing.T, doctor *Doctor) string {
	t.Helper()

	report, err := doctor.Diagnose(guild, text.English)
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	return report
}

// A bot that cannot reach Discord must still produce a report. Failing to
// diagnose is the one thing a diagnosis may not do.
func TestTheReportSurvivesHavingNoDiscordConnection(t *testing.T) {
	doctor, _ := doctorBot(t)

	report := diagnose(t, doctor)

	if report == "" {
		t.Fatal("the doctor produced nothing")
	}
	if !strings.Contains(report, "Discord") {
		t.Errorf("the report does not mention Discord: %s", report)
	}
}

// The database and the build are what a bug report needs first and what people
// forget to include.
func TestTheReportNamesTheDatabaseAndTheBuild(t *testing.T) {
	doctor, _ := doctorBot(t)

	report := diagnose(t, doctor)

	if !strings.Contains(report, "schema migration") {
		t.Errorf("the migration state is missing: %s", report)
	}
	if !strings.Contains(report, "v1.2.3-test") {
		t.Errorf("the build version is missing: %s", report)
	}
}

// A fresh guild is incomplete, not broken, and the report has to say what to do
// about it rather than only that something is missing.
func TestAFreshGuildIsToldWhatToSetUp(t *testing.T) {
	doctor, _ := doctorBot(t)

	report := diagnose(t, doctor)

	for _, expected := range []string{"/au setup channels", "/au capture pair"} {
		if !strings.Contains(report, expected) {
			t.Errorf("the report does not say to run %s:\n%s", expected, report)
		}
	}
}

func TestAnUnpairedCaptureIsAWarningNotAFailure(t *testing.T) {
	doctor, _ := doctorBot(t)

	report := diagnose(t, doctor)

	line := lineContaining(t, report, "Capture")
	if !strings.HasPrefix(strings.TrimSpace(line), "⚠️") {
		t.Errorf("an unpaired capture should warn, not fail: %q", line)
	}
}

// Once a game is running the report has to say what AUVC believes, because
// "the bot is doing nothing" and "the bot sees no game" look identical from
// the outside.
func TestTheReportSaysWhatAUVCBelievesAboutTheGame(t *testing.T) {
	doctor, _ := doctorBot(t)

	if err := doctor.bot.HandleCapture(guild, &protocol.Snapshot{
		Header:  protocol.Header{Session: "s1"},
		Phase:   protocol.PhaseTasks,
		Players: []protocol.Player{{Name: "Red"}, {Name: "Blue", Dead: true}},
	}); err != nil {
		t.Fatalf("handling a snapshot: %v", err)
	}

	report := diagnose(t, doctor)
	line := lineContaining(t, report, "Game state")

	if !strings.Contains(line, "1 alive") || !strings.Contains(line, "1 dead") {
		t.Errorf("the report does not describe the round: %q", line)
	}
}

// A session that is only tracked, not applied, has to be visible. Otherwise
// "AUVC is not muting anyone" has no explanation in the report.
func TestAStoppedSessionIsReportedAsAWarning(t *testing.T) {
	doctor, _ := doctorBot(t)

	if err := doctor.bot.HandleCapture(guild, &protocol.Snapshot{
		Header:  protocol.Header{Session: "s1"},
		Phase:   protocol.PhaseTasks,
		Players: []protocol.Player{{Name: "Red"}},
	}); err != nil {
		t.Fatalf("handling a snapshot: %v", err)
	}

	line := lineContaining(t, diagnose(t, doctor), "Game state")
	if !strings.Contains(line, "stopped") {
		t.Errorf("the report does not say the session is stopped: %q", line)
	}
	if !strings.Contains(diagnose(t, doctor), "/au session start") {
		t.Error("the report does not say how to start managing voice")
	}
}

// The report reaches a Discord channel. It describes whether things work, never
// what they were configured with, so there is nothing in it to leak.
func TestTheReportCarriesNoCredential(t *testing.T) {
	doctor, db := doctorBot(t)

	code, _, err := pairing.NewService(db).Pair(guild, "admin")
	if err != nil {
		t.Fatalf("pair: %v", err)
	}
	stored, err := db.Pairing(guild)
	if err != nil {
		t.Fatalf("read pairing: %v", err)
	}

	report := diagnose(t, doctor)
	for _, secret := range []string{code.Display(), code.Secret.Reveal(), stored.CodeHash} {
		if strings.Contains(report, secret) {
			t.Errorf("the report leaked a secret:\n%s", report)
		}
	}
}

func lineContaining(t *testing.T, report, needle string) string {
	t.Helper()

	for _, line := range strings.Split(report, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	t.Fatalf("no line mentions %q:\n%s", needle, report)
	return ""
}
