package main

import (
	"errors"
	"io"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/bot"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/bot/command"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/au"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/locale"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/pairing"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage/sqlite"
	"github.com/bwmarrin/discordgo"
)

var (
	version = "v8.1.0"
	commit  = "none"
	date    = "unknown"
)

// containerDatabasePath is where the image keeps the database. It is a mounted
// volume, so a container restart does not lose the configuration.
const containerDatabasePath = "/data/amongus.db"

// defaultDatabasePath picks a sensible place for the database when
// AUVC_DATABASE_PATH says nothing.
//
// The container sets the variable itself, so this is the answer for somebody
// running the binary directly. On Windows that is beside the user's other
// application data: "/data/amongus.db" would land at the root of the current
// drive, which is neither writable nor anywhere anyone would look for it.
func defaultDatabasePath() string {
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "AUVC", "amongus.db")
		}
	}
	if config, err := os.UserConfigDir(); err == nil && config != "" {
		return filepath.Join(config, "auvc", "amongus.db")
	}
	return containerDatabasePath
}

type registeredCommand struct {
	GuildID            string
	ApplicationCommand *discordgo.ApplicationCommand
}

func main() {
	if err := run(); err != nil {
		log.Println("AUVC exited with the following error:")
		log.Println(err)
		// A failure has to be a failing exit code. A container manager, a
		// service supervisor and a shell script all decide what to do next
		// from this number, and zero tells all three that everything went
		// fine.
		os.Exit(1)
	}
}

func run() error {
	discordToken := os.Getenv("DISCORD_BOT_TOKEN")
	if discordToken == "" {
		return errors.New("no DISCORD_BOT_TOKEN provided")
	}

	if err := startLogging(); err != nil {
		return err
	}
	log.Printf("AUVC %s-%s (%s)", version, commit, date)

	locale.InitLang(os.Getenv("LOCALE_PATH"), os.Getenv("BOT_LANG"))

	databasePath := os.Getenv("AUVC_DATABASE_PATH")
	if databasePath == "" {
		databasePath = defaultDatabasePath()
	}
	// Said out loud because it is the first question when a setting seems to
	// have been forgotten: the answer is usually that the database is
	// somewhere other than where the reader assumed.
	log.Printf("Database: %s", databasePath)
	auvcDB, err := sqlite.Open(databasePath)
	if err != nil {
		return err
	}
	defer auvcDB.Close()

	// The pairing service shares the database, so a credential issued by
	// /au capture pair is the same one the transport checks later.
	pairingService := pairing.NewService(auvcDB)
	auvcService := au.NewServiceWithPairing(auvcDB, pairingService, version, commit)

	controller := bot.MakeAndStartBot(version, commit, discordToken, auvcService)
	if controller == nil {
		return errors.New("the bot failed to start; is the Discord bot token valid?")
	}
	defer controller.Close()

	controller.AUVCLinks = auvcDB
	// The session commands need the bot, and the bot needed the service to
	// answer commands at all, so they are connected once both exist.
	auvcService.AttachSessionControl(bot.NewSessionControl(controller))
	auvcService.AttachDoctor(bot.NewDoctor(controller))

	// The capture listener starts after the bot, because the bot is what
	// applies the messages it receives.
	stopCaptureListener := startCaptureListener(pairingService, controller)
	defer stopCaptureListener()

	// The fail-safe: a capture that dies mid-round would otherwise leave every
	// living player server-muted, and nothing in Discord expires that on its own.
	stopWatchdog := controller.WatchCapture()
	defer stopWatchdog()

	registered, err := registerCommands(controller)
	if err != nil {
		return err
	}

	log.Println("AUVC is running. Press CTRL-C to exit.")
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-signals

	log.Println("Shutting down")
	unregisterCommands(controller, registered)
	return nil
}

// startLogging mirrors the log to a file unless that is switched off.
func startLogging() error {
	if os.Getenv("DISABLE_LOG_FILE") != "" {
		return nil
	}

	logPath := os.Getenv("LOG_PATH")
	if logPath == "" {
		logPath = "./"
	}

	file, err := os.Create(path.Join(logPath, "logs.txt"))
	if err != nil {
		return err
	}
	log.SetOutput(io.MultiWriter(os.Stdout, file))
	return nil
}

// registerCommands publishes the command tree to Discord.
//
// SLASH_COMMAND_GUILD_IDS registers to named guilds instead of globally, which
// is what makes a change visible immediately: a global registration can take
// up to an hour to appear.
func registerCommands(controller *bot.Bot) ([]registeredCommand, error) {
	guilds := []string{""} // empty means global
	if configured := strings.ReplaceAll(os.Getenv("SLASH_COMMAND_GUILD_IDS"), " ", ""); configured != "" {
		guilds = strings.Split(configured, ",")
	}

	var registered []registeredCommand
	for _, guild := range guilds {
		for _, definition := range command.All {
			where := "globally"
			if guild != "" {
				where = "in guild " + guild
			}
			log.Printf("Registering /%s %s", definition.Name, where)

			created, err := controller.PrimarySession.ApplicationCommandCreate(
				controller.PrimarySession.State.User.ID, guild, definition)
			if err != nil {
				return registered, err
			}
			registered = append(registered, registeredCommand{
				GuildID:            guild,
				ApplicationCommand: created,
			})
		}
	}
	return registered, nil
}

// unregisterCommands removes what registerCommands published.
//
// A failure here is logged rather than returned: the process is already on its
// way out, and a command left registered is a cosmetic problem next to failing
// to shut down.
func unregisterCommands(controller *bot.Bot, registered []registeredCommand) {
	for _, entry := range registered {
		err := controller.PrimarySession.ApplicationCommandDelete(
			entry.ApplicationCommand.ApplicationID, entry.GuildID, entry.ApplicationCommand.ID)
		if err != nil {
			log.Printf("Could not remove /%s: %v", entry.ApplicationCommand.Name, err)
		}
	}
}
