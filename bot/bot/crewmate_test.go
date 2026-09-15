package bot

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/crewmate"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/game"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/protocol"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/session"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/text"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/voice"
	"github.com/bwmarrin/discordgo"
)

const crewGuild = "crew-guild"

type sentCrewmateBoard struct {
	channelID string
	messageID string
	message   *discordgo.MessageSend
}

// fakeCrewmateAPI stands in for Discord and records what the board asked for.
type fakeCrewmateAPI struct {
	mu      sync.Mutex
	emojis  []*discordgo.Emoji
	created []string
	sent    []sentCrewmateBoard
	edits   []*discordgo.MessageEdit
	deleted []string
	editErr error
	nextID  int
	posted  chan struct{}
}

func (f *fakeCrewmateAPI) ApplicationEmojis(string) ([]*discordgo.Emoji, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*discordgo.Emoji(nil), f.emojis...), nil
}

func (f *fakeCrewmateAPI) CreateApplicationEmoji(_ string, params *discordgo.EmojiParams) (*discordgo.Emoji, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.nextID++
	emoji := &discordgo.Emoji{ID: fmt.Sprintf("emoji-%d", f.nextID), Name: params.Name}
	f.created = append(f.created, params.Name)
	f.emojis = append(f.emojis, emoji)
	return emoji, nil
}

func (f *fakeCrewmateAPI) SendMessage(channelID string, message *discordgo.MessageSend) (*discordgo.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.nextID++
	id := fmt.Sprintf("message-%d", f.nextID)
	f.sent = append(f.sent, sentCrewmateBoard{channelID: channelID, messageID: id, message: message})
	if f.posted != nil {
		select {
		case f.posted <- struct{}{}:
		default:
		}
	}
	return &discordgo.Message{ID: id, ChannelID: channelID}, nil
}

func (f *fakeCrewmateAPI) EditMessage(edit *discordgo.MessageEdit) (*discordgo.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.editErr != nil {
		err := f.editErr
		f.editErr = nil
		return nil, err
	}
	f.edits = append(f.edits, edit)
	return &discordgo.Message{ID: edit.ID, ChannelID: edit.Channel}, nil
}

func (f *fakeCrewmateAPI) DeleteMessage(channelID, messageID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.deleted = append(f.deleted, channelID+"/"+messageID)
	return nil
}

func (f *fakeCrewmateAPI) counts() (sent, edited, deleted int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent), len(f.edits), len(f.deleted)
}

func unknownMessage() error {
	return &discordgo.RESTError{
		Response: &http.Response{StatusCode: http.StatusNotFound},
		Message:  &discordgo.APIErrorMessage{Code: discordgo.ErrCodeUnknownMessage, Message: "Unknown Message"},
	}
}

func openCrewDB(t *testing.T) *sqlite.DB {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "auvc.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func boardWith(names ...string) crewmate.Board {
	players := make([]session.GamePlayer, 0, len(names))
	for index, name := range names {
		players = append(players, session.GamePlayer{Name: name, Color: index, Alive: true})
	}
	return crewmate.Render(text.English, crewmate.Round{}, players, nil, nil)
}

func show(t *testing.T, boards *CrewmateBoards, channelID string, view crewmate.Board) {
	t.Helper()
	if err := boards.Show(crewGuild, channelID, view); err != nil {
		t.Fatalf("show in %q: %v", channelID, err)
	}
}

func TestTheBoardIsPostedOnceAndEditedOnlyWhenItChanges(t *testing.T) {
	api := &fakeCrewmateAPI{}
	db := openCrewDB(t)
	boards := NewCrewmateBoards(api, db, "app")

	show(t, boards, "text-1", boardWith("Alice"))
	show(t, boards, "text-1", boardWith("Alice"))
	if sent, edited, _ := api.counts(); sent != 1 || edited != 0 {
		t.Fatalf("an unchanged board was sent %d times and edited %d times", sent, edited)
	}

	show(t, boards, "text-1", boardWith("Alice", "Bob"))
	if sent, edited, _ := api.counts(); sent != 1 || edited != 1 {
		t.Fatalf("a changed board should be edited in place: sent %d, edited %d", sent, edited)
	}

	if mentions := api.sent[0].message.AllowedMentions; mentions == nil || len(mentions.Parse) != 0 {
		t.Errorf("the board must not ping the players it names: %+v", mentions)
	}
	stored, err := db.CrewmateBoard(crewGuild)
	if err != nil || stored.MessageID != api.sent[0].messageID || stored.ChannelID != "text-1" {
		t.Errorf("the board was not remembered: %+v, %v", stored, err)
	}
}

func TestABoardSomebodyDeletedIsPostedAgain(t *testing.T) {
	api := &fakeCrewmateAPI{}
	db := openCrewDB(t)
	boards := NewCrewmateBoards(api, db, "app")

	show(t, boards, "text-1", boardWith("Alice"))
	api.editErr = unknownMessage()
	show(t, boards, "text-1", boardWith("Alice", "Bob"))

	if sent, _, _ := api.counts(); sent != 2 {
		t.Fatalf("a deleted board should be posted again, sent %d", sent)
	}
	stored, _ := db.CrewmateBoard(crewGuild)
	if stored.MessageID != api.sent[1].messageID {
		t.Errorf("the new message was not remembered: %+v", stored)
	}
}

// A rate limit or a network error says nothing about the message being gone.
// Posting a second board then would leave two menus in the channel.
func TestAFailedEditDoesNotPostASecondBoard(t *testing.T) {
	api := &fakeCrewmateAPI{}
	boards := NewCrewmateBoards(api, openCrewDB(t), "app")

	show(t, boards, "text-1", boardWith("Alice"))
	api.editErr = errors.New("connection reset")
	if err := boards.Show(crewGuild, "text-1", boardWith("Alice", "Bob")); err == nil {
		t.Fatal("the failed edit should be reported")
	}
	show(t, boards, "text-1", boardWith("Alice", "Bob"))

	if sent, edited, _ := api.counts(); sent != 1 || edited != 1 {
		t.Errorf("want one board edited on the retry, got sent %d, edited %d", sent, edited)
	}
}

func TestMovingTheTextChannelMovesTheBoard(t *testing.T) {
	api := &fakeCrewmateAPI{}
	db := openCrewDB(t)
	boards := NewCrewmateBoards(api, db, "app")

	show(t, boards, "text-1", boardWith("Alice"))
	show(t, boards, "text-2", boardWith("Alice"))

	if len(api.deleted) != 1 || api.deleted[0] != "text-1/"+api.sent[0].messageID {
		t.Errorf("the old board was not removed: %v", api.deleted)
	}
	if len(api.sent) != 2 || api.sent[1].channelID != "text-2" {
		t.Fatalf("the board was not posted in the new channel: %+v", api.sent)
	}
	if stored, _ := db.CrewmateBoard(crewGuild); stored.ChannelID != "text-2" {
		t.Errorf("the new channel was not remembered: %+v", stored)
	}
}

func TestWithoutATextChannelTheBoardIsTakenDown(t *testing.T) {
	api := &fakeCrewmateAPI{}
	db := openCrewDB(t)
	boards := NewCrewmateBoards(api, db, "app")

	show(t, boards, "text-1", boardWith("Alice"))
	show(t, boards, "", boardWith("Alice"))

	if _, _, deleted := api.counts(); deleted != 1 {
		t.Errorf("the board was not removed, %d deletes", deleted)
	}
	if _, err := db.CrewmateBoard(crewGuild); !errors.Is(err, sqlite.ErrNotFound) {
		t.Errorf("the removed board is still remembered: %v", err)
	}
}

// The app can be killed, and then nobody takes the board down. The next start
// must pick the same message up instead of adding a second one.
func TestARestartedBotEditsTheBoardItLeftBehind(t *testing.T) {
	api := &fakeCrewmateAPI{}
	db := openCrewDB(t)

	show(t, NewCrewmateBoards(api, db, "app"), "text-1", boardWith("Alice"))
	show(t, NewCrewmateBoards(api, db, "app"), "text-1", boardWith("Alice"))

	if sent, edited, _ := api.counts(); sent != 1 || edited != 1 {
		t.Errorf("want the old board edited, got sent %d, edited %d", sent, edited)
	}
}

func TestStoppingTakesEveryBoardDownAndPostsNoMore(t *testing.T) {
	api := &fakeCrewmateAPI{}
	db := openCrewDB(t)
	if err := db.SaveCrewmateBoard(sqlite.CrewmateBoard{GuildID: "earlier", ChannelID: "text-9", MessageID: "message-9"}); err != nil {
		t.Fatalf("seed a board from an earlier run: %v", err)
	}
	boards := NewCrewmateBoards(api, db, "app")

	show(t, boards, "text-1", boardWith("Alice"))
	boards.Close()
	boards.Close()
	show(t, boards, "text-1", boardWith("Alice", "Bob"))

	if sent, edited, deleted := api.counts(); sent != 1 || edited != 0 || deleted != 2 {
		t.Errorf("want both boards deleted and nothing posted after stopping, got sent %d, edited %d, deleted %d",
			sent, edited, deleted)
	}
	if remaining, _ := db.CrewmateBoards(); len(remaining) != 0 {
		t.Errorf("boards are still remembered: %+v", remaining)
	}
}

func TestOnlyMissingPicturesAreUploaded(t *testing.T) {
	api := &fakeCrewmateAPI{emojis: []*discordgo.Emoji{{ID: "existing", Name: "auvc_red"}}}
	boards := NewCrewmateBoards(api, openCrewDB(t), "app")

	images, err := crewmate.Images()
	if err != nil {
		t.Fatalf("images: %v", err)
	}

	uploaded, err := boards.UploadEmojis()
	if err != nil || uploaded != len(images)-1 {
		t.Fatalf("uploaded %d (%v), want %d", uploaded, err, len(images)-1)
	}
	for _, name := range api.created {
		if name == "auvc_red" {
			t.Error("a picture that already existed was uploaded again")
		}
	}
	if boards.Emojis()["auvc_red"] != "existing" {
		t.Error("the existing picture is not used")
	}
	if have, total := boards.EmojiStatus(); have != total || total != len(images) {
		t.Errorf("status %d of %d, want %d of %d", have, total, len(images), len(images))
	}

	if again, err := boards.UploadEmojis(); err != nil || again != 0 {
		t.Errorf("a second start uploaded %d pictures (%v)", again, err)
	}
}

// newCrewBot is a bot whose server has a text channel for the board, and whose
// Discord is a fake.
func newCrewBot(t *testing.T) (*Bot, *fakeCrewmateAPI, *sqlite.DB, *discordgo.Session) {
	t.Helper()

	db := openCrewDB(t)
	config := sqlite.DefaultGuildConfig(crewGuild)
	config.MainVoiceChannelID = "main"
	config.GhostVoiceChannelID = "ghost"
	config.ControlTextChannelID = "text-1"
	config.AutoStart = false
	if err := db.SaveGuildConfig(config); err != nil {
		t.Fatalf("save configuration: %v", err)
	}

	state := discordgo.NewState()
	if err := state.GuildAdd(&discordgo.Guild{
		ID:          crewGuild,
		OwnerID:     "owner",
		VoiceStates: []*discordgo.VoiceState{{GuildID: crewGuild, UserID: "member", ChannelID: "main"}},
	}); err != nil {
		t.Fatalf("add guild state: %v", err)
	}
	s := &discordgo.Session{State: state}

	api := &fakeCrewmateAPI{posted: make(chan struct{}, 8)}
	bot := &Bot{
		PrimarySession:  s,
		AUVC:            au.NewService(db, "test", "test"),
		CaptureSessions: NewCaptureSessions(),
		AUVCLinks:       db,
		Crewmates:       NewCrewmateBoards(api, db, "app"),
	}
	t.Cleanup(bot.Crewmates.Close)
	return bot, api, db, s
}

func lobbyOf(bot *Bot, phase game.Phase, names ...string) {
	players := make([]session.GamePlayer, 0, len(names))
	for index, name := range names {
		players = append(players, session.GamePlayer{Name: name, Color: index, Alive: true})
	}
	guild := bot.CaptureSessions.forGuild(crewGuild)
	guild.mu.Lock()
	guild.live.Reset(phase, players)
	guild.mu.Unlock()
}

func chooseCrewmate(user, value string, private bool) *discordgo.InteractionCreate {
	interaction := &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: crewGuild,
		Member:  &discordgo.Member{User: &discordgo.User{ID: user}},
		Data: discordgo.MessageComponentInteractionData{
			CustomID:      crewmate.SelectID,
			ComponentType: discordgo.SelectMenuComponent,
			Values:        []string{value},
		},
		Message: &discordgo.Message{},
	}}
	if private {
		interaction.Message.Flags = discordgo.MessageFlagsEphemeral
	}
	return interaction
}

func TestTheBoardAppearsOnceCaptureConnects(t *testing.T) {
	bot, api, _, _ := newCrewBot(t)

	// No capture yet: there is no lobby to choose from.
	bot.refreshCrewmateBoard(crewGuild)
	if sent, _, _ := api.counts(); sent != 0 {
		t.Fatalf("a board was posted before capture connected")
	}

	err := bot.HandleCapture(crewGuild, &protocol.Snapshot{
		Header:  protocol.Header{Protocol: protocol.Version, Type: protocol.TypeSnapshot, Session: "session-a", Seq: 1},
		Phase:   protocol.PhaseLobby,
		Players: []protocol.Player{{Name: "Alice", Color: game.Red}},
	})
	if err != nil {
		t.Fatalf("handle snapshot: %v", err)
	}

	select {
	case <-api.posted:
	case <-time.After(5 * time.Second):
		t.Fatal("capture connected, but no board was posted")
	}

	api.mu.Lock()
	defer api.mu.Unlock()
	board := api.sent[0]
	if board.channelID != "text-1" {
		t.Errorf("posted in %q, want the text channel", board.channelID)
	}
	if len(board.message.Embeds) != 1 || !strings.Contains(board.message.Embeds[0].Description, "Alice") {
		t.Errorf("the board does not show the lobby: %+v", board.message.Embeds)
	}
}

func TestChoosingACrewmateLinksWhoeverChose(t *testing.T) {
	bot, _, db, s := newCrewBot(t)
	lobbyOf(bot, game.LOBBY, "Alice")

	response := bot.handleCrewmateChoice(s, chooseCrewmate("member", "player:Alice", false))

	if response.Type != discordgo.InteractionResponseChannelMessageWithSource ||
		response.Data.Flags != discordgo.MessageFlagsEphemeral {
		t.Errorf("a choice on the public board must be answered privately: %+v", response)
	}
	if !strings.Contains(response.Data.Content, "Alice") {
		t.Errorf("unexpected answer: %q", response.Data.Content)
	}

	links, err := db.Links(crewGuild)
	if err != nil || len(links) != 1 || links[0].InGameName != "Alice" || links[0].DiscordUserID != "member" {
		t.Errorf("the choice was not saved as a link: %+v, %v", links, err)
	}
}

// A board can be older than the lobby. Linking somebody to a player who has
// left would look like it worked and do nothing in the next round.
func TestAPlayerWhoLeftCannotBeChosen(t *testing.T) {
	bot, _, db, s := newCrewBot(t)
	lobbyOf(bot, game.LOBBY, "Alice")

	response := bot.handleCrewmateChoice(s, chooseCrewmate("member", "player:Bob", false))

	if !strings.Contains(response.Data.Content, "not in the lobby") {
		t.Errorf("unexpected answer: %q", response.Data.Content)
	}
	if links, _ := db.Links(crewGuild); len(links) != 0 {
		t.Errorf("a player who left was linked: %+v", links)
	}
}

// The answer to a choice is private even on the public board, so it follows the
// Discord language of whoever chose.
func TestTheAnswerToAChoiceIsInTheLanguageOfWhoeverChose(t *testing.T) {
	bot, _, _, s := newCrewBot(t)
	lobbyOf(bot, game.LOBBY, "Alice")

	choice := chooseCrewmate("member", "player:Bob", false)
	choice.Locale = discordgo.German
	response := bot.handleCrewmateChoice(s, choice)

	if want := text.German.Say(text.PlayerLeftLobby, "Bob"); response.Data.Content != want {
		t.Errorf("got %q, want %q", response.Data.Content, want)
	}
}

func TestUnlinkMeRemovesTheLink(t *testing.T) {
	bot, _, db, s := newCrewBot(t)
	if err := db.SaveLink(crewGuild, "Alice", "member"); err != nil {
		t.Fatalf("link: %v", err)
	}

	bot.handleCrewmateChoice(s, chooseCrewmate("member", "unlink", false))

	if links, _ := db.Links(crewGuild); len(links) != 0 {
		t.Errorf("the link is still there: %+v", links)
	}
}

func TestAChoiceFromThePrivateMenuReplacesTheMenu(t *testing.T) {
	bot, _, _, s := newCrewBot(t)
	lobbyOf(bot, game.LOBBY, "Alice")

	response := bot.handleCrewmateChoice(s, chooseCrewmate("member", "player:Alice", true))

	if response.Type != discordgo.InteractionResponseUpdateMessage {
		t.Fatalf("response type %d, want an update of the private menu", response.Type)
	}
	if response.Data.Components == nil || len(response.Data.Components) != 0 {
		t.Errorf("the menu should be removed once chosen: %+v", response.Data.Components)
	}
}

func TestLinkWithoutANameOffersTheMenu(t *testing.T) {
	bot, _, db, s := newCrewBot(t)
	link := func(options ...*discordgo.ApplicationCommandInteractionDataOption) *discordgo.InteractionResponse {
		return bot.handleAUCommand(s, &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
			Type:    discordgo.InteractionApplicationCommand,
			GuildID: crewGuild,
			Member:  &discordgo.Member{User: &discordgo.User{ID: "member"}},
			Data: discordgo.ApplicationCommandInteractionData{
				Name: au.Name,
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{Name: au.Link, Type: discordgo.ApplicationCommandOptionSubCommand, Options: options},
				},
			},
		}})
	}

	if response := link(); !strings.Contains(response.Data.Content, "no Among Us lobby") {
		t.Errorf("without a lobby, /au link should say what to do: %q", response.Data.Content)
	}

	lobbyOf(bot, game.LOBBY, "Alice")
	response := link()
	if response.Data.Flags != discordgo.MessageFlagsEphemeral || len(response.Data.Components) != 1 || len(response.Data.Embeds) != 1 {
		t.Fatalf("/au link should offer the menu privately: %+v", response.Data)
	}

	// Typing the name still works as before.
	link(&discordgo.ApplicationCommandInteractionDataOption{
		Name: au.OptionPlayer, Type: discordgo.ApplicationCommandOptionString, Value: "Alice",
	})
	if links, _ := db.Links(crewGuild); len(links) != 1 || links[0].DiscordUserID != "member" {
		t.Errorf("/au link player:Alice did not link: %+v", links)
	}
}

type crewApplier struct {
	mu      sync.Mutex
	changes []voice.Change
}

func (a *crewApplier) Apply(_ string, change voice.Change) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.changes = append(a.changes, change)
	return nil
}

// A player who links themselves mid-round should be muted now, not at the next
// game event, which during tasks can be minutes away.
func TestANewLinkIsAppliedToVoiceStraightAway(t *testing.T) {
	bot, _, db, _ := newCrewBot(t)
	applier := &crewApplier{}
	bot.Reconciler = voice.NewReconciler(applier)
	lobbyOf(bot, game.TASKS, "Alice")
	bot.CaptureSessions.SetMode(crewGuild, Running)

	if err := db.SaveLink(crewGuild, "Alice", "member"); err != nil {
		t.Fatalf("link: %v", err)
	}
	bot.LinksChanged(crewGuild)

	applier.mu.Lock()
	defer applier.mu.Unlock()
	for _, change := range applier.changes {
		if change.UserID == "member" && change.Muted != nil && *change.Muted {
			return
		}
	}
	t.Errorf("the newly linked player was not muted: %+v", applier.changes)
}
