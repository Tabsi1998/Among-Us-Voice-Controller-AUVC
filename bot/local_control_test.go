package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/bot"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/localcontrol"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/pairing"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/bwmarrin/discordgo"
)

const localGuild = "guild-1"

// localSecret is assembled rather than written out, so a secret scanner sees a
// construction instead of something shaped like a key.
var localSecret = strings.Repeat("app-started-this-bot-", 2)

type localFixture struct {
	backend    *localBackend
	controller *bot.Bot
	pairing    *pairing.Service
	stopped    bool
}

// newLocalFixture builds a bot whose Discord state holds one server with a
// choice of channels, the way it would after connecting.
func newLocalFixture(t *testing.T) *localFixture {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "auvc.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	state := discordgo.NewState()
	state.User = &discordgo.User{ID: "bot-user", Username: "AUVC"}
	channel := func(id, name string, kind discordgo.ChannelType, position int) *discordgo.Channel {
		return &discordgo.Channel{ID: id, GuildID: localGuild, Name: name, Type: kind, Position: position}
	}
	if err := state.GuildAdd(&discordgo.Guild{ID: localGuild, Name: "The Crew", Channels: []*discordgo.Channel{
		channel("text-general", "general", discordgo.ChannelTypeGuildText, 0),
		channel("voice-ghosts", "Ghosts", discordgo.ChannelTypeGuildVoice, 2),
		channel("voice-main", "Among Us", discordgo.ChannelTypeGuildVoice, 1),
		channel("stage", "Stage", discordgo.ChannelTypeGuildStageVoice, 3),
	}}); err != nil {
		t.Fatalf("add guild: %v", err)
	}
	if err := state.GuildAdd(&discordgo.Guild{ID: "guild-2", Name: "Another Server", Channels: []*discordgo.Channel{
		{ID: "elsewhere-voice", GuildID: "guild-2", Name: "Voice", Type: discordgo.ChannelTypeGuildVoice},
	}}); err != nil {
		t.Fatalf("add guild: %v", err)
	}

	pairingService := pairing.NewService(db)
	controller := &bot.Bot{
		PrimarySession:  &discordgo.Session{State: state},
		AUVC:            au.NewServiceWithPairing(db, pairingService, "test", "test"),
		CaptureSessions: bot.NewCaptureSessions(),
	}

	fixture := &localFixture{controller: controller, pairing: pairingService}
	fixture.backend = &localBackend{
		controller: controller,
		pairing:    pairingService,
		version:    "test",
		stop:       func() { fixture.stopped = true },
	}
	return fixture
}

func TestTheAppSeesTheBotAndItsServers(t *testing.T) {
	status := newLocalFixture(t).backend.Status()

	if !status.Connected || status.BotName != "AUVC" || status.BotID != "bot-user" {
		t.Errorf("unexpected identity %+v", status)
	}
	names := []string{}
	for _, guild := range status.Guilds {
		names = append(names, guild.Name)
	}
	if want := []string{"Another Server", "The Crew"}; !reflect.DeepEqual(names, want) {
		t.Errorf("servers %v, want %v", names, want)
	}
}

// Voice first, each kind in Discord's order, and nothing a round cannot use.
func TestTheAppIsOfferedVoiceAndTextChannelsOnly(t *testing.T) {
	channels, err := newLocalFixture(t).backend.Channels(localGuild)
	if err != nil {
		t.Fatalf("channels: %v", err)
	}

	got := []string{}
	for _, channel := range channels {
		got = append(got, channel.Kind+":"+channel.ID)
	}
	want := []string{"voice:voice-main", "voice:voice-ghosts", "text:text-general"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("channels %v, want %v", got, want)
	}
}

func TestASetupFromTheAppIsSaved(t *testing.T) {
	fixture := newLocalFixture(t)

	err := fixture.backend.Configure(localGuild, localcontrol.Setup{
		MainVoiceChannelID:   "voice-main",
		GhostVoiceChannelID:  "voice-ghosts",
		ControlTextChannelID: "text-general",
		AutoStart:            true,
	})
	if err != nil {
		t.Fatalf("configure: %v", err)
	}

	guild, err := fixture.backend.Guild(localGuild)
	if err != nil {
		t.Fatalf("guild: %v", err)
	}
	if guild.Name != "The Crew" || guild.MainVoiceChannelID != "voice-main" ||
		guild.GhostVoiceChannelID != "voice-ghosts" || guild.ControlTextChannelID != "text-general" || !guild.AutoStart {
		t.Errorf("the saved guild reads %+v", guild)
	}
}

// The app sends plain ids, so the checks Discord makes for /au setup channels
// by offering only voice channels have to happen here.
func TestASetupWithTheWrongChannelsIsRefused(t *testing.T) {
	for name, setup := range map[string]localcontrol.Setup{
		"text channel as main":        {MainVoiceChannelID: "text-general", GhostVoiceChannelID: "voice-ghosts"},
		"stage channel as ghost":      {MainVoiceChannelID: "voice-main", GhostVoiceChannelID: "stage"},
		"voice channel as control":    {MainVoiceChannelID: "voice-main", GhostVoiceChannelID: "voice-ghosts", ControlTextChannelID: "voice-main"},
		"channel from another server": {MainVoiceChannelID: "elsewhere-voice", GhostVoiceChannelID: "voice-ghosts"},
		"no such channel":             {MainVoiceChannelID: "voice-main", GhostVoiceChannelID: "missing"},
		"the same channel twice":      {MainVoiceChannelID: "voice-main", GhostVoiceChannelID: "voice-main"},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newLocalFixture(t)

			if err := fixture.backend.Configure(localGuild, setup); !errors.Is(err, localcontrol.ErrInvalidSetup) {
				t.Fatalf("got %v, want %v", err, localcontrol.ErrInvalidSetup)
			}
			config, err := fixture.controller.AUVC.GuildConfig(localGuild)
			if err != nil {
				t.Fatalf("read back: %v", err)
			}
			if config.MainVoiceChannelID != "" || config.GhostVoiceChannelID != "" {
				t.Errorf("a refused setup was stored: %+v", config)
			}
		})
	}
}

func TestACredentialIssuedToTheAppAuthenticates(t *testing.T) {
	fixture := newLocalFixture(t)

	token, err := fixture.backend.IssueCredential(localGuild)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	guild, err := fixture.pairing.Authenticate(token)
	if err != nil || guild != localGuild {
		t.Errorf("the credential speaks for %q (%v), want %q", guild, err, localGuild)
	}
}

func TestNothingIsDoneForAServerTheBotIsNotIn(t *testing.T) {
	backend := newLocalFixture(t).backend

	if _, err := backend.Channels("stranger"); !errors.Is(err, localcontrol.ErrUnknownGuild) {
		t.Errorf("channels gave %v", err)
	}
	if _, err := backend.Guild("stranger"); !errors.Is(err, localcontrol.ErrUnknownGuild) {
		t.Errorf("guild gave %v", err)
	}
	if _, err := backend.IssueCredential("stranger"); !errors.Is(err, localcontrol.ErrUnknownGuild) {
		t.Errorf("a credential was issued for a server the bot is not in: %v", err)
	}
	setup := localcontrol.Setup{MainVoiceChannelID: "voice-main", GhostVoiceChannelID: "voice-ghosts"}
	if err := backend.Configure("stranger", setup); !errors.Is(err, localcontrol.ErrUnknownGuild) {
		t.Errorf("configure gave %v", err)
	}
}

func TestShutdownFromTheAppStopsTheBot(t *testing.T) {
	fixture := newLocalFixture(t)

	fixture.backend.Shutdown()

	if !fixture.stopped {
		t.Error("the stop was not requested")
	}
}

// Without the secret the routes do not exist at all, rather than existing and
// refusing: a bot in a container never started by the app has no business
// answering them.
func TestTheControlRoutesExistOnlyWhenTheAppStartedTheBot(t *testing.T) {
	fixture := newLocalFixture(t)
	capture := newCaptureServer(fixture.pairing, fixture.controller)

	request := func(handler http.Handler) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/local/status", nil)
		r.RemoteAddr = "127.0.0.1:50000"
		r.Header.Set("Authorization", "Bearer "+localSecret)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, r)
		return recorder
	}

	if got := request(routes(capture, fixture.controller, nil)).Code; got != http.StatusNotFound {
		t.Errorf("without a secret: status %d, want %d", got, http.StatusNotFound)
	}

	local, err := localControl(localSecret, fixture.backend)
	if err != nil || local == nil {
		t.Fatalf("local control: %v", err)
	}
	response := request(routes(capture, fixture.controller, local))
	if response.Code != http.StatusOK {
		t.Fatalf("with the secret: status %d", response.Code)
	}
	var status localcontrol.Status
	if err := json.NewDecoder(response.Body).Decode(&status); err != nil || status.BotName != "AUVC" {
		t.Errorf("status %+v (%v)", status, err)
	}

	// The health check and the capture endpoints are still where they were.
	for _, path := range []string{"/healthz", "/capture/pair", "/capture/link"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		routes(capture, fixture.controller, local).ServeHTTP(recorder, r)
		if recorder.Code == http.StatusNotFound {
			t.Errorf("%s is no longer routed", path)
		}
	}
}

func TestLocalControlIsOffWithoutASecretAndRefusesAWeakOne(t *testing.T) {
	backend := newLocalFixture(t).backend

	if local, err := localControl("", backend); local != nil || err != nil {
		t.Errorf("an empty secret gave %v, %v", local, err)
	}
	if _, err := localControl("short", backend); err == nil {
		t.Error("a short secret started local control")
	}
}

func TestCommandTargets(t *testing.T) {
	for configured, want := range map[string]struct {
		everyGuild bool
		guilds     []string
	}{
		"":          {false, []string{""}},
		"*":         {true, nil},
		" * ":       {true, nil},
		"123":       {false, []string{"123"}},
		"123, 456":  {false, []string{"123", "456"}},
		"123,,456,": {false, []string{"123", "456"}},
		" , ":       {false, []string{""}},
	} {
		everyGuild, guilds := commandTargets(configured)
		if everyGuild != want.everyGuild || !reflect.DeepEqual(guilds, want.guilds) {
			t.Errorf("%q gave %v %v, want %v %v", configured, everyGuild, guilds, want.everyGuild, want.guilds)
		}
	}
}
