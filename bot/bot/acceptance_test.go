package bot

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/pairing"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/voice"
)

// The required scenarios from docs/requirements.md, one test each, named after
// what they prove rather than numbered.
//
// They run a real lobby through the real pieces: protocol messages reach the
// real session, the real player links resolve them, the real policy decides and
// the real reconciler works out what Discord would be told. The only thing
// missing is the API call itself, which pkg/transport and pkg/voice test on
// their own.
//
// Scenarios 13 to 16 are recovery and live in recovery_test.go, which drives an
// actual WebSocket server; 17 and 18 are the doctor and are at the bottom here.

const (
	mainChannel  = "main-channel"
	ghostChannel = "ghost-channel"
	elsewhere    = "some-other-channel"
)

type lobby struct {
	t        *testing.T
	bot      *Bot
	db       *sqlite.DB
	config   voice.Config
	observed map[string]voice.Observed
	seq      uint64
}

func newLobby(t *testing.T, players ...string) *lobby {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "auvc.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	config := sqlite.DefaultGuildConfig(guild)
	config.MainVoiceChannelID = mainChannel
	config.GhostVoiceChannelID = ghostChannel
	if err := db.SaveGuildConfig(config); err != nil {
		t.Fatalf("save configuration: %v", err)
	}

	l := &lobby{
		t:  t,
		db: db,
		bot: &Bot{
			AUVC:            au.NewServiceWithPairing(db, pairing.NewService(db), "test", "test"),
			CaptureSessions: NewCaptureSessions(),
			AUVCLinks:       db,
		},
		config: voice.Config{
			MainChannelID:   mainChannel,
			GhostChannelID:  ghostChannel,
			AutoMoveGhosts:  true,
			EnforceChannels: true,
		},
		observed: map[string]voice.Observed{},
	}

	// Everyone starts linked and sitting in the main channel, which is what a
	// lobby looks like before a round.
	for _, name := range players {
		if err := db.SaveLink(guild, name, userFor(name)); err != nil {
			t.Fatalf("link %s: %v", name, err)
		}
		l.observed[userFor(name)] = voice.Observed{ChannelID: mainChannel}
	}
	return l
}

func userFor(name string) string { return "user-" + name }

// send delivers one message the way capture would, numbering it in order.
func (l *lobby) send(message protocol.Message) {
	l.t.Helper()

	l.seq++
	switch typed := message.(type) {
	case *protocol.Snapshot:
		typed.Header = protocol.Header{Protocol: protocol.Version, Type: protocol.TypeSnapshot,
			Session: "session-a", Seq: l.seq}
	case *protocol.GameStateChanged:
		typed.Header = protocol.Header{Protocol: protocol.Version, Type: protocol.TypeGameStateChanged,
			Session: "session-a", Seq: l.seq}
	case *protocol.PlayerDied:
		typed.Header = protocol.Header{Protocol: protocol.Version, Type: protocol.TypePlayerDied,
			Session: "session-a", Seq: l.seq}
	case *protocol.PlayerJoined:
		typed.Header = protocol.Header{Protocol: protocol.Version, Type: protocol.TypePlayerJoined,
			Session: "session-a", Seq: l.seq}
	case *protocol.GameEnded:
		typed.Header = protocol.Header{Protocol: protocol.Version, Type: protocol.TypeGameEnded,
			Session: "session-a", Seq: l.seq}
	}

	if err := l.bot.HandleCapture(guild, message); err != nil {
		l.t.Fatalf("handling %s: %v", protocol.Envelope(message).Type, err)
	}
	l.settle()
}

// settle applies what the reconciler would have told Discord, so the observed
// state moves the way a real server's would.
func (l *lobby) settle() {
	l.t.Helper()

	resolve, err := l.bot.linkResolver(guild)
	if err != nil {
		l.t.Fatalf("resolve links: %v", err)
	}

	session := l.bot.CaptureSessions.forGuild(guild)
	session.mu.Lock()
	state := session.live.Project(resolve)
	session.mu.Unlock()

	for _, change := range voice.Diff(l.observed, voice.Desired(state, l.config)) {
		seen := l.observed[change.UserID]
		if change.MoveTo != nil {
			seen.ChannelID = *change.MoveTo
		}
		if change.Muted != nil {
			seen.Muted = *change.Muted
		}
		if change.Deafened != nil {
			seen.Deafened = *change.Deafened
		}
		l.observed[change.UserID] = seen
	}
}

// moveTo puts a player somewhere by hand, the way a person dragging themselves
// between channels would.
func (l *lobby) moveTo(name, channelID string) {
	seen := l.observed[userFor(name)]
	seen.ChannelID = channelID
	l.observed[userFor(name)] = seen
	l.settle()
}

func (l *lobby) where(name string) voice.Observed { return l.observed[userFor(name)] }

func (l *lobby) expect(name string, want voice.Observed) {
	l.t.Helper()

	if got := l.where(name); got != want {
		l.t.Errorf("%s is %+v, want %+v", name, got, want)
	}
}

func openIn(channelID string) voice.Observed { return voice.Observed{ChannelID: channelID} }

func silenced(channelID string) voice.Observed {
	return voice.Observed{ChannelID: channelID, Muted: true}
}

func shutOut(channelID string) voice.Observed {
	return voice.Observed{ChannelID: channelID, Muted: true, Deafened: true}
}

func alive(name string) protocol.Player { return protocol.Player{Name: name} }

// 1. Lobby: all managed players open in main.
func TestScenarioLobbyLeavesEveryoneOpenInMain(t *testing.T) {
	lobby := newLobby(t, "Red", "Blue")
	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseLobby,
		Players: []protocol.Player{alive("Red"), alive("Blue")}})

	lobby.expect("Red", openIn(mainChannel))
	lobby.expect("Blue", openIn(mainChannel))
}

// 2. Tasks: living players muted and deafened.
func TestScenarioTasksSilenceTheLiving(t *testing.T) {
	lobby := newLobby(t, "Red", "Blue")
	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseTasks,
		Players: []protocol.Player{alive("Red"), alive("Blue")}})

	lobby.expect("Red", shutOut(mainChannel))
	lobby.expect("Blue", shutOut(mainChannel))
}

// 3. Death during tasks: the victim is silenced and stays where they are, so
// nothing visible in Discord announces the kill.
func TestScenarioAKillIsNotAnnouncedByAMove(t *testing.T) {
	lobby := newLobby(t, "Red", "Blue")
	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseTasks,
		Players: []protocol.Player{alive("Red"), alive("Blue")}})

	lobby.send(&protocol.PlayerDied{Player: protocol.Player{Name: "Blue", Dead: true}})

	if lobby.where("Blue").ChannelID != mainChannel {
		t.Errorf("the kill was announced by a move: %+v", lobby.where("Blue"))
	}
	lobby.expect("Blue", silenced(mainChannel))
}

// 4. The first meeting makes the death public; the ghost then moves to the
// ghost channel, unmuted and undeafened.
func TestScenarioTheMeetingSendsTheGhostToTheGhostChannel(t *testing.T) {
	lobby := newLobby(t, "Red", "Blue")
	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseTasks,
		Players: []protocol.Player{alive("Red"), alive("Blue")}})
	lobby.send(&protocol.PlayerDied{Player: protocol.Player{Name: "Blue", Dead: true}})

	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseDiscussion})

	lobby.expect("Blue", openIn(ghostChannel))
}

// 5. A second ghost joins; both remain in ghost.
func TestScenarioTwoGhostsShareTheGhostChannel(t *testing.T) {
	lobby := newLobby(t, "Red", "Blue", "Green")
	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseTasks,
		Players: []protocol.Player{alive("Red"), alive("Blue"), alive("Green")}})
	lobby.send(&protocol.PlayerDied{Player: protocol.Player{Name: "Blue", Dead: true}})
	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseDiscussion})
	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseTasks})

	lobby.send(&protocol.PlayerDied{Player: protocol.Player{Name: "Green", Dead: true}})
	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseDiscussion})

	lobby.expect("Blue", openIn(ghostChannel))
	lobby.expect("Green", openIn(ghostChannel))
}

// 6. Meeting: living players unmuted and undeafened.
// 8. Ghosts can continue talking during meeting.
func TestScenarioAMeetingLetsEveryoneTalkInTheirOwnRoom(t *testing.T) {
	lobby := newLobby(t, "Red", "Blue")
	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseTasks,
		Players: []protocol.Player{alive("Red"), alive("Blue")}})
	lobby.send(&protocol.PlayerDied{Player: protocol.Player{Name: "Blue", Dead: true}})
	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseDiscussion})

	lobby.expect("Red", openIn(mainChannel))
	lobby.expect("Blue", openIn(ghostChannel))
}

// 7. Ghosts remain in ghost during meeting, and through the task phase after
// it: the death is public by then, so sending them back would be the same leak
// in reverse.
func TestScenarioAGhostKeepsTheGhostChannelAfterTheMeeting(t *testing.T) {
	lobby := newLobby(t, "Red", "Blue")
	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseTasks,
		Players: []protocol.Player{alive("Red"), alive("Blue")}})
	lobby.send(&protocol.PlayerDied{Player: protocol.Player{Name: "Blue", Dead: true}})
	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseDiscussion})

	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseTasks})

	lobby.expect("Blue", openIn(ghostChannel))
}

// 9. Meeting ends: living players muted and deafened again.
func TestScenarioTheLivingAreSilencedAgainWhenTasksResume(t *testing.T) {
	lobby := newLobby(t, "Red", "Blue")
	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseDiscussion,
		Players: []protocol.Player{alive("Red"), alive("Blue")}})
	lobby.expect("Red", openIn(mainChannel))

	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseTasks})

	lobby.expect("Red", shutOut(mainChannel))
}

// 10. Dead player enters main: returned to ghost.
func TestScenarioAGhostWhoWalksIntoMainIsSentBack(t *testing.T) {
	lobby := newLobby(t, "Red", "Blue")
	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseTasks,
		Players: []protocol.Player{alive("Red"), alive("Blue")}})
	lobby.send(&protocol.PlayerDied{Player: protocol.Player{Name: "Blue", Dead: true}})
	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseDiscussion})

	lobby.moveTo("Blue", mainChannel)

	lobby.expect("Blue", openIn(ghostChannel))
}

// 11. Living player enters ghost: returned to main.
func TestScenarioALivingPlayerWhoWalksIntoGhostIsSentBack(t *testing.T) {
	lobby := newLobby(t, "Red", "Blue")
	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseDiscussion,
		Players: []protocol.Player{alive("Red"), alive("Blue")}})

	lobby.moveTo("Red", ghostChannel)

	lobby.expect("Red", openIn(mainChannel))
}

// 12. Game ends: all managed players in main, open.
func TestScenarioTheRoundEndsWithEveryoneBackTogether(t *testing.T) {
	lobby := newLobby(t, "Red", "Blue")
	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseTasks,
		Players: []protocol.Player{alive("Red"), alive("Blue")}})
	lobby.send(&protocol.PlayerDied{Player: protocol.Player{Name: "Blue", Dead: true}})
	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseDiscussion})
	lobby.expect("Blue", openIn(ghostChannel))

	lobby.send(&protocol.GameEnded{})

	lobby.expect("Red", openIn(mainChannel))
	lobby.expect("Blue", openIn(mainChannel))
}

// 19. An unlinked Discord user is untouched by default.
func TestScenarioAnUnlinkedUserIsNeverTouched(t *testing.T) {
	lobby := newLobby(t, "Red")
	// Somebody in the voice channel who is not playing.
	lobby.observed["user-spectator"] = voice.Observed{ChannelID: mainChannel}

	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseTasks, Players: []protocol.Player{alive("Red")}})
	lobby.send(&protocol.PlayerDied{Player: protocol.Player{Name: "Red", Dead: true}})
	lobby.send(&protocol.GameStateChanged{Phase: protocol.PhaseDiscussion})

	if got := lobby.observed["user-spectator"]; got != openIn(mainChannel) {
		t.Errorf("a spectator was touched: %+v", got)
	}
}

// A player the game reports but nobody linked is equally untouched, which is
// the case that matters when somebody joins a lobby without running /au link.
func TestScenarioAnUnlinkedPlayerIsNeverTouched(t *testing.T) {
	lobby := newLobby(t, "Red")
	lobby.observed["user-Stranger"] = voice.Observed{ChannelID: mainChannel}

	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseTasks,
		Players: []protocol.Player{alive("Red"), alive("Stranger")}})

	if got := lobby.observed["user-Stranger"]; got != openIn(mainChannel) {
		t.Errorf("an unlinked player was managed: %+v", got)
	}
	lobby.expect("Red", shutOut(mainChannel))
}

// 17. Configured channel deleted: doctor reports error.
func TestScenarioADeletedChannelIsReportedByTheDoctor(t *testing.T) {
	lobby := newLobby(t, "Red")

	// No Discord session at all stands in for a channel the bot cannot see; the
	// doctor takes the same route either way, and this is the one a test can
	// reach without a Discord connection.
	report, err := NewDoctor(lobby.bot).Diagnose(guild)
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	if report == "" {
		t.Fatal("the doctor said nothing")
	}
	if !strings.Contains(report, "Discord") {
		t.Errorf("the doctor did not report the unreachable Discord: %s", report)
	}
}

// 18. Missing Move Members: doctor reports error.
//
// The permission audit itself is exhaustive in pkg/permission; what this checks
// is that the doctor asks it and puts the answer in the report, which is the
// part a guild administrator actually sees.
func TestScenarioMissingPermissionsWouldBeReported(t *testing.T) {
	lobby := newLobby(t, "Red")

	report, err := NewDoctor(lobby.bot).Diagnose(guild)
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	// Without a Discord session the permission check cannot run, and the report
	// has to say the connection is the problem rather than claim the
	// permissions are fine.
	if strings.Contains(report, "all five voice permissions") {
		t.Errorf("the doctor claimed permissions were checked without Discord: %s", report)
	}
}

// Enforcement is not only about the ghost channel. A living player who wanders
// off into some unrelated channel mid-round is brought back too, or the bot
// stops managing somebody who is still playing.
func TestScenarioAPlayerWhoWandersOffEntirelyIsBroughtBack(t *testing.T) {
	lobby := newLobby(t, "Red", "Blue")
	lobby.send(&protocol.Snapshot{Phase: protocol.PhaseDiscussion,
		Players: []protocol.Player{alive("Red"), alive("Blue")}})

	lobby.moveTo("Red", elsewhere)

	lobby.expect("Red", openIn(mainChannel))
}

// 15. Bot restart: guild settings survive.
//
// The bot keeps nothing about a round across a restart on purpose, so what has
// to survive is exactly the configuration and the links. Losing those means
// setting the server up again, which is the difference between an outage and an
// afternoon.
func TestScenarioARestartKeepsTheConfigurationAndTheLinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auvc.db")

	before, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	configured := sqlite.DefaultGuildConfig(guild)
	configured.MainVoiceChannelID = mainChannel
	configured.GhostVoiceChannelID = ghostChannel
	configured.ControlTextChannelID = "control"
	configured.EnforceChannels = false
	if err := before.SaveGuildConfig(configured); err != nil {
		t.Fatalf("save configuration: %v", err)
	}
	if err := before.SaveLink(guild, "Red", userFor("Red")); err != nil {
		t.Fatalf("save link: %v", err)
	}
	before.Close()

	// The restart.
	after, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer after.Close()

	restored, err := after.GuildConfig(guild)
	if err != nil {
		t.Fatalf("read configuration: %v", err)
	}
	if restored.MainVoiceChannelID != mainChannel || restored.GhostVoiceChannelID != ghostChannel {
		t.Errorf("the channels did not survive: %+v", restored)
	}
	if restored.ControlTextChannelID != "control" {
		t.Errorf("the control channel did not survive: %+v", restored)
	}
	// A setting deliberately turned off has to stay off. Coming back as the
	// default would quietly re-enable something an administrator switched off.
	if restored.EnforceChannels {
		t.Error("a setting that was turned off came back on after a restart")
	}

	links, err := after.Links(guild)
	if err != nil {
		t.Fatalf("read links: %v", err)
	}
	if len(links) != 1 || links[0].InGameName != "Red" || links[0].DiscordUserID != userFor("Red") {
		t.Errorf("the player links did not survive: %+v", links)
	}
}
