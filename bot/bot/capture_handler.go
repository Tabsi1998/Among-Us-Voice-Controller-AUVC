package bot

import (
	"fmt"
	"sync"

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

// CaptureSessions holds the live session of every guild that has a capture
// connected.
//
// Work is serialized per guild. One capture produces one ordered stream, but a
// reconnect can overlap the tail of the previous connection, and two streams
// writing the same session would interleave into a state neither of them sent.
type CaptureSessions struct {
	mu       sync.Mutex
	sessions map[string]*session.Live
	locks    map[string]*sync.Mutex
}

// NewCaptureSessions returns an empty set.
func NewCaptureSessions() *CaptureSessions {
	return &CaptureSessions{
		sessions: map[string]*session.Live{},
		locks:    map[string]*sync.Mutex{},
	}
}

// forGuild returns a guild's session and the lock that guards it, creating
// both on first use.
func (c *CaptureSessions) forGuild(guildID string) (*session.Live, *sync.Mutex) {
	c.mu.Lock()
	defer c.mu.Unlock()

	live, ok := c.sessions[guildID]
	if !ok {
		live = session.NewLive()
		c.sessions[guildID] = live
		c.locks[guildID] = &sync.Mutex{}
	}
	return live, c.locks[guildID]
}

// Snapshot returns a copy of a guild's session for reading. It is what /au
// session status and /au doctor report.
func (c *CaptureSessions) Snapshot(guildID string) (game.Phase, []session.GamePlayer) {
	live, lock := c.forGuild(guildID)

	lock.Lock()
	defer lock.Unlock()

	return live.Phase(), live.Players()
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

	live, lock := bot.CaptureSessions.forGuild(guildID)

	lock.Lock()
	changed, err := applyCaptureMessage(live, message)
	lock.Unlock()

	if err != nil || !changed {
		return err
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

	live, lock := bot.CaptureSessions.forGuild(guildID)
	lock.Lock()
	state := live.Project(resolve)
	lock.Unlock()

	guild, err := bot.PrimarySession.State.Guild(guildID)
	if err != nil || guild == nil {
		return fmt.Errorf("discord guild %s is unavailable: %w", guildID, err)
	}

	if err := bot.Reconciler.Reconcile(guildID, observeVoiceStates(guild), voice.Desired(state, config)); err != nil {
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
