package bot

import (
	"fmt"
	"sync"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/permission"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/session"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/voice"
	"github.com/bwmarrin/discordgo"
)

// phaseFromProtocol maps the protocol's phase names onto the bot's own.
//
// The two are separate types on purpose: renaming one must not silently change
// the other, and this function is where a new phase has to be considered.
func phaseFromProtocol(phase protocol.Phase) (game.Phase, bool) {
	switch phase {
	case protocol.PhaseLobby:
		return game.LOBBY, true
	case protocol.PhaseTasks:
		return game.TASKS, true
	case protocol.PhaseDiscussion:
		return game.DISCUSS, true
	case protocol.PhaseMenu:
		return game.MENU, true
	case protocol.PhaseEnded:
		return game.GAMEOVER, true
	default:
		return game.UNINITIALIZED, false
	}
}

// playerFromProtocol converts one player as capture reported them.
func playerFromProtocol(player protocol.Player) session.GamePlayer {
	return session.GamePlayer{
		Name:  player.Name,
		Color: player.Color,
		// The protocol reports death; the session tracks life. Inverting here
		// keeps the double negative out of the policy, which is read far more
		// often than this line.
		Alive:        !player.Dead,
		Disconnected: player.Disconnected,
	}
}

// Mode is whether the bot is acting on a guild's session.
type Mode int

const (
	// Stopped means the session is tracked but nobody is moved or muted. It is
	// where a guild starts unless auto_start is on.
	Stopped Mode = iota
	// Running means the voice policy is applied on every change.
	Running
	// Paused means the session keeps being tracked and Discord is left exactly
	// as it is. It is what an administrator reaches for mid-round, and it is
	// deliberately different from Stopped: stopping releases everyone, pausing
	// leaves them where they are.
	Paused
)

func (m Mode) String() string {
	switch m {
	case Running:
		return "running"
	case Paused:
		return "paused"
	default:
		return "stopped"
	}
}

// guildSession is one guild's session and whether the bot is acting on it.
type guildSession struct {
	mu   sync.Mutex
	live *session.Live
	mode Mode

	// lastSeen is when capture last said anything at all. Every accepted
	// message counts, not only heartbeats: a session sending game events is
	// evidently alive.
	lastSeen time.Time

	// captureSession is the protocol session the guild is currently
	// following. The protocol receiver lives per connection, so it cannot
	// know that another connection replaced it: that is decided here.
	captureSession string

	// stalled marks a session the capture timeout interrupted, and
	// stalledFrom is the mode it was in at the time. Capture coming back
	// restores what the administrator chose, rather than leaving a session
	// paused by a fault nobody asked for.
	stalled     bool
	stalledFrom Mode
}

// CaptureSessions holds the live session of every guild that has a capture
// connected.
//
// Work is serialized per guild. One capture produces one ordered stream, but a
// reconnect can overlap the tail of the previous connection, and two streams
// writing the same session would interleave into a state neither of them sent.
type CaptureSessions struct {
	mu       sync.Mutex
	sessions map[string]*guildSession
}

// NewCaptureSessions returns an empty set.
func NewCaptureSessions() *CaptureSessions {
	return &CaptureSessions{sessions: map[string]*guildSession{}}
}

// forGuild returns a guild's session, creating it on first use.
func (c *CaptureSessions) forGuild(guildID string) *guildSession {
	c.mu.Lock()
	defer c.mu.Unlock()

	existing, ok := c.sessions[guildID]
	if !ok {
		existing = &guildSession{live: session.NewLive(), mode: Stopped}
		c.sessions[guildID] = existing
	}
	return existing
}

// SetMode records whether the bot should act on a guild and returns the mode it
// was in before.
func (c *CaptureSessions) SetMode(guildID string, mode Mode) Mode {
	guild := c.forGuild(guildID)

	guild.mu.Lock()
	defer guild.mu.Unlock()

	previous := guild.mode
	guild.mode = mode
	return previous
}

// Mode reports whether the bot is acting on a guild.
func (c *CaptureSessions) Mode(guildID string) Mode {
	guild := c.forGuild(guildID)

	guild.mu.Lock()
	defer guild.mu.Unlock()

	return guild.mode
}

// Snapshot returns a guild's session for reading. It is what /au session status
// and /au doctor report.
func (c *CaptureSessions) Snapshot(guildID string) (Mode, game.Phase, []session.GamePlayer) {
	guild := c.forGuild(guildID)

	guild.mu.Lock()
	defer guild.mu.Unlock()

	return guild.mode, guild.live.Phase(), guild.live.Players()
}

// HandleCapture applies one accepted protocol message and brings Discord voice
// into line with the result.
//
// Only messages that can change the voice picture trigger a reconciliation. A
// heartbeat says nothing about the round, and reconciling on one would mean
// talking to Discord every few seconds for no reason.
func (bot *Bot) HandleCapture(guildID string, message protocol.Message) error {
	if guildID == "" {
		return fmt.Errorf("capture message carries no guild")
	}

	// Any accepted message proves capture is alive, not only a heartbeat: a
	// session sending game events is evidently running.
	bot.CaptureSessions.seen(guildID, time.Now())
	bot.captureReturned(guildID)

	guild := bot.CaptureSessions.forGuild(guildID)

	guild.mu.Lock()
	if !guild.follows(message) {
		guild.mu.Unlock()
		return nil
	}
	changed, err := applyCaptureMessage(guild.live, message)
	mode := guild.mode
	guild.mu.Unlock()

	if err != nil || !changed {
		return err
	}

	// auto_start exists so a guild that always wants AUVC does not have to run
	// a command before every match. A guild that left it off has said it wants
	// to decide, so a snapshot must not quietly take over.
	if mode == Stopped {
		if !bot.autoStart(guildID) {
			return nil
		}
		bot.CaptureSessions.SetMode(guildID, Running)
	} else if mode == Paused {
		// The session keeps being tracked so that resuming acts on the round as
		// it is now, not as it was when the pause started.
		return nil
	}

	return bot.reconcileCaptureSession(guildID)
}

// applyCaptureMessage updates the session and reports whether the voice picture
// could have changed.
func applyCaptureMessage(live *session.Live, message protocol.Message) (bool, error) {
	switch typed := message.(type) {
	case *protocol.Snapshot:
		phase, ok := phaseFromProtocol(typed.Phase)
		if !ok {
			return false, fmt.Errorf("capture reported unknown phase %q", typed.Phase)
		}

		players := make([]session.GamePlayer, 0, len(typed.Players))
		for _, player := range typed.Players {
			players = append(players, playerFromProtocol(player))
		}
		live.Reset(phase, players)
		return true, nil

	case *protocol.GameStateChanged:
		phase, ok := phaseFromProtocol(typed.Phase)
		if !ok {
			return false, fmt.Errorf("capture reported unknown phase %q", typed.Phase)
		}
		return live.SetPhase(phase), nil

	case *protocol.PlayerJoined:
		live.Upsert(playerFromProtocol(typed.Player))
		return true, nil

	case *protocol.PlayerChanged:
		live.Upsert(playerFromProtocol(typed.Player))
		return true, nil

	case *protocol.PlayerDied:
		live.Upsert(playerFromProtocol(typed.Player))
		return true, nil

	case *protocol.PlayerLeft:
		live.Remove(typed.Player.Name)
		return true, nil

	case *protocol.GameEnded:
		live.EndRound()
		return true, nil

	case *protocol.Heartbeat:
		// Liveness, not game state. Phase 15 uses it for the capture timeout.
		return false, nil

	default:
		return false, nil
	}
}

// reconcileCaptureSession projects the session, asks the voice policy what it
// should look like, and applies the difference.
//
// This is the same policy and reconciler the legacy path uses, reached without
// Redis: the session lives in this process and the links come from SQLite.
func (bot *Bot) reconcileCaptureSession(guildID string) error {
	config, ready := bot.voicePolicyConfig(guildID)
	if !ready {
		// A guild that has not run /au setup channels has nowhere to move
		// anyone. Saying nothing here is right: /au capture status and
		// /au doctor are where an unconfigured guild is reported.
		return nil
	}

	resolve, err := bot.linkResolver(guildID)
	if err != nil {
		return err
	}

	guild := bot.CaptureSessions.forGuild(guildID)
	guild.mu.Lock()
	state := guild.live.Project(resolve)
	guild.mu.Unlock()

	discordGuild, err := bot.PrimarySession.State.Guild(guildID)
	if err != nil || discordGuild == nil {
		return fmt.Errorf("discord guild %s is unavailable: %w", guildID, err)
	}

	if err := bot.Reconciler.Reconcile(guildID, observeVoiceStates(discordGuild), voice.Desired(state, config)); err != nil {
		if summary := permission.Summary(permission.Audit(
			bot.voiceChannelPermissions(config.MainChannelID, config.GhostChannelID)...,
		)); summary != "" {
			return fmt.Errorf("%w\n%s", err, summary)
		}
		return err
	}
	return nil
}

// linkResolver builds the function that turns an in-game name into a Discord
// user.
//
// The links are read once per reconciliation rather than per player: a lobby is
// ten players and a round produces a reconciliation per event, so a query each
// would be ten times the work for an answer that cannot change in between.
func (bot *Bot) linkResolver(guildID string) (func(string) (string, bool, bool), error) {
	if bot.AUVCLinks == nil {
		return nil, fmt.Errorf("player links are unavailable; the capture path cannot resolve anyone")
	}

	links, err := bot.AUVCLinks.Links(guildID)
	if err != nil {
		return nil, fmt.Errorf("read player links for guild %s: %w", guildID, err)
	}

	byName := make(map[string]string, len(links))
	for _, link := range links {
		byName[link.InGameName] = link.DiscordUserID
	}

	return func(inGameName string) (string, bool, bool) {
		userID, ok := byName[inGameName]
		if !ok {
			return "", false, false
		}
		return userID, bot.isBotAccount(guildID, userID), true
	}, nil
}

// isBotAccount reports whether a Discord user is a bot.
//
// A music bot linked to a player name by accident must never be muted or
// moved. When Discord cannot tell us, the answer is no: refusing to manage
// every player the cache has not seen would be worse than managing one bot.
func (bot *Bot) isBotAccount(guildID, userID string) bool {
	if bot.PrimarySession == nil || bot.PrimarySession.State == nil {
		return false
	}

	member, err := bot.PrimarySession.State.Member(guildID, userID)
	if err != nil || member == nil || member.User == nil {
		return false
	}
	return member.User.Bot
}

// LinkStore is the part of the database the capture path reads.
type LinkStore interface {
	Links(guildID string) ([]sqlite.PlayerLink, error)
}

// captureGuildMember exists so the compiler checks that the Discord state
// really offers what isBotAccount needs.
var _ = func(s *discordgo.Session, guildID, userID string) (*discordgo.Member, error) {
	return s.State.Member(guildID, userID)
}

// autoStart reports whether the guild wants AUVC to take over on its own.
//
// A configuration that cannot be read answers no. Taking over a guild's voice
// channels because a database was briefly unavailable is the wrong way to be
// wrong; the administrator can always run /au session start.
func (bot *Bot) autoStart(guildID string) bool {
	config, err := bot.AUVC.GuildConfig(guildID)
	if err != nil {
		return false
	}
	return config.AutoStart
}

// StartSession begins applying the voice policy and brings Discord into line
// immediately, so the effect is visible rather than waiting for the next event.
func (bot *Bot) StartSession(guildID string) error {
	bot.CaptureSessions.SetMode(guildID, Running)
	return bot.reconcileCaptureSession(guildID)
}

// PauseSession stops applying voice changes and leaves Discord exactly as it is.
//
// The session keeps being tracked, so resuming acts on the round as it is then,
// not as it was when the pause started.
func (bot *Bot) PauseSession(guildID string) Mode {
	return bot.CaptureSessions.SetMode(guildID, Paused)
}

// StopSession stops managing voice and releases everyone.
func (bot *Bot) StopSession(guildID string) error {
	bot.CaptureSessions.SetMode(guildID, Stopped)
	return bot.releaseEveryone(guildID)
}

// follows reports whether a message belongs to the capture session this guild
// is currently listening to, adopting a new one when it introduces itself.
//
// Every connection gets its own protocol receiver, and a receiver cannot know
// that a newer connection has replaced it. So a capture that reconnects while
// the previous socket is still draining would otherwise have both streams
// writing the same session, and the older one would undo the snapshot that just
// rebuilt the round.
//
// A snapshot is what takes over, because the protocol requires one before any
// event on every connection: the first thing a new session says is always a
// complete picture. Anything else from a session this guild is not following is
// late by definition and is dropped.
//
// The caller holds the lock.
func (g *guildSession) follows(message protocol.Message) bool {
	incoming := protocol.Envelope(message).Session
	if incoming == "" || incoming == g.captureSession {
		return true
	}

	if _, isSnapshot := message.(*protocol.Snapshot); isSnapshot {
		g.captureSession = incoming
		return true
	}
	return false
}
