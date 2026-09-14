package au

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/credential"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/pairing"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/voice"
)

var (
	ErrUnauthorized = errors.New("not authorized")
	ErrInvalidInput = errors.New("invalid input")
	ErrUnknownPath  = errors.New("unknown /au command")
)

// Store is the persistent state the currently implemented /au commands need.
// Later capture and session services extend the handler through separate
// interfaces instead of teaching Discord handlers about their internals.
type Store interface {
	EnsureGuildConfig(guildID string) (sqlite.GuildConfig, error)
	SaveGuildConfig(config sqlite.GuildConfig) error
	ReplaceLink(guildID, inGameName, discordUserID string) error
	DeleteLink(guildID, inGameName string) error
	DeleteLinksForUser(guildID, discordUserID string) error
	SchemaVersion() (int, error)
}

// Diagnostician answers /au doctor. Like SessionController it returns a
// finished reply: what to say about a broken setup is Discord knowledge, and it
// lives with the code that can actually look at Discord.
type Diagnostician interface {
	Diagnose(guildID string) (string, error)
}

// SessionController answers the /au session commands.
//
// It returns finished replies rather than values this service renders: what to
// say about a running session is Discord knowledge, and it lives with the code
// that owns the session. This service stays a router.
type SessionController interface {
	Start(guildID string) (string, error)
	Stop(guildID string) (string, error)
	Pause(guildID string) (string, error)
	Resume(guildID string) (string, error)
	Status(guildID string) (string, error)
}

// Service executes /au commands without depending on a Discord connection.
type Service struct {
	store   Store
	capture *pairing.Service
	session SessionController
	doctor  Diagnostician
	version string
	commit  string
	mu      sync.Mutex
}

// NewService builds a service without capture pairing. The /au capture
// commands report their staged status until a pairing service is supplied.
func NewService(store Store, version, commit string) *Service {
	return &Service{store: store, version: version, commit: commit}
}

// NewServiceWithPairing builds a service that can issue and withdraw capture
// credentials.
func NewServiceWithPairing(store Store, capture *pairing.Service, version, commit string) *Service {
	return &Service{store: store, capture: capture, version: version, commit: commit}
}

// AttachSessionControl enables the /au session commands. It is set after
// construction because the controller needs the bot, and the bot needs this
// service to answer commands at all.
func (s *Service) AttachSessionControl(controller SessionController) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.session = controller
}

// AttachDoctor enables the full /au doctor report. Without it the command
// still answers, with the checks that need no Discord connection.
func (s *Service) AttachDoctor(diagnostician Diagnostician) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.doctor = diagnostician
}

// GuildConfig returns a guild's configuration, creating the default row when
// the guild has not been configured yet.
//
// It takes the same lock as Handle. A command performs a read-modify-write, and
// reading between those two halves would hand out a configuration that is about
// to change - which for the voice path means acting on channels an
// administrator has just replaced.
func (s *Service) GuildConfig(guildID string) (sqlite.GuildConfig, error) {
	if s == nil || s.store == nil {
		return sqlite.GuildConfig{}, errors.New("AUVC configuration store is unavailable")
	}
	if guildID == "" {
		return sqlite.GuildConfig{}, fmt.Errorf("%w: guild is required", ErrInvalidInput)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.store.EnsureGuildConfig(guildID)
}

// VoiceConfig returns the part of a guild's configuration the voice policy
// reads, and whether the guild is set up well enough to manage voice at all.
func (s *Service) VoiceConfig(guildID string) (voice.Config, bool, error) {
	config, err := s.GuildConfig(guildID)
	if err != nil {
		return voice.Config{}, false, err
	}

	return voice.Config{
		MainChannelID:   config.MainVoiceChannelID,
		GhostChannelID:  config.GhostVoiceChannelID,
		AutoMoveGhosts:  config.AutoMoveGhosts,
		EnforceChannels: config.EnforceChannels,
	}, Ready(config), nil
}

// CaptureStatus reports a guild's capture pairing, for /au doctor.
func (s *Service) CaptureStatus(guildID string) (pairing.Status, error) {
	if s.capture == nil {
		return pairing.Status{}, errors.New("capture pairing is unavailable in this build")
	}
	return s.capture.Status(guildID)
}

// Version is the build, rendered the way /au version renders it.
func (s *Service) Version() string {
	return fmt.Sprintf("AUVC %s (`%s`)", fallback(s.version, "development"), fallback(s.commit, "unknown"))
}

// SchemaVersion reports the highest applied migration.
func (s *Service) SchemaVersion() (int, error) {
	if s == nil || s.store == nil {
		return 0, errors.New("AUVC configuration store is unavailable")
	}
	return s.store.SchemaVersion()
}

// Handle serializes configuration changes so two simultaneous Discord
// interactions cannot overwrite each other's read-modify-write update.
func (s *Service) Handle(request Request) (string, error) {
	if s == nil || s.store == nil {
		return "", errors.New("AUVC configuration store is unavailable")
	}
	if request.GuildID == "" || request.Invoker.UserID == "" {
		return "", fmt.Errorf("%w: guild and invoking user are required", ErrInvalidInput)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	config, err := s.store.EnsureGuildConfig(request.GuildID)
	if err != nil {
		return "", fmt.Errorf("load guild configuration: %w", err)
	}

	targetUserID, _ := request.Values.String(OptionUser)
	scope := ScopeFor(request.Group, request.Command)
	if (request.Command == Link || request.Command == Unlink) &&
		TargetsAnotherUser(request.Invoker.UserID, targetUserID) {
		scope = ScopeAdmin
	}
	if !Authorized(request.Invoker, config.AdminRoleID, scope) {
		return "", ErrUnauthorized
	}

	switch request.Group {
	case GroupSetup:
		return s.handleSetup(request, config)
	case GroupSettings:
		return s.handleSettings(request, config)
	case GroupCapture:
		return s.handleCapture(request)
	case GroupSession:
		return s.handleSession(request)
	case "":
		return s.handleDirect(request, config)
	default:
		return "", fmt.Errorf("%w: group %q", ErrUnknownPath, request.Group)
	}
}

func (s *Service) handleSetup(request Request, config sqlite.GuildConfig) (string, error) {
	switch request.Command {
	case SetupChannels:
		mainChannel, mainOK := request.Values.String(OptionMainChannel)
		ghostChannel, ghostOK := request.Values.String(OptionGhostChannel)
		if !mainOK || !ghostOK || mainChannel == "" || ghostChannel == "" {
			return "", fmt.Errorf("%w: main and ghost channels are required", ErrInvalidInput)
		}
		config.MainVoiceChannelID = mainChannel
		config.GhostVoiceChannelID = ghostChannel
		config.ControlTextChannelID, _ = request.Values.String(OptionControlChannel)
		if err := s.save(config); err != nil {
			return "", err
		}
		return "✅ AUVC channels saved.\n" + formatChannels(config), nil

	case SetupPermissions:
		roleID, ok := request.Values.String(OptionAdminRole)
		if !ok || roleID == "" {
			return "", fmt.Errorf("%w: an admin role is required", ErrInvalidInput)
		}
		config.AdminRoleID = roleID
		if err := s.save(config); err != nil {
			return "", err
		}
		return fmt.Sprintf("✅ AUVC administrators set to <@&%s>.", roleID), nil

	case SetupReset:
		confirmed, ok := request.Values.Bool(OptionConfirm)
		if !ok || !confirmed {
			return "", fmt.Errorf("%w: reset requires confirm=true", ErrInvalidInput)
		}
		if err := s.save(sqlite.DefaultGuildConfig(request.GuildID)); err != nil {
			return "", err
		}
		return "✅ Guild configuration reset to AUVC defaults. Persistent player links were kept.", nil

	default:
		return "", fmt.Errorf("%w: setup %q", ErrUnknownPath, request.Command)
	}
}

func (s *Service) handleSettings(request Request, config sqlite.GuildConfig) (string, error) {
	switch request.Command {
	case SettingsShow:
		return formatSettings(config), nil

	case SettingsExport:
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return "", fmt.Errorf("export configuration: %w", err)
		}
		return "```json\n" + string(data) + "\n```", nil

	case SettingsPreset:
		policy, ok := request.Values.String(OptionPolicy)
		if !ok || policy == "" {
			return "", fmt.Errorf("%w: a voice policy is required", ErrInvalidInput)
		}
		config.VoicePolicy = policy
		if policy == "ghost-chat" {
			config.AutoMoveGhosts = true
			config.EnforceChannels = true
		}

	case SettingsVoice:
		if enabled, ok := request.Values.Bool(OptionEnabled); ok {
			config.Enabled = enabled
		}
		if policy, ok := request.Values.String(OptionPolicy); ok {
			config.VoicePolicy = policy
		}

	case SettingsGhosts:
		if autoMove, ok := request.Values.Bool(OptionAutoMoveGhosts); ok {
			config.AutoMoveGhosts = autoMove
		}
		if enforce, ok := request.Values.Bool(OptionEnforce); ok {
			config.EnforceChannels = enforce
		}

	case SettingsSafety:
		if timeout, ok := request.Values.Integer(OptionTimeout); ok {
			config.CaptureTimeoutSeconds = int(timeout)
		}
		if action, ok := request.Values.String(OptionTimeoutAction); ok {
			config.CaptureTimeoutAction = action
		}
		if autoStart, ok := request.Values.Bool(OptionAutoStart); ok {
			config.AutoStart = autoStart
		}

	default:
		return "", fmt.Errorf("%w: settings %q", ErrUnknownPath, request.Command)
	}

	if err := s.save(config); err != nil {
		return "", err
	}
	return "✅ AUVC settings saved.\n" + formatSettings(config), nil
}

func (s *Service) handleDirect(request Request, config sqlite.GuildConfig) (string, error) {
	switch request.Command {
	case Link:
		player, ok := request.Values.String(OptionPlayer)
		player = strings.TrimSpace(player)
		if !ok || player == "" {
			return "", fmt.Errorf("%w: player name is required", ErrInvalidInput)
		}
		userID, ok := request.Values.String(OptionUser)
		if !ok || userID == "" {
			userID = request.Invoker.UserID
		}
		if err := s.store.ReplaceLink(request.GuildID, player, userID); err != nil {
			return "", fmt.Errorf("save player link: %w", err)
		}
		return fmt.Sprintf("✅ Linked <@%s> to Among Us player `%s`.", userID, player), nil

	case Unlink:
		userID, ok := request.Values.String(OptionUser)
		if !ok || userID == "" {
			userID = request.Invoker.UserID
		}
		if err := s.store.DeleteLinksForUser(request.GuildID, userID); err != nil {
			return "", fmt.Errorf("remove player link: %w", err)
		}
		return fmt.Sprintf("✅ Removed persistent Among Us links for <@%s>.", userID), nil

	case Doctor:
		if s.doctor != nil {
			return s.doctor.Diagnose(request.GuildID)
		}
		version, err := s.store.SchemaVersion()
		if err != nil {
			return "", fmt.Errorf("read database schema: %w", err)
		}
		lines := []string{fmt.Sprintf("✅ SQLite reachable; schema migration %d applied.", version)}
		for _, problem := range Validate(config) {
			lines = append(lines, "❌ "+problem.Error())
		}
		for _, problem := range NotReady(config) {
			lines = append(lines, "⚠️ "+problem.Error())
		}
		if Valid(config) && Ready(config) {
			lines = append(lines, "✅ Guild configuration is valid and has both voice channels.")
		}
		lines = append(lines, "⚠️ Discord, permission and capture checks need a running bot and are unavailable here.")
		return strings.Join(lines, "\n"), nil

	case Version:
		return fmt.Sprintf("AUVC %s (`%s`)", fallback(s.version, "development"), fallback(s.commit, "unknown")), nil

	default:
		return "", fmt.Errorf("%w: %q", ErrUnknownPath, request.Command)
	}
}

func (s *Service) save(config sqlite.GuildConfig) error {
	if problems := Validate(config); len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalidInput, joinProblems(problems))
	}
	if err := s.store.SaveGuildConfig(config); err != nil {
		return fmt.Errorf("save guild configuration: %w", err)
	}
	return nil
}

func formatChannels(config sqlite.GuildConfig) string {
	control := "not configured"
	if config.ControlTextChannelID != "" {
		control = "<#" + config.ControlTextChannelID + ">"
	}
	return fmt.Sprintf("Main: <#%s>\nGhost: <#%s>\nControl: %s",
		config.MainVoiceChannelID, config.GhostVoiceChannelID, control)
}

func formatSettings(config sqlite.GuildConfig) string {
	adminRole := "not configured"
	if config.AdminRoleID != "" {
		adminRole = "<@&" + config.AdminRoleID + ">"
	}
	return fmt.Sprintf("**AUVC settings**\nEnabled: %t\n%s\nAdmin role: %s\nVoice policy: `%s`\nAuto-move ghosts: %t\nEnforce channels: %t\nCapture timeout: %ds (`%s`)\nAuto-start: %t\nConfig version: %d",
		config.Enabled, formatChannels(config), adminRole, config.VoicePolicy,
		config.AutoMoveGhosts, config.EnforceChannels, config.CaptureTimeoutSeconds,
		config.CaptureTimeoutAction, config.AutoStart, config.ConfigVersion)
}

func joinProblems(problems []Problem) string {
	parts := make([]string, len(problems))
	for index, problem := range problems {
		parts[index] = problem.Error()
	}
	return strings.Join(parts, "; ")
}

// handleCapture runs the pairing commands.
//
// Every /au reply is ephemeral, which is what makes it acceptable to put a
// pairing code in one at all: it reaches the administrator who asked and
// nobody else in the channel.
func (s *Service) handleCapture(request Request) (string, error) {
	if s.capture == nil {
		return captureStaged(request.Command), nil
	}

	switch request.Command {
	case CapturePair:
		code, expires, err := s.capture.Pair(request.GuildID, request.Invoker.UserID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(
			"🔗 Pairing code: `%s`\n\n"+
				"Type it into the AUVC capture app on the PC that runs Among Us. "+
				"It works once and expires %s (in %s). "+
				"Running this command again replaces it.",
			code.Display(), expires.UTC().Format(time.RFC3339), credential.PairingCodeLifetime), nil

	case CaptureStatus:
		status, err := s.capture.Status(request.GuildID)
		if err != nil {
			return "", err
		}
		return status.Describe(), nil

	case CaptureRevoke:
		confirmed, ok := request.Values.Bool(OptionConfirm)
		if !ok || !confirmed {
			return "", fmt.Errorf("%w: revoke requires confirm=true", ErrInvalidInput)
		}
		revoked, err := s.capture.Revoke(request.GuildID)
		if err != nil {
			return "", err
		}
		if revoked == 0 {
			return "✅ Nothing to revoke: no capture was paired. Any outstanding pairing code was cancelled.", nil
		}
		return fmt.Sprintf(
			"✅ Revoked %d capture credential(s), cancelled any outstanding pairing code "+
				"and disconnected any capture app that was still connected. "+
				"Run `/au capture pair` to connect a capture app again.", revoked), nil

	default:
		return "", fmt.Errorf("%w: capture %q", ErrUnknownPath, request.Command)
	}
}

func captureStaged(command string) string {
	return fmt.Sprintf("⚠️ `/au capture %s` is registered, but capture pairing is unavailable in this build.", command)
}

// handleSession routes the session commands to the controller.
func (s *Service) handleSession(request Request) (string, error) {
	if s.session == nil {
		return fmt.Sprintf(
			"⚠️ `/au session %s` is registered, but session control is unavailable in this build.",
			request.Command), nil
	}

	switch request.Command {
	case SessionStart:
		return s.session.Start(request.GuildID)
	case SessionStop:
		return s.session.Stop(request.GuildID)
	case SessionPause:
		return s.session.Pause(request.GuildID)
	case SessionResume:
		return s.session.Resume(request.GuildID)
	case SessionStatus:
		return s.session.Status(request.GuildID)
	default:
		return "", fmt.Errorf("%w: session %q", ErrUnknownPath, request.Command)
	}
}

func fallback(value, replacement string) string {
	if value == "" {
		return replacement
	}
	return value
}
