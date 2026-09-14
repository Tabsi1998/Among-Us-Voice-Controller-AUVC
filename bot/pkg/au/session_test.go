package au

import (
	"errors"
	"strings"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
)

// recordingController answers every session command with its own name, so the
// routing can be checked without a bot behind it.
type recordingController struct {
	calls []string
}

func (r *recordingController) record(name, guildID string, language text.Language) (string, error) {
	r.calls = append(r.calls, name+":"+guildID+":"+string(language))
	return "handled " + name, nil
}

func (r *recordingController) Start(guildID string, language text.Language) (string, error) {
	return r.record("start", guildID, language)
}
func (r *recordingController) Stop(guildID string, language text.Language) (string, error) {
	return r.record("stop", guildID, language)
}
func (r *recordingController) Pause(guildID string, language text.Language) (string, error) {
	return r.record("pause", guildID, language)
}
func (r *recordingController) Resume(guildID string, language text.Language) (string, error) {
	return r.record("resume", guildID, language)
}
func (r *recordingController) Status(guildID string, language text.Language) (string, error) {
	return r.record("status", guildID, language)
}

func TestEverySessionCommandReachesTheController(t *testing.T) {
	service, _ := testService(t)
	controller := &recordingController{}
	service.AttachSessionControl(controller)

	for _, command := range []string{SessionStart, SessionStop, SessionPause, SessionResume, SessionStatus} {
		asked := request(GroupSession, command)
		asked.Language = text.German
		reply, err := service.Handle(asked)
		if err != nil {
			t.Fatalf("%s: %v", command, err)
		}
		if !strings.Contains(reply, command) {
			t.Errorf("%s produced %q", command, reply)
		}
	}

	if len(controller.calls) != 5 {
		t.Errorf("expected five calls, got %v", controller.calls)
	}
	for _, call := range controller.calls {
		if !strings.HasSuffix(call, ":guild:de") {
			t.Errorf("a command reached the controller without its guild or language: %s", call)
		}
	}
}

func TestAnUnknownSessionCommandIsRefused(t *testing.T) {
	service, _ := testService(t)
	service.AttachSessionControl(&recordingController{})

	if _, err := service.Handle(request(GroupSession, "rewind")); !errors.Is(err, ErrUnknownPath) {
		t.Errorf("got %v, want %v", err, ErrUnknownPath)
	}
}

// A build without session control still has to answer, rather than failing in a
// way that looks like a broken command.
func TestSessionCommandsReportThemselvesWhenControlIsUnavailable(t *testing.T) {
	service, _ := testService(t)

	for _, command := range []string{SessionStart, SessionStop, SessionStatus} {
		reply, err := service.Handle(request(GroupSession, command))
		if err != nil {
			t.Fatalf("%s: %v", command, err)
		}
		if !strings.Contains(reply, command) {
			t.Errorf("%s did not name itself: %s", command, reply)
		}
	}
}

// Every /au reply reaches only the member who asked, so it is written in their
// Discord language.
func TestARepliesInTheLanguageOfWhoeverAsked(t *testing.T) {
	service, _ := testService(t)

	link := request("", Link)
	link.Values.Strings[OptionPlayer] = "Red"
	link.Language = text.German
	reply, err := service.Handle(link)
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	if want := text.German.Say(text.Linked, "owner", "Red"); reply != want {
		t.Errorf("got %q, want %q", reply, want)
	}

	show := request(GroupSettings, SettingsShow)
	show.Language = text.German
	settings, err := service.Handle(show)
	if err != nil {
		t.Fatalf("settings show: %v", err)
	}
	for _, expected := range []string{"AUVC-Einstellungen", "Hauptkanal", "nicht gesetzt", "ja"} {
		if !strings.Contains(settings, expected) {
			t.Errorf("the German settings are missing %q: %s", expected, settings)
		}
	}
}

// A member whose Discord language AUVC does not speak, or a request that names
// none, is answered in English.
func TestARequestWithoutALanguageIsAnsweredInEnglish(t *testing.T) {
	service, _ := testService(t)

	link := request("", Link)
	link.Values.Strings[OptionPlayer] = "Red"
	reply, err := service.Handle(link)
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	if want := text.English.Say(text.Linked, "owner", "Red"); reply != want {
		t.Errorf("got %q, want %q", reply, want)
	}
}
