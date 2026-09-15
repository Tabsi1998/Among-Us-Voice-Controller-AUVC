// Package localcontrol lets the AUVC Windows app set up and watch the bot it
// started on the same PC.
//
// The app launches the bot as a child process and hands it a random secret in
// AUVC_LOCAL_CONTROL_SECRET. Only a caller that presents that secret, connects
// from this computer and is not a browser gets an answer. Without the variable
// none of these routes exist.
//
// The routes do what an administrator would otherwise do in Discord: choose the
// channels, turn on auto start and pair capture. Issuing a credential without a
// pairing code is safe here for the same reason the rest is: whoever holds the
// secret started this bot, with its token.
package localcontrol

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	// Named lang in this package: its tests declare a text helper.
	lang "github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
)

// MinSecretLength is the shortest secret accepted. The app generates far more
// than this; the floor exists so that a hand-set placeholder cannot quietly
// protect an interface that can issue credentials.
const MinSecretLength = 32

// maxBodyBytes caps a request body. A setup is a few hundred bytes.
const maxBodyBytes = 16 * 1024

var (
	// ErrUnknownGuild means the bot is not in the server the app asked about.
	ErrUnknownGuild = errors.New("the bot is not in that server")
	// ErrInvalidSetup means the app asked for a setup the bot cannot use.
	ErrInvalidSetup = errors.New("invalid setup")
	// ErrInvalidLink means the app asked to link a crewmate or a member the bot
	// cannot use.
	ErrInvalidLink = errors.New("invalid link")
)

// Backend is what the app may read and change. The bot implements it; the tests
// fake it.
type Backend interface {
	Status() Status
	Channels(guildID string) ([]Channel, error)
	// Guild describes a guild, with its checks written in the app's language.
	Guild(guildID string, language lang.Language) (Guild, error)
	Configure(guildID string, setup Setup) error
	// Crewmates reports who plays in the lobby and whom they can be linked to.
	Crewmates(guildID string) (Crewmates, error)
	// Link links a crewmate to a member, or unlinks it for an empty user id.
	Link(guildID string, link Link) error
	IssueCredential(guildID string) (string, error)
	// Shutdown stops the bot. It is called after the answer has been sent.
	Shutdown()
}

// Status is the bot as a whole.
type Status struct {
	Connected bool           `json:"connected"`
	BotID     string         `json:"bot_id"`
	BotName   string         `json:"bot_name"`
	Version   string         `json:"version"`
	Guilds    []GuildSummary `json:"guilds"`
}

// GuildSummary is one server the bot is in.
type GuildSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Channel kinds the app can choose from.
const (
	KindVoice = "voice"
	KindText  = "text"
)

// Channel is one channel the app can offer for selection.
type Channel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Position int    `json:"position"`
}

// Guild is one server: how it is set up and how the doctor judges it.
type Guild struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	MainVoiceChannelID   string `json:"main_voice_channel_id"`
	GhostVoiceChannelID  string `json:"ghost_voice_channel_id"`
	ControlTextChannelID string `json:"control_text_channel_id"`
	AutoStart            bool   `json:"auto_start"`
	CaptureConnections   int    `json:"capture_connections"`
	// Session is running, paused or stopped, as /au session status reports it.
	Session string  `json:"session"`
	Checks  []Check `json:"checks"`
}

// Check is one line of the doctor's report.
type Check struct {
	Name   string `json:"name"`
	Level  string `json:"level"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

// Setup is what the app chose for a server.
type Setup struct {
	MainVoiceChannelID   string `json:"main_voice_channel_id"`
	GhostVoiceChannelID  string `json:"ghost_voice_channel_id"`
	ControlTextChannelID string `json:"control_text_channel_id"`
	AutoStart            bool   `json:"auto_start"`
}

// Crewmates is who plays in the lobby, and whom the app can link them to.
type Crewmates struct {
	Players []Crewmate `json:"players"`
	Members []Member   `json:"members"`
}

// Crewmate is one player in the lobby. UserID is empty while nobody is linked.
type Crewmate struct {
	Name   string `json:"name"`
	Color  string `json:"color"`
	UserID string `json:"user_id"`
	// Muted, Deafened and InGhostChannel are what AUVC holds on the linked member
	// right now. All are false for a crewmate nobody is linked to.
	Muted          bool `json:"muted"`
	Deafened       bool `json:"deafened"`
	InGhostChannel bool `json:"in_ghost_channel"`
}

// Member is a Discord member the app can link a crewmate to.
type Member struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Link is the app linking a crewmate to a member, or unlinking it with an empty
// user id.
type Link struct {
	Player string `json:"player"`
	UserID string `json:"user_id"`
}

// Server answers the app.
type Server struct {
	secret  []byte
	backend Backend
}

// New builds a server, refusing a secret too short to protect it.
func New(secret string, backend Backend) (*Server, error) {
	if len(secret) < MinSecretLength {
		return nil, fmt.Errorf("AUVC_LOCAL_CONTROL_SECRET must be at least %d characters", MinSecretLength)
	}
	if backend == nil {
		return nil, errors.New("local control needs a backend")
	}
	return &Server{secret: []byte(secret), backend: backend}, nil
}

// Register adds the routes to a mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /local/status", s.guard(s.status))
	mux.HandleFunc("GET /local/guilds/{guild}", s.guard(s.guild))
	mux.HandleFunc("GET /local/guilds/{guild}/channels", s.guard(s.channels))
	mux.HandleFunc("PUT /local/guilds/{guild}/setup", s.guard(s.setup))
	mux.HandleFunc("GET /local/guilds/{guild}/crewmates", s.guard(s.crewmates))
	mux.HandleFunc("PUT /local/guilds/{guild}/links", s.guard(s.link))
	mux.HandleFunc("POST /local/guilds/{guild}/credential", s.guard(s.credential))
	mux.HandleFunc("POST /local/shutdown", s.guard(s.shutdown))
}

// guard lets a request through only from this computer, not from a browser,
// and with the secret.
//
// A web page can make a browser send requests to localhost. It cannot read the
// secret, and it cannot set an Authorization header on a cross-origin request
// without a preflight this server never answers, but refusing anything with an
// Origin header closes the door before either of those has to hold.
func (s *Server) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")

		if !fromThisComputer(r.RemoteAddr) {
			writeError(w, http.StatusForbidden, "remote", "local control only answers this computer")
			return
		}
		if r.Header.Get("Origin") != "" {
			writeError(w, http.StatusForbidden, "browser", "local control does not answer browsers")
			return
		}
		presented, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(presented), s.secret) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized", "the local control secret is missing or wrong")
			return
		}

		next(w, r)
	}
}

func fromThisComputer(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	status := s.backend.Status()
	if status.Guilds == nil {
		status.Guilds = []GuildSummary{}
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) guild(w http.ResponseWriter, r *http.Request) {
	guild, err := s.backend.Guild(r.PathValue("guild"), languageOf(r))
	if s.failed(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, guild)
}

// languageOf is the language the app asks for in Accept-Language, which it sets
// to its own. Only the first, preferred tag counts; anything AUVC does not speak
// is English.
func languageOf(r *http.Request) lang.Language {
	preferred, _, _ := strings.Cut(r.Header.Get("Accept-Language"), ",")
	preferred, _, _ = strings.Cut(preferred, ";")
	return lang.FromDiscord(strings.TrimSpace(preferred))
}

func (s *Server) channels(w http.ResponseWriter, r *http.Request) {
	channels, err := s.backend.Channels(r.PathValue("guild"))
	if s.failed(w, err) {
		return
	}
	if channels == nil {
		channels = []Channel{}
	}
	writeJSON(w, http.StatusOK, channels)
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	var setup Setup
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&setup); err != nil || decoder.More() {
		writeError(w, http.StatusBadRequest, "malformed", "the request is not a setup")
		return
	}

	guildID := r.PathValue("guild")
	if s.failed(w, s.backend.Configure(guildID, setup)) {
		return
	}

	// The answer is the guild as it now stands, so the app shows what was
	// saved rather than what it believes it sent.
	guild, err := s.backend.Guild(guildID, languageOf(r))
	if s.failed(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, guild)
}

func (s *Server) crewmates(w http.ResponseWriter, r *http.Request) {
	crewmates, err := s.backend.Crewmates(r.PathValue("guild"))
	if s.failed(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, filled(crewmates))
}

func (s *Server) link(w http.ResponseWriter, r *http.Request) {
	var link Link
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&link); err != nil || decoder.More() || link.Player == "" {
		writeError(w, http.StatusBadRequest, "malformed", "the request is not a link")
		return
	}

	guildID := r.PathValue("guild")
	if s.failed(w, s.backend.Link(guildID, link)) {
		return
	}

	// The answer is the lobby as it now stands, like the answer to a setup.
	crewmates, err := s.backend.Crewmates(guildID)
	if s.failed(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, filled(crewmates))
}

// filled turns missing lists into empty ones, which the app reads as "none"
// rather than as a broken answer.
func filled(crewmates Crewmates) Crewmates {
	if crewmates.Players == nil {
		crewmates.Players = []Crewmate{}
	}
	if crewmates.Members == nil {
		crewmates.Members = []Member{}
	}
	return crewmates
}

func (s *Server) credential(w http.ResponseWriter, r *http.Request) {
	token, err := s.backend.IssueCredential(r.PathValue("guild"))
	if s.failed(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"credential": token})
}

func (s *Server) shutdown(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "stopping"})
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	// After answering: stopping closes the listener this request arrived on.
	go s.backend.Shutdown()
}

// failed answers an error and reports whether there was one.
func (s *Server) failed(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, ErrUnknownGuild):
		writeError(w, http.StatusNotFound, "unknown_guild", err.Error())
	case errors.Is(err, ErrInvalidSetup):
		writeError(w, http.StatusBadRequest, "invalid_setup", err.Error())
	case errors.Is(err, ErrInvalidLink):
		writeError(w, http.StatusBadRequest, "invalid_link", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "unavailable", err.Error())
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
