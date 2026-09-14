package bot

import (
	"errors"
	"strings"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
)

// Everybody in the text channel reads the warning, so it comes in the server's
// language and says what AUVC did about the silence.
func TestTheCaptureWarningSaysWhatAUVCDid(t *testing.T) {
	for _, situation := range []struct {
		name   string
		action string
		says   []text.Key
	}{
		{"paused", au.CaptureTimeoutPause, []text.Key{text.NoticeCaptureStopped, text.NoticePausedByChoice}},
		{"released", au.CaptureTimeoutFailOpen, []text.Key{text.NoticeCaptureStopped, text.NoticeReleased, text.NoticeResumesOnItsOwn}},
	} {
		for _, language := range text.Languages {
			notice := captureStoppedNotice(language, situation.action, nil)
			for _, key := range situation.says {
				if !strings.Contains(notice, language.Say(key)) {
					t.Errorf("%s, %s: the warning does not say %q: %s", situation.name, language, key, notice)
				}
			}
		}
	}
}

// A release that failed must not be reported as done.
func TestAFailedReleaseIsNotReportedAsDone(t *testing.T) {
	notice := captureStoppedNotice(text.German, au.CaptureTimeoutFailOpen, errors.New("Discord antwortet nicht"))

	if !strings.Contains(notice, text.German.Say(text.NoticeReleaseFailed, "Discord antwortet nicht")) {
		t.Errorf("the failure is not in the warning: %s", notice)
	}
	if strings.Contains(notice, text.German.Say(text.NoticeReleased)) {
		t.Errorf("the warning claims everyone was released: %s", notice)
	}
}
