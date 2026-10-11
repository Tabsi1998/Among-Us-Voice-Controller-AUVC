package au

import (
	"errors"
	"strings"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
)

// The result after a round is on until an admin switches it off.
func TestRoundResultsCanBeSwitchedOff(t *testing.T) {
	service, db := testService(t)
	if config, _ := db.EnsureGuildConfig("guild"); !config.PostRoundResults {
		t.Fatal("a new server should get the result after a round")
	}

	off := request(GroupSettings, SettingsRound)
	off.Values.Booleans[OptionRoundResults] = false
	if _, err := service.Handle(off); err != nil {
		t.Fatalf("settings round: %v", err)
	}

	if config, _ := db.GuildConfig("guild"); config.PostRoundResults {
		t.Error("round results are still on")
	}
}

// Everyone may read the stats; only an admin may forget them.
func TestStatsAreForEveryoneAndTheResetForAdmins(t *testing.T) {
	service, db := testService(t)
	if err := db.RecordRound("guild", []sqlite.RoundPlayer{{Name: "Mira", Impostor: true, Won: true}, {Name: "Paula"}}); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordRound("guild", []sqlite.RoundPlayer{{Name: "mira", Won: true}}); err != nil {
		t.Fatal(err)
	}

	member := request("", Stats)
	member.Language = text.English
	member.Invoker = Invoker{UserID: "member"}
	reply, err := service.Handle(member)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if want := text.English.Say(text.StatsLine, "Mira", 2, 2, 1); !strings.Contains(reply, want) {
		t.Errorf("stats do not say %q:\n%s", want, reply)
	}
	if strings.Index(reply, "Mira") > strings.Index(reply, "Paula") {
		t.Errorf("most rounds should come first:\n%s", reply)
	}

	member.Values.Booleans[OptionReset] = true
	if _, err := service.Handle(member); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("a member reset: %v", err)
	}
	owner := request("", Stats)
	owner.Values.Booleans[OptionReset] = true
	if _, err := service.Handle(owner); err != nil {
		t.Fatalf("owner reset: %v", err)
	}
	if stats, _ := db.PlayerStats("guild", 10); len(stats) != 0 {
		t.Errorf("reset left %+v", stats)
	}
}

// A name formats nothing in the stats either.
func TestStatsEscapeNames(t *testing.T) {
	if got := EscapeMarkdown("**@everyone_"); got != `\*\*\@everyone\_` {
		t.Errorf("escaped %q", got)
	}
}
