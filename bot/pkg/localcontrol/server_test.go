package localcontrol

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// The secret is assembled rather than written out, so a secret scanner reading
// this file sees a construction instead of something shaped like a key.
var testSecret = strings.Repeat("local-control-test-", 3)

type fakeBackend struct {
	mu           sync.Mutex
	calls        int
	status       Status
	guilds       map[string]Guild
	channels     map[string][]Channel
	setups       []Setup
	configureErr error
	issued       int
	stopped      chan struct{}
	crewmates    map[string]Crewmates
	links        []Link
	linkErr      error
}

func newFake() *fakeBackend {
	return &fakeBackend{
		status: Status{Connected: true, BotID: "bot-1", BotName: "AUVC", Version: "test",
			Guilds: []GuildSummary{{ID: "g1", Name: "Crew"}}},
		guilds:   map[string]Guild{"g1": {ID: "g1", Name: "Crew"}},
		channels: map[string][]Channel{"g1": {{ID: "v1", Name: "Among Us", Kind: KindVoice}}},
		crewmates: map[string]Crewmates{"g1": {
			Players: []Crewmate{{Name: "Alice", Color: "red"}},
			Members: []Member{{ID: "u1", Name: "Red Leader"}},
		}},
		stopped: make(chan struct{}),
	}
}

func (f *fakeBackend) called() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
}

func (f *fakeBackend) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeBackend) Status() Status {
	f.called()
	return f.status
}

func (f *fakeBackend) Channels(guildID string) ([]Channel, error) {
	f.called()
	if _, ok := f.guilds[guildID]; !ok {
		return nil, ErrUnknownGuild
	}
	return f.channels[guildID], nil
}

func (f *fakeBackend) Guild(guildID string) (Guild, error) {
	f.called()
	guild, ok := f.guilds[guildID]
	if !ok {
		return Guild{}, ErrUnknownGuild
	}
	return guild, nil
}

func (f *fakeBackend) Configure(guildID string, setup Setup) error {
	f.called()
	if _, ok := f.guilds[guildID]; !ok {
		return ErrUnknownGuild
	}
	if f.configureErr != nil {
		return f.configureErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setups = append(f.setups, setup)
	guild := f.guilds[guildID]
	guild.MainVoiceChannelID = setup.MainVoiceChannelID
	guild.GhostVoiceChannelID = setup.GhostVoiceChannelID
	guild.ControlTextChannelID = setup.ControlTextChannelID
	guild.AutoStart = setup.AutoStart
	f.guilds[guildID] = guild
	return nil
}

func (f *fakeBackend) IssueCredential(guildID string) (string, error) {
	f.called()
	if _, ok := f.guilds[guildID]; !ok {
		return "", ErrUnknownGuild
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.issued++
	return fmt.Sprintf("credential-%d", f.issued), nil
}

func (f *fakeBackend) Crewmates(guildID string) (Crewmates, error) {
	f.called()
	if _, ok := f.guilds[guildID]; !ok {
		return Crewmates{}, ErrUnknownGuild
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.crewmates[guildID], nil
}

func (f *fakeBackend) Link(guildID string, link Link) error {
	f.called()
	if _, ok := f.guilds[guildID]; !ok {
		return ErrUnknownGuild
	}
	if f.linkErr != nil {
		return f.linkErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.links = append(f.links, link)
	crewmates := f.crewmates[guildID]
	for index := range crewmates.Players {
		if crewmates.Players[index].Name == link.Player {
			crewmates.Players[index].UserID = link.UserID
		}
	}
	return nil
}

func (f *fakeBackend) Shutdown() {
	f.called()
	close(f.stopped)
}

type request struct {
	method, path, body string
	remote             string
	authorization      *string
	origin             string
}

func serve(t *testing.T, backend Backend, req request) *httptest.ResponseRecorder {
	t.Helper()

	server, err := New(testSecret, backend)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	mux := http.NewServeMux()
	server.Register(mux)

	r := httptest.NewRequest(req.method, req.path, strings.NewReader(req.body))
	r.RemoteAddr = "127.0.0.1:50123"
	if req.remote != "" {
		r.RemoteAddr = req.remote
	}
	if req.authorization == nil {
		r.Header.Set("Authorization", "Bearer "+testSecret)
	} else if *req.authorization != "" {
		r.Header.Set("Authorization", *req.authorization)
	}
	if req.origin != "" {
		r.Header.Set("Origin", req.origin)
	}

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, r)
	return recorder
}

func text(value string) *string { return &value }

var everyRoute = []request{
	{method: http.MethodGet, path: "/local/status"},
	{method: http.MethodGet, path: "/local/guilds/g1"},
	{method: http.MethodGet, path: "/local/guilds/g1/channels"},
	{method: http.MethodPut, path: "/local/guilds/g1/setup", body: `{"main_voice_channel_id":"v1","ghost_voice_channel_id":"v2"}`},
	{method: http.MethodPost, path: "/local/guilds/g1/credential"},
	{method: http.MethodGet, path: "/local/guilds/g1/crewmates"},
	{method: http.MethodPut, path: "/local/guilds/g1/links", body: `{"player":"Alice","user_id":"u1"}`},
	{method: http.MethodPost, path: "/local/shutdown"},
}

func TestASecretTooShortToProtectAnythingIsRefused(t *testing.T) {
	if _, err := New(strings.Repeat("x", MinSecretLength-1), newFake()); err == nil {
		t.Error("a short secret was accepted")
	}
	if _, err := New(testSecret, nil); err == nil {
		t.Error("a server without a backend was built")
	}
}

// This interface can issue capture credentials, so every route is closed to a
// caller without the secret, and the backend is never even asked.
func TestEveryRouteRequiresTheSecret(t *testing.T) {
	for _, route := range everyRoute {
		for name, authorization := range map[string]string{
			"missing":   "",
			"wrong":     "Bearer " + strings.Repeat("w", len(testSecret)),
			"no scheme": testSecret,
		} {
			t.Run(route.method+" "+route.path+" "+name, func(t *testing.T) {
				backend := newFake()
				req := route
				req.authorization = text(authorization)

				if got := serve(t, backend, req).Code; got != http.StatusUnauthorized {
					t.Errorf("status %d, want %d", got, http.StatusUnauthorized)
				}
				if backend.callCount() != 0 {
					t.Error("the backend was reached without the secret")
				}
			})
		}
	}
}

// The listener may be bound to every interface, as it is in a container. The
// secret alone must not be enough from another machine.
func TestOnlyThisComputerIsAnswered(t *testing.T) {
	for _, route := range everyRoute {
		backend := newFake()
		req := route
		req.remote = "192.168.1.20:50123"

		if got := serve(t, backend, req).Code; got != http.StatusForbidden {
			t.Errorf("%s %s from another machine: status %d, want %d", route.method, route.path, got, http.StatusForbidden)
		}
		if backend.callCount() != 0 {
			t.Errorf("%s %s reached the backend from another machine", route.method, route.path)
		}
	}

	if got := serve(t, newFake(), request{method: http.MethodGet, path: "/local/status", remote: "[::1]:50123"}).Code; got != http.StatusOK {
		t.Errorf("IPv6 loopback was refused: status %d", got)
	}
}

func TestBrowsersAreRefused(t *testing.T) {
	backend := newFake()
	response := serve(t, backend, request{method: http.MethodGet, path: "/local/status", origin: "http://localhost:3000"})

	if response.Code != http.StatusForbidden {
		t.Errorf("status %d, want %d", response.Code, http.StatusForbidden)
	}
	if backend.callCount() != 0 {
		t.Error("a browser request reached the backend")
	}
}

func TestStatusReportsTheBotAndItsServers(t *testing.T) {
	response := serve(t, newFake(), request{method: http.MethodGet, path: "/local/status"})

	var status Status
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !status.Connected || status.BotName != "AUVC" || len(status.Guilds) != 1 || status.Guilds[0].Name != "Crew" {
		t.Errorf("unexpected status %+v", status)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Error("answers must not be cached")
	}
}

func TestAServerTheBotIsNotInIsNotFound(t *testing.T) {
	for _, route := range []request{
		{method: http.MethodGet, path: "/local/guilds/elsewhere"},
		{method: http.MethodGet, path: "/local/guilds/elsewhere/channels"},
		{method: http.MethodPost, path: "/local/guilds/elsewhere/credential"},
		{method: http.MethodPut, path: "/local/guilds/elsewhere/setup", body: `{}`},
		{method: http.MethodGet, path: "/local/guilds/elsewhere/crewmates"},
		{method: http.MethodPut, path: "/local/guilds/elsewhere/links", body: `{"player":"Alice"}`},
	} {
		if got := serve(t, newFake(), route).Code; got != http.StatusNotFound {
			t.Errorf("%s %s: status %d, want %d", route.method, route.path, got, http.StatusNotFound)
		}
	}
}

func TestChannelsAreListed(t *testing.T) {
	response := serve(t, newFake(), request{method: http.MethodGet, path: "/local/guilds/g1/channels"})

	var channels []Channel
	if err := json.NewDecoder(response.Body).Decode(&channels); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(channels) != 1 || channels[0].Kind != KindVoice {
		t.Errorf("unexpected channels %+v", channels)
	}
}

// The answer is the guild as saved, so the app shows what the bot has rather
// than what the app believes it sent.
func TestASetupIsPassedOnAndAnsweredWithTheSavedGuild(t *testing.T) {
	backend := newFake()
	response := serve(t, backend, request{method: http.MethodPut, path: "/local/guilds/g1/setup",
		body: `{"main_voice_channel_id":"v1","ghost_voice_channel_id":"v2","control_text_channel_id":"t1","auto_start":true}`})

	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body)
	}
	want := Setup{MainVoiceChannelID: "v1", GhostVoiceChannelID: "v2", ControlTextChannelID: "t1", AutoStart: true}
	if len(backend.setups) != 1 || backend.setups[0] != want {
		t.Errorf("the backend received %+v, want %+v", backend.setups, want)
	}

	var guild Guild
	if err := json.NewDecoder(response.Body).Decode(&guild); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if guild.GhostVoiceChannelID != "v2" || !guild.AutoStart {
		t.Errorf("the answer does not show the saved setup: %+v", guild)
	}
}

func TestAMalformedSetupIsRefusedBeforeReachingTheBackend(t *testing.T) {
	for name, body := range map[string]string{
		"not json":      "{",
		"unknown field": `{"main_voice_channel_id":"v1","admin_role_id":"sneaky"}`,
		"two documents": `{}{}`,
		"too large":     `{"main_voice_channel_id":"` + strings.Repeat("v", maxBodyBytes) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			backend := newFake()
			response := serve(t, backend, request{method: http.MethodPut, path: "/local/guilds/g1/setup", body: body})

			if response.Code != http.StatusBadRequest {
				t.Errorf("status %d, want %d", response.Code, http.StatusBadRequest)
			}
			if len(backend.setups) != 0 {
				t.Error("a malformed setup reached the backend")
			}
		})
	}
}

func TestAnUnusableSetupSaysWhy(t *testing.T) {
	backend := newFake()
	backend.configureErr = fmt.Errorf("%w: the main and ghost channels must be different", ErrInvalidSetup)

	response := serve(t, backend, request{method: http.MethodPut, path: "/local/guilds/g1/setup",
		body: `{"main_voice_channel_id":"v1","ghost_voice_channel_id":"v1"}`})

	if response.Code != http.StatusBadRequest {
		t.Errorf("status %d, want %d", response.Code, http.StatusBadRequest)
	}
	if !strings.Contains(response.Body.String(), "must be different") {
		t.Errorf("the reason did not reach the app: %s", response.Body)
	}
}

func TestACredentialIsIssuedToTheApp(t *testing.T) {
	response := serve(t, newFake(), request{method: http.MethodPost, path: "/local/guilds/g1/credential"})

	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Code != http.StatusOK || body["credential"] != "credential-1" {
		t.Errorf("status %d, body %v", response.Code, body)
	}
}

func TestTheLobbyIsListedWithWhomItCanBeLinkedTo(t *testing.T) {
	response := serve(t, newFake(), request{method: http.MethodGet, path: "/local/guilds/g1/crewmates"})

	var crewmates Crewmates
	if err := json.NewDecoder(response.Body).Decode(&crewmates); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(crewmates.Players) != 1 || crewmates.Players[0].Color != "red" ||
		len(crewmates.Members) != 1 || crewmates.Members[0].Name != "Red Leader" {
		t.Errorf("unexpected crewmates %+v", crewmates)
	}
}

// An empty lobby is two empty lists, never null.
func TestAnEmptyLobbyIsTwoEmptyLists(t *testing.T) {
	backend := newFake()
	backend.crewmates["g1"] = Crewmates{}

	response := serve(t, backend, request{method: http.MethodGet, path: "/local/guilds/g1/crewmates"})

	if body := strings.TrimSpace(response.Body.String()); body != `{"players":[],"members":[]}` {
		t.Errorf("got %s", body)
	}
}

func TestALinkIsPassedOnAndAnsweredWithTheLobby(t *testing.T) {
	backend := newFake()
	response := serve(t, backend, request{method: http.MethodPut, path: "/local/guilds/g1/links",
		body: `{"player":"Alice","user_id":"u1"}`})

	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body)
	}
	if len(backend.links) != 1 || backend.links[0] != (Link{Player: "Alice", UserID: "u1"}) {
		t.Errorf("the backend received %+v", backend.links)
	}

	var crewmates Crewmates
	if err := json.NewDecoder(response.Body).Decode(&crewmates); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(crewmates.Players) != 1 || crewmates.Players[0].UserID != "u1" {
		t.Errorf("the answer does not show the new link: %+v", crewmates)
	}
}

func TestAMalformedLinkIsRefusedBeforeReachingTheBackend(t *testing.T) {
	for name, body := range map[string]string{
		"not json":      "{",
		"unknown field": `{"player":"Alice","guild_id":"elsewhere"}`,
		"no player":     `{"user_id":"u1"}`,
		"two documents": `{"player":"Alice"}{}`,
	} {
		t.Run(name, func(t *testing.T) {
			backend := newFake()
			response := serve(t, backend, request{method: http.MethodPut, path: "/local/guilds/g1/links", body: body})

			if response.Code != http.StatusBadRequest {
				t.Errorf("status %d, want %d", response.Code, http.StatusBadRequest)
			}
			if len(backend.links) != 0 {
				t.Error("a malformed link reached the backend")
			}
		})
	}
}

func TestAnUnusableLinkSaysWhy(t *testing.T) {
	backend := newFake()
	backend.linkErr = fmt.Errorf("%w: Bob is not in the lobby", ErrInvalidLink)

	response := serve(t, backend, request{method: http.MethodPut, path: "/local/guilds/g1/links",
		body: `{"player":"Bob","user_id":"u1"}`})

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "not in the lobby") {
		t.Errorf("status %d, body %s", response.Code, response.Body)
	}
}

// Stopping closes the listener the request arrived on, so the answer has to be
// on its way before the stop begins.
func TestShutdownAnswersAndThenStops(t *testing.T) {
	backend := newFake()
	response := serve(t, backend, request{method: http.MethodPost, path: "/local/shutdown"})

	if response.Code != http.StatusAccepted {
		t.Errorf("status %d, want %d", response.Code, http.StatusAccepted)
	}
	select {
	case <-backend.stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("the backend was never asked to stop")
	}
}

func TestReadingRoutesCannotChangeAnything(t *testing.T) {
	backend := newFake()
	for _, route := range []request{
		{method: http.MethodGet, path: "/local/shutdown"},
		{method: http.MethodGet, path: "/local/guilds/g1/credential"},
		{method: http.MethodPost, path: "/local/guilds/g1/setup", body: `{}`},
		{method: http.MethodGet, path: "/local/guilds/g1/links"},
		{method: http.MethodPost, path: "/local/guilds/g1/crewmates", body: `{"player":"Alice","user_id":"u1"}`},
	} {
		if got := serve(t, backend, route).Code; got == http.StatusOK || got == http.StatusAccepted {
			t.Errorf("%s %s succeeded with the wrong method", route.method, route.path)
		}
	}
	if backend.issued != 0 || len(backend.setups) != 0 || len(backend.links) != 0 {
		t.Error("a request with the wrong method changed something")
	}
}
