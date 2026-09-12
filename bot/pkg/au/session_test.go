package au

import (
	"errors"
	"strings"
	"testing"
)

// recordingController answers every session command with its own name, so the
// routing can be checked without a bot behind it.
type recordingController struct {
	calls []string
}

func (r *recordingController) record(name, guildID string) (string, error) {
	r.calls = append(r.calls, name+":"+guildID)
	return "handled " + name, nil
}

func (r *recordingController) Start(guildID string) (string, error) {
	return r.record("start", guildID)
}
func (r *recordingController) Stop(guildID string) (string, error) { return r.record("stop", guildID) }
func (r *recordingController) Pause(guildID string) (string, error) {
	return r.record("pause", guildID)
}
func (r *recordingController) Resume(guildID string) (string, error) {
	return r.record("resume", guildID)
}
func (r *recordingController) Status(guildID string) (string, error) {
	return r.record("status", guildID)
}

func TestEverySessionCommandReachesTheController(t *testing.T) {
	service, _ := testService(t)
	controller := &recordingController{}
	service.AttachSessionControl(controller)

	for _, command := range []string{SessionStart, SessionStop, SessionPause, SessionResume, SessionStatus} {
		reply, err := service.Handle(request(GroupSession, command))
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
		if !strings.HasSuffix(call, ":guild") {
			t.Errorf("a command reached the controller without its guild: %s", call)
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
