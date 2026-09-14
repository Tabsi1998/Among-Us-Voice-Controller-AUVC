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
	"slices"
	"strings"
	"sync"
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
	// Players choose their crewmate on a board in the control channel. The
	// pictures upload in the background, so starting is not held up by them.
	controller.AttachCrewmates(auvcDB)
	// The session commands need the bot, and the bot needed the service to
	// answer commands at all, so they are connected once both exist.
	auvcService.AttachSessionControl(bot.NewSessionControl(controller))
	diagnostician := bot.NewDoctor(controller)
	auvcService.AttachDoctor(diagnostician)

	captureServer := newCaptureServer(pairingService, controller)

	// The Windows app stops the bot it started by asking, rather than by
	// killing it, so the players in voice are released on the way out.
	stopRequested := make(chan struct{})
	requestStop := sync.OnceFunc(func() { close(stopRequested) })

	local, err := localControl(os.Getenv("AUVC_LOCAL_CONTROL_SECRET"), &localBackend{
		controller: controller,
		pairing:    pairingService,
		capture:    captureServer,
		doctor:     diagnostician,
		version:    version,
		stop:       requestStop,
	})
	if err != nil {
		return err
	}

	// The capture listener starts after the bot, because the bot is what
	// applies the messages it receives.
	stopCaptureListener := startCaptureListener(captureServer, controller, local)
	defer stopCaptureListener()

	// The fail-safe: a capture that dies mid-round would otherwise leave every
	// living player server-muted, and nothing in Discord expires that on its own.
	stopWatchdog := controller.WatchCapture()
	defer stopWatchdog()

	unpublish, err := publishCommands(controller, os.Getenv("SLASH_COMMAND_GUILD_IDS"))
	if err != nil {
		return err
	}

	log.Println("AUVC is running. Press CTRL-C to exit.")
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	select {
	case <-signals:
	case <-stopRequested:
		log.Println("The AUVC app asked the bot to stop")
	}

	log.Println("Shutting down")
	controller.ReleaseAll()
	unpublish()
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

// commandTargets reads SLASH_COMMAND_GUILD_IDS. Empty registers the commands
// globally, "*" in every guild the bot is in, and a comma-separated list in
// those guilds.
func commandTargets(configured string) (everyGuild bool, guilds []string) {
	configured = strings.ReplaceAll(configured, " ", "")
	if configured == "*" {
		return true, nil
	}

	for _, guild := range strings.Split(configured, ",") {
		if guild != "" {
			guilds = append(guilds, guild)
		}
	}
	if len(guilds) == 0 {
		return false, []string{""} // empty means global
	}
	return false, guilds
}

// publishCommands registers the command tree and returns what removes it again.
func publishCommands(controller *bot.Bot, configured string) (func(), error) {
	everyGuild, guilds := commandTargets(configured)
	if everyGuild {
		return publishInEveryGuild(controller), nil
	}

	registered, err := registerCommands(controller, guilds)
	if err != nil {
		return nil, err
	}
	return func() { unregisterCommands(controller, registered) }, nil
}

// registerCommands publishes the command tree to Discord.
//
// Registering to named guilds instead of globally is what makes a change
// visible immediately: a global registration can take up to an hour to appear.
func registerCommands(controller *bot.Bot, guilds []string) ([]registeredCommand, error) {
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

// publishInEveryGuild registers the commands in each guild the bot is in, now
// and in any guild it joins while running.
//
// This is what the Windows app uses: /au appears the moment the bot is invited,
// with no server id to look up and no hour to wait. Overwriting a guild's
// commands in bulk is idempotent, so a guild seen twice, on a reconnect for
// instance, costs one request and changes nothing.
func publishInEveryGuild(controller *bot.Bot) func() {
	session := controller.PrimarySession
	applicationID := session.State.User.ID

	var mu sync.Mutex
	published := map[string]bool{}

	publish := func(guildID string) {
		mu.Lock()
		if published[guildID] {
			mu.Unlock()
			return
		}
		published[guildID] = true
		mu.Unlock()

		if _, err := session.ApplicationCommandBulkOverwrite(applicationID, guildID, command.All); err != nil {
			log.Printf("Could not register /au in guild %s: %v", guildID, err)
			mu.Lock()
			delete(published, guildID)
			mu.Unlock()
			return
		}
		log.Printf("Registered /au in guild %s", guildID)
	}

	removeHandler := session.AddHandler(func(_ *discordgo.Session, created *discordgo.GuildCreate) {
		publish(created.ID)
	})

	session.State.RLock()
	current := make([]string, 0, len(session.State.Guilds))
	for _, guild := range session.State.Guilds {
		current = append(current, guild.ID)
	}
	session.State.RUnlock()

	for _, guildID := range current {
		publish(guildID)
	}

	return func() {
		removeHandler()

		mu.Lock()
		guilds := make([]string, 0, len(published))
		for guildID := range published {
			guilds = append(guilds, guildID)
		}
		mu.Unlock()
		slices.Sort(guilds)

		for _, guildID := range guilds {
			if _, err := session.ApplicationCommandBulkOverwrite(applicationID, guildID, []*discordgo.ApplicationCommand{}); err != nil {
				log.Printf("Could not remove /au from guild %s: %v", guildID, err)
			}
		}
	}
}
