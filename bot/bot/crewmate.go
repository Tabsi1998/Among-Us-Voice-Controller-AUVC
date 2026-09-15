package bot

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/crewmate"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
	"github.com/bwmarrin/discordgo"
)

// crewmateAPI is the part of Discord the crewmate board talks to.
type crewmateAPI interface {
	ApplicationEmojis(appID string) ([]*discordgo.Emoji, error)
	CreateApplicationEmoji(appID string, params *discordgo.EmojiParams) (*discordgo.Emoji, error)
	SendMessage(channelID string, message *discordgo.MessageSend) (*discordgo.Message, error)
	EditMessage(edit *discordgo.MessageEdit) (*discordgo.Message, error)
	DeleteMessage(channelID, messageID string) error
}

// discordCrewmateAPI is crewmateAPI over a real Discord session.
type discordCrewmateAPI struct {
	session *discordgo.Session
}

func (d discordCrewmateAPI) ApplicationEmojis(appID string) ([]*discordgo.Emoji, error) {
	return d.session.ApplicationEmojis(appID)
}

func (d discordCrewmateAPI) CreateApplicationEmoji(appID string, params *discordgo.EmojiParams) (*discordgo.Emoji, error) {
	return d.session.ApplicationEmojiCreate(appID, params)
}

func (d discordCrewmateAPI) SendMessage(channelID string, message *discordgo.MessageSend) (*discordgo.Message, error) {
	return d.session.ChannelMessageSendComplex(channelID, message)
}

func (d discordCrewmateAPI) EditMessage(edit *discordgo.MessageEdit) (*discordgo.Message, error) {
	return d.session.ChannelMessageEditComplex(edit)
}

func (d discordCrewmateAPI) DeleteMessage(channelID, messageID string) error {
	return d.session.ChannelMessageDelete(channelID, messageID)
}

// BoardStore remembers where each guild's crewmate board is.
type BoardStore interface {
	CrewmateBoard(guildID string) (sqlite.CrewmateBoard, error)
	SaveCrewmateBoard(board sqlite.CrewmateBoard) error
	DeleteCrewmateBoard(guildID string) error
	CrewmateBoards() ([]sqlite.CrewmateBoard, error)
}

// CrewmateBoards keeps one message per guild in the control channel up to date,
// where players choose which crewmate they are.
//
// The message is edited in place rather than posted again, and only when what
// it shows has changed. Discord limits how often a channel's messages can be
// edited, and a round produces an event every few seconds; most of them change
// nothing on the board, and an unannounced death must not change it at all.
type CrewmateBoards struct {
	api   crewmateAPI
	store BoardStore
	appID string

	mu      sync.Mutex
	emojis  crewmate.Emojis
	boards  map[string]*crewmateBoard
	wake    map[string]chan struct{}
	closed  bool
	done    chan struct{}
	workers sync.WaitGroup
}

// crewmateBoard is what is known about one guild's message.
type crewmateBoard struct {
	mu        sync.Mutex
	loaded    bool
	channelID string
	messageID string
	rendered  string
}

// NewCrewmateBoards returns boards that post through api and remember their
// messages in store.
func NewCrewmateBoards(api crewmateAPI, store BoardStore, appID string) *CrewmateBoards {
	return &CrewmateBoards{
		api:    api,
		store:  store,
		appID:  appID,
		boards: map[string]*crewmateBoard{},
		wake:   map[string]chan struct{}{},
		done:   make(chan struct{}),
	}
}

// Emojis returns the crewmate pictures uploaded so far.
func (c *CrewmateBoards) Emojis() crewmate.Emojis {
	c.mu.Lock()
	defer c.mu.Unlock()

	emojis := make(crewmate.Emojis, len(c.emojis))
	for name, id := range c.emojis {
		emojis[name] = id
	}
	return emojis
}

// EmojiStatus reports how many crewmate pictures Discord has, out of how many
// there are.
func (c *CrewmateBoards) EmojiStatus() (uploaded, total int) {
	images, err := crewmate.Images()
	if err != nil {
		return 0, 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.emojis), len(images)
}

// UploadEmojis makes sure every crewmate picture exists as an emoji of this
// application and reports how many it had to upload.
//
// Application emojis belong to the bot rather than to a server, so they need
// no server permission and no free emoji slot, and they are uploaded once for
// every server the bot is in. A picture that is already there is left alone.
func (c *CrewmateBoards) UploadEmojis() (int, error) {
	images, err := crewmate.Images()
	if err != nil {
		return 0, err
	}

	existing, err := c.api.ApplicationEmojis(c.appID)
	if err != nil {
		return 0, fmt.Errorf("list the application's emojis: %w", err)
	}
	have := map[string]string{}
	for _, emoji := range existing {
		if emoji != nil && emoji.ID != "" {
			have[emoji.Name] = emoji.ID
		}
	}

	uploaded := 0
	var failures []error
	emojis := crewmate.Emojis{}
	for _, image := range images {
		if id, ok := have[image.Name]; ok {
			emojis[image.Name] = id
			continue
		}
		if c.isClosed() {
			break
		}

		created, err := c.api.CreateApplicationEmoji(c.appID, &discordgo.EmojiParams{
			Name:  image.Name,
			Image: image.DataURI,
		})
		if err != nil || created == nil {
			failures = append(failures, fmt.Errorf("upload %s: %w", image.Name, err))
			continue
		}
		emojis[image.Name] = created.ID
		uploaded++
	}

	c.mu.Lock()
	c.emojis = emojis
	c.mu.Unlock()

	return uploaded, errors.Join(failures...)
}

// Show makes a guild's board show view in channelID. An empty channelID takes
// the board down.
func (c *CrewmateBoards) Show(guildID, channelID string, view crewmate.Board) error {
	board := c.forGuild(guildID)

	board.mu.Lock()
	defer board.mu.Unlock()

	// Checked under the board's lock, which Close waits for: a refresh that was
	// already running finishes before the boards are taken down, and none can
	// put a board back afterwards.
	if c.isClosed() {
		return nil
	}
	c.load(guildID, board)

	if channelID == "" {
		return c.takeDown(guildID, board)
	}

	key := view.Key()
	switch {
	case board.messageID != "" && board.channelID == channelID:
		if board.rendered == key {
			return nil
		}

		embeds := []*discordgo.MessageEmbed{view.Embed}
		components := view.Components
		if components == nil {
			// An absent field leaves the old menu in place; an empty one
			// removes it.
			components = []discordgo.MessageComponent{}
		}
		edit := discordgo.NewMessageEdit(channelID, board.messageID)
		edit.Embeds = &embeds
		edit.Components = &components

		_, err := c.api.EditMessage(edit)
		if err == nil {
			board.rendered = key
			return nil
		}
		if !gone(err) {
			return fmt.Errorf("edit the crewmate board in guild %s: %w", guildID, err)
		}
		// Somebody deleted the message. Post it again rather than leaving the
		// server without a menu.
		board.messageID, board.rendered = "", ""

	case board.messageID != "":
		// The control channel changed.
		if err := c.takeDown(guildID, board); err != nil {
			log.Printf("Could not remove the crewmate board from its old channel in guild %s: %v", guildID, err)
			board.channelID, board.messageID, board.rendered = "", "", ""
		}
	}

	message, err := c.api.SendMessage(channelID, &discordgo.MessageSend{
		Embeds:     []*discordgo.MessageEmbed{view.Embed},
		Components: view.Components,
		// The board names players with mentions so everyone can see who is who.
		// Nobody should be pinged for it.
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	})
	if err != nil {
		return fmt.Errorf("post the crewmate board in guild %s: %w", guildID, err)
	}

	board.channelID, board.messageID, board.rendered = channelID, message.ID, key
	if err := c.store.SaveCrewmateBoard(sqlite.CrewmateBoard{
		GuildID: guildID, ChannelID: channelID, MessageID: message.ID,
	}); err != nil {
		return fmt.Errorf("remember the crewmate board in guild %s: %w", guildID, err)
	}
	return nil
}

// Close stops refreshing and takes every board down, including boards a
// previous run left behind.
//
// A board whose bot has stopped still shows a menu, and choosing from it only
// earns an "interaction failed". Removing it is the honest way to go offline.
func (c *CrewmateBoards) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	close(c.done)
	c.mu.Unlock()

	c.workers.Wait()

	records, err := c.store.CrewmateBoards()
	if err != nil {
		log.Printf("Could not read the crewmate boards to remove them: %v", err)
		return
	}
	for _, record := range records {
		board := c.forGuild(record.GuildID)
		board.mu.Lock()
		board.loaded = true
		board.channelID, board.messageID = record.ChannelID, record.MessageID
		if err := c.takeDown(record.GuildID, board); err != nil {
			log.Printf("Could not remove the crewmate board in guild %s: %v", record.GuildID, err)
		}
		board.mu.Unlock()
	}
}

// request asks for a guild's board to be refreshed without waiting for it.
//
// Requests that arrive while a refresh is running collapse into one more
// refresh, which reads the session as it is by then. A burst of capture events
// therefore costs at most two edits, and capture is never held up by Discord.
func (c *CrewmateBoards) request(guildID string, refresh func()) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	wake, running := c.wake[guildID]
	if !running {
		wake = make(chan struct{}, 1)
		c.wake[guildID] = wake

		c.workers.Add(1)
		go func() {
			defer c.workers.Done()
			for {
				select {
				case <-c.done:
					return
				case <-wake:
					refresh()
				}
			}
		}()
	}

	select {
	case wake <- struct{}{}:
	default:
	}
}

func (c *CrewmateBoards) forGuild(guildID string) *crewmateBoard {
	c.mu.Lock()
	defer c.mu.Unlock()

	board, ok := c.boards[guildID]
	if !ok {
		board = &crewmateBoard{}
		c.boards[guildID] = board
	}
	return board
}

func (c *CrewmateBoards) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// load reads where a board was left, once. The caller holds the board's lock.
func (c *CrewmateBoards) load(guildID string, board *crewmateBoard) {
	if board.loaded {
		return
	}
	board.loaded = true

	stored, err := c.store.CrewmateBoard(guildID)
	if err != nil {
		if !errors.Is(err, sqlite.ErrNotFound) {
			log.Printf("Could not read the crewmate board of guild %s: %v", guildID, err)
		}
		return
	}
	board.channelID, board.messageID = stored.ChannelID, stored.MessageID
}

// takeDown deletes a board's message and forgets it. The caller holds the
// board's lock.
func (c *CrewmateBoards) takeDown(guildID string, board *crewmateBoard) error {
	if board.messageID == "" {
		return nil
	}
	if err := c.api.DeleteMessage(board.channelID, board.messageID); err != nil && !gone(err) {
		// Remembered, so the next start can try again.
		return fmt.Errorf("delete the crewmate board: %w", err)
	}

	board.channelID, board.messageID, board.rendered = "", "", ""
	return c.store.DeleteCrewmateBoard(guildID)
}

// gone reports whether Discord says the message or its channel no longer
// exists.
func gone(err error) bool {
	var rest *discordgo.RESTError
	if !errors.As(err, &rest) {
		return false
	}
	if rest.Message != nil {
		switch rest.Message.Code {
		case discordgo.ErrCodeUnknownMessage, discordgo.ErrCodeUnknownChannel:
			return true
		}
	}
	return rest.Response != nil && rest.Response.StatusCode == http.StatusNotFound
}

// AttachCrewmates enables the crewmate board and the menu behind /au link, and
// uploads the crewmate pictures in the background.
func (bot *Bot) AttachCrewmates(store BoardStore) {
	bot.Crewmates = NewCrewmateBoards(
		discordCrewmateAPI{session: bot.PrimarySession}, store, bot.PrimarySession.State.User.ID)

	// A server that changes its language in Discord sees the board rewritten in
	// it straight away, rather than at the next change in the lobby.
	bot.PrimarySession.AddHandler(func(_ *discordgo.Session, update *discordgo.GuildUpdate) {
		if update.Guild != nil {
			bot.RefreshCrewmates(update.Guild.ID)
		}
	})

	go func() {
		uploaded, err := bot.Crewmates.UploadEmojis()
		if err != nil {
			log.Printf("Some crewmate pictures could not be uploaded; the menu shows names instead: %v", err)
		}
		if uploaded > 0 {
			log.Printf("Uploaded %d crewmate pictures", uploaded)
		}
		for _, guildID := range bot.CaptureSessions.guilds() {
			bot.RefreshCrewmates(guildID)
		}
	}()
}

// RefreshCrewmates brings a guild's crewmate board up to date in the
// background.
func (bot *Bot) RefreshCrewmates(guildID string) {
	if bot.Crewmates == nil || guildID == "" {
		return
	}
	bot.Crewmates.request(guildID, func() { bot.refreshCrewmateBoard(guildID) })
}

// refreshCrewmateBoard brings a guild's crewmate board up to date now.
func (bot *Bot) refreshCrewmateBoard(guildID string) {
	config, err := bot.AUVC.GuildConfig(guildID)
	if err != nil {
		log.Printf("Could not read the configuration for the crewmate board of guild %s: %v", guildID, err)
		return
	}
	// Everybody in the text channel reads the board, so it is in the server's
	// language rather than anybody's own.
	view, err := bot.crewmateMenu(guildID, bot.guildLanguage(guildID))
	if err != nil {
		log.Printf("Could not build the crewmate board of guild %s: %v", guildID, err)
		return
	}

	// The board goes where the server asked for AUVC's notices, and only once
	// capture has connected: before that there is no lobby to choose from.
	channelID := config.ControlTextChannelID
	if _, seen := bot.CaptureSessions.LastSeen(guildID); !seen {
		channelID = ""
	}

	if err := bot.Crewmates.Show(guildID, channelID, view); err != nil {
		log.Printf("Could not update the crewmate board of guild %s: %v", guildID, err)
	}
}

// crewmateMenu renders the board for a guild's current session, in a language.
func (bot *Bot) crewmateMenu(guildID string, language text.Language) (crewmate.Board, error) {
	if bot.CaptureSessions == nil {
		return crewmate.Render(language, nil, nil, nil), nil
	}
	_, _, players := bot.CaptureSessions.Snapshot(guildID)

	owners := map[string]string{}
	if bot.AUVCLinks != nil {
		links, err := bot.AUVCLinks.Links(guildID)
		if err != nil {
			return crewmate.Board{}, fmt.Errorf("read player links: %w", err)
		}
		for _, link := range links {
			owners[link.InGameName] = link.DiscordUserID
		}
	}

	var emojis crewmate.Emojis
	if bot.Crewmates != nil {
		emojis = bot.Crewmates.Emojis()
	}
	return crewmate.Render(language, players, owners, emojis), nil
}

// crewmatePicker answers /au link without a name: the menu, visible only to
// whoever asked.
func (bot *Bot) crewmatePicker(guildID string, language text.Language) *discordgo.InteractionResponse {
	view, err := bot.crewmateMenu(guildID, language)
	if err != nil {
		return auPrivateResponse(language.Say(text.CrewmateMenuFailed, err))
	}
	if len(view.Components) == 0 {
		return auPrivateResponse(language.Say(text.NoLobbyYet))
	}

	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsEphemeral,
			Embeds:     []*discordgo.MessageEmbed{view.Embed},
			Components: view.Components,
		},
	}
}

// handleCrewmateChoice links whoever chose from a crewmate menu.
//
// It goes through the same /au link and /au unlink as typing the command, so
// who may link whom and what is stored stay decided in one place.
func (bot *Bot) handleCrewmateChoice(s *discordgo.Session, interaction *discordgo.InteractionCreate) *discordgo.InteractionResponse {
	// The answer to a choice is private, even on the public board, so it is in
	// the Discord language of whoever chose.
	language := text.FromDiscord(string(interaction.Locale))

	if interaction.GuildID == "" || interaction.Member == nil || interaction.Member.User == nil {
		return crewmateReply(interaction, language.Say(text.CrewmateOnlyInServer))
	}
	if bot.AUVC == nil {
		return crewmateReply(interaction, language.Say(text.StorageUnavailable))
	}

	choice, ok := crewmate.ParseChoice(interaction.MessageComponentData().Values)
	if !ok {
		return crewmateReply(interaction, language.Say(text.ChoiceNotUnderstood))
	}

	invoker, err := invokerOf(s, interaction)
	if err != nil {
		return crewmateReply(interaction, language.Say(text.ServerUnavailable))
	}

	request := au.Request{
		GuildID:  interaction.GuildID,
		Command:  au.Unlink,
		Invoker:  invoker,
		Values:   au.Values{Strings: map[string]string{}, Booleans: map[string]bool{}, Integers: map[string]int64{}},
		Language: language,
	}
	if !choice.Unlink {
		// The menu may be older than the lobby. Linking somebody to a player
		// who has left would only look like it worked.
		if !bot.inLobby(interaction.GuildID, choice.Player) {
			bot.RefreshCrewmates(interaction.GuildID)
			return crewmateReply(interaction, language.Say(text.PlayerLeftLobby, choice.Player))
		}
		request.Command = au.Link
		request.Values.Strings[au.OptionPlayer] = choice.Player
	}

	content, err := bot.AUVC.Handle(request)
	if errors.Is(err, au.ErrUnauthorized) {
		return crewmateReply(interaction, language.Say(text.ChoiceNotAllowed))
	}
	if err != nil {
		return crewmateReply(interaction, language.Say(text.ChoiceNotSaved, err))
	}

	go bot.LinksChanged(interaction.GuildID)
	return crewmateReply(interaction, content)
}

// crewmateReply answers a choice. On the private menu from /au link the answer
// replaces the menu; on the public board it is a private message of its own,
// because the board belongs to everyone.
func crewmateReply(interaction *discordgo.InteractionCreate, content string) *discordgo.InteractionResponse {
	if interaction.Message != nil && interaction.Message.Flags&discordgo.MessageFlagsEphemeral != 0 {
		return &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    content,
				Embeds:     []*discordgo.MessageEmbed{},
				Components: []discordgo.MessageComponent{},
			},
		}
	}
	return auPrivateResponse(content)
}

// inLobby reports whether a player is in a guild's session and connected.
func (bot *Bot) inLobby(guildID, name string) bool {
	if bot.CaptureSessions == nil {
		return false
	}
	_, _, players := bot.CaptureSessions.Snapshot(guildID)
	for _, player := range players {
		if player.Name == name && !player.Disconnected {
			return true
		}
	}
	return false
}

// LinksChanged shows a new or removed link on the board and, while a
// session is running, applies it to voice straight away instead of at the next
// game event.
func (bot *Bot) LinksChanged(guildID string) {
	bot.RefreshCrewmates(guildID)

	if bot.CaptureSessions == nil || bot.CaptureSessions.Mode(guildID) != Running {
		return
	}
	if err := bot.reconcileCaptureSession(guildID); err != nil {
		log.Printf("Could not apply a changed player link in guild %s: %v", guildID, err)
	}
}

// guilds returns every guild with a session, in a stable order.
func (c *CaptureSessions) guilds() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	guilds := make([]string, 0, len(c.sessions))
	for guildID := range c.sessions {
		guilds = append(guilds, guildID)
	}
	sort.Strings(guilds)
	return guilds
}
