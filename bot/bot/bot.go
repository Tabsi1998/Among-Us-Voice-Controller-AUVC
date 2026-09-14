package bot

import (
	"log"
	"os"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/crewmate"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/voice"
	"github.com/bwmarrin/discordgo"
)

// Bot is the Discord side of AUVC.
//
// It holds no game state of its own. The session a capture drives lives in
// CaptureSessions, the configuration and player links live in SQLite, and the
// voice decisions are made by pkg/voice. What is left here is the Discord
// connection and the wiring between those.
type Bot struct {
	version string
	commit  string

	PrimarySession *discordgo.Session

	// AUVC answers the /au commands.
	AUVC *au.Service

	// CaptureSessions holds the live session of every guild whose capture is
	// connected. A self-hosted bot is one process, so a session that lives in
	// it needs no external store to be found again.
	CaptureSessions *CaptureSessions

	// AUVCLinks reads the persistent player links the capture path resolves
	// in-game names with.
	AUVCLinks LinkStore

	// Reconciler applies the voice policy. It is held on the bot rather than
	// created per call because it serializes work per guild, which only works
	// if every reconciliation goes through the same instance.
	Reconciler *voice.Reconciler

	// Crewmates keeps the crewmate board in each control channel up to date.
	// It stays nil until AttachCrewmates, and everything that uses it copes.
	Crewmates *CrewmateBoards
}

// MakeAndStartBot connects to Discord and returns the running bot, or nil if
// the connection could not be made.
func MakeAndStartBot(version, commit, botToken string, auvc *au.Service) *Bot {
	dg, err := discordgo.New("Bot " + botToken)
	if err != nil {
		log.Println("Could not create the Discord session:", err)
		return nil
	}

	bot := Bot{
		version:         version,
		commit:          commit,
		PrimarySession:  dg,
		AUVC:            auvc,
		CaptureSessions: NewCaptureSessions(),
		Reconciler:      voice.NewReconciler(discordApplier{session: dg}),
	}
	dg.LogLevel = discordgo.LogInformational

	dg.AddHandler(bot.handleInteractionCreate)
	dg.AddHandler(func(_ *discordgo.Session, _ *discordgo.Ready) {
		log.Println("Connected to Discord and ready for events")
	})

	// Guild voice states are what the reconciler compares against, and guilds
	// are needed to resolve members and channels. Nothing else is requested:
	// AUVC reads the game from capture, not from Discord messages.
	dg.Identify.Intents = discordgo.MakeIntent(
		discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuilds)

	if err := dg.Open(); err != nil {
		log.Println("Could not connect to Discord:", err)
		return nil
	}

	log.Println("Finished identifying to the Discord API")
	bot.announce()

	return &bot
}

// announce sets the activity Discord shows under the bot's name.
func (bot *Bot) announce() {
	listeningTo := os.Getenv("AUVC_LISTENING")
	if listeningTo == "" {
		listeningTo = "/au"
	}

	err := bot.PrimarySession.UpdateStatusComplex(discordgo.UpdateStatusData{
		Activities: []*discordgo.Activity{{
			Name: listeningTo,
			Type: discordgo.ActivityTypeListening,
		}},
	})
	if err != nil {
		log.Println("Could not set the bot activity:", err)
	}
}

// Close shuts the Discord connection down.
func (bot *Bot) Close() {
	// The boards go first, while Discord is still connected to delete them.
	if bot.Crewmates != nil {
		bot.Crewmates.Close()
	}
	if err := bot.PrimarySession.Close(); err != nil {
		log.Println("Could not close the Discord session cleanly:", err)
	}
}

// handleInteractionCreate routes Discord interactions: the one command AUVC
// registers, and choices from the crewmate menu.
func (bot *Bot) handleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var response *discordgo.InteractionResponse

	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		if i.ApplicationCommandData().Name != au.Name {
			return
		}
		response = bot.handleAUCommand(s, i)

	case discordgo.InteractionMessageComponent:
		if i.MessageComponentData().CustomID != crewmate.SelectID {
			return
		}
		response = bot.handleCrewmateChoice(s, i)

	default:
		return
	}

	if response == nil {
		return
	}
	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		log.Println("Could not answer an interaction:", err)
	}
}
