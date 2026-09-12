package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/bot"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/pairing"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/transport"
)

func probe(t *testing.T, controller *bot.Bot) (int, string) {
	t.Helper()

	recorder := httptest.NewRecorder()
	health(controller)(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	return recorder.Code, recorder.Body.String()
}

// A health check that only proves the process is running is worth little: a bot
// that lost Discord is as useless as one that crashed, and only the crash
// restarts itself.
func TestHealthFailsWithoutADiscordConnection(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "auvc.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	controller := &bot.Bot{AUVC: au.NewService(db, "test", "test")}

	code, body := probe(t, controller)
	if code != http.StatusServiceUnavailable {
		t.Errorf("status %d, want %d", code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(body, "Discord") {
		t.Errorf("the reply does not say what is wrong: %q", body)
	}
}

// The same for the database, and the reply has to name which one failed:
// "unhealthy" on its own sends an operator reading logs they may not have kept.
func TestHealthFailsWithoutADatabase(t *testing.T) {
	controller := &bot.Bot{}

	code, body := probe(t, controller)
	if code != http.StatusServiceUnavailable {
		t.Errorf("status %d, want %d", code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(body, "database") {
		t.Errorf("the reply does not mention the database: %q", body)
	}
}

func TestHealthNamesEveryProblemAtOnce(t *testing.T) {
	_, body := probe(t, &bot.Bot{})

	if !strings.Contains(body, "Discord") || !strings.Contains(body, "database") {
		t.Errorf("a reply should list everything that is wrong: %q", body)
	}
}

// The capture endpoints and the health endpoint have to coexist: the health
// check lives on the same listener, so a mux that swallowed one would make the
// container unreachable or unmonitorable.
func TestHealthAndCaptureEndpointsShareTheListener(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "auvc.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	controller := &bot.Bot{AUVC: au.NewService(db, "test", "test")}
	handler := routes(transport.NewServer(pairing.NewService(db), controller, nil), controller)

	for path, unwanted := range map[string]int{
		"/healthz":      http.StatusNotFound,
		"/capture/pair": http.StatusNotFound,
		"/capture/link": http.StatusNotFound,
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

		if recorder.Code == unwanted {
			t.Errorf("%s is not routed", path)
		}
	}
}
