package main

import (
	_ "embed"
	"errors"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/bot/command"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/bot/tokenprovider"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/capture"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/locale"
	storage2 "github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/storage"
	"github.com/bwmarrin/discordgo"
	"io"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/storage"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/bot"
)

var (
	version = "v8.1.0"
	commit  = "none"
	date    = "unknown"
)

//go:embed storage/postgres.sql
var postgresFileContents string

const (
	DefaultURL                   = "http://localhost:8123"
	DefaultMaxRequests5Sec int64 = 7
)

type registeredCommand struct {
	GuildID            string
	ApplicationCommand *discordgo.ApplicationCommand
}

func main() {
	// seed the rand generator (used for making connection codes)
	rand.Seed(time.Now().Unix())
	err := discordMainWrapper()
	if err != nil {
		log.Println("Program exited with the following error:")
		log.Println(err)
		return
	}
}

func discordMainWrapper() error {
	discordToken := os.Getenv("DISCORD_BOT_TOKEN")
	if discordToken == "" {
		return errors.New("no DISCORD_BOT_TOKEN provided")
	}
	logPath := os.Getenv("LOG_PATH")
	if logPath == "" {
		logPath = "./"
	}

	logEntry := os.Getenv("DISABLE_LOG_FILE")
	if logEntry == "" {
		file, err := os.Create(path.Join(logPath, "logs.txt"))
		if err != nil {
			return err
		}
		mw := io.MultiWriter(os.Stdout, file)
		log.SetOutput(mw)
	}

	emojiGuildID := os.Getenv("EMOJI_GUILD_ID")

	log.Println(version + "-" + commit)

	url := os.Getenv("HOST")
	if url == "" {
		log.Printf("[Info] No valid HOST provided. Defaulting to %s\n", DefaultURL)
		url = DefaultURL
	}

	var redisClient bot.RedisInterface
	var storageInterface storage.StorageInterface

	redisAddr := os.Getenv("REDIS_ADDR")
	redisPassword := os.Getenv("REDIS_PASS")
	if redisAddr != "" {
		err := redisClient.Init(storage.RedisParameters{
			Addr:     redisAddr,
			Username: "",
			Password: redisPassword,
		})
		if err != nil {
			log.Println(err)
		}
		err = storageInterface.Init(storage.RedisParameters{
			Addr:     redisAddr,
			Username: "",
			Password: redisPassword,
		})
		if err != nil {
			log.Println(err)
		}
	} else {
		return errors.New("no REDIS_ADDR specified; exiting")
	}

	locale.InitLang(os.Getenv("LOCALE_PATH"), os.Getenv("BOT_LANG"))

	psql := storage2.PsqlInterface{}
	pAddr := os.Getenv("POSTGRES_ADDR")
	if pAddr == "" {
		return errors.New("no POSTGRES_ADDR specified; exiting")
	}

	pUser := os.Getenv("POSTGRES_USER")
	if pUser == "" {
		return errors.New("no POSTGRES_USER specified; exiting")
	}

	pPass := os.Getenv("POSTGRES_PASS")
	if pPass == "" {
		return errors.New("no POSTGRES_PASS specified; exiting")
	}

	err := psql.Init(storage2.ConstructPsqlConnectURL(pAddr, pUser, pPass))
	if err != nil {
		return err
	}

	go func() {
		err := psql.ExecFromString(postgresFileContents)
		if err != nil {
			log.Println("Exiting with fatal error when attempting to execute postgres.sql:")
			log.Fatal(err)
		}
	}()

	log.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	taskTimeoutms := capture.DefaultCaptureBotTimeout

	taskTimeoutmsStr := os.Getenv("ACK_TIMEOUT_MS")
	num, err := strconv.ParseInt(taskTimeoutmsStr, 10, 64)
	if err == nil {
		log.Printf("Read from env; using ACK_TIMEOUT_MS=%d\n", num)
		taskTimeoutms = time.Millisecond * time.Duration(num)
	}

	maxReq5Sec := os.Getenv("MAX_REQ_5_SEC")
	maxReq := DefaultMaxRequests5Sec
	num, err = strconv.ParseInt(maxReq5Sec, 10, 64)
	if err == nil {
		maxReq = num
	}

	tokenProvider := tokenprovider.NewTokenProvider(nil, nil, taskTimeoutms, maxReq)
	b := bot.MakeAndStartBot(version, commit, discordToken, url, emojiGuildID, &redisClient, &storageInterface, &psql, logPath)
	if b == nil {
		log.Fatal("bot failed to initialize; did you provide a valid Discord Bot Token?")
	}

	b.InitTokenProvider(tokenProvider)
	b.TokenProvider = tokenProvider
	// empty string entry = global
	slashCommandGuildIds := []string{""}
	slashCommandGuildIdStr := strings.ReplaceAll(os.Getenv("SLASH_COMMAND_GUILD_IDS"), " ", "")
	if slashCommandGuildIdStr != "" {
		slashCommandGuildIds = strings.Split(slashCommandGuildIdStr, ",")
	}

	var registeredCommands []registeredCommand
	for _, guild := range slashCommandGuildIds {
		for _, v := range command.All {
			if guild == "" {
				log.Printf("Registering command %s GLOBALLY\n", v.Name)
			} else {
				log.Printf("Registering command %s in guild %s\n", v.Name, guild)
			}

			id, err := b.PrimarySession.ApplicationCommandCreate(b.PrimarySession.State.User.ID, guild, v)
			if err != nil {
				log.Panicf("Cannot create command: %v", err)
			} else {
				registeredCommands = append(registeredCommands, registeredCommand{
					GuildID:            guild,
					ApplicationCommand: id,
				})
			}
		}
	}
	log.Println("Finishing registering all commands!")

	<-sc
	log.Printf("Received Sigterm or Kill signal. Bot will terminate in 1 second")
	time.Sleep(time.Second)

	log.Println("Deleting slash commands")
	for _, v := range registeredCommands {
		if v.GuildID == "" {
			log.Printf("Deleting command %s GLOBALLY\n", v.ApplicationCommand.Name)
		} else {
			log.Printf("Deleting command %s on guild %s\n", v.ApplicationCommand.Name, v.GuildID)
		}
		err = b.PrimarySession.ApplicationCommandDelete(v.ApplicationCommand.ApplicationID, v.GuildID, v.ApplicationCommand.ID)
		if err != nil {
			log.Println(err)
		}
	}
	log.Println("Finished deleting all commands")

	b.Close()
	tokenProvider.Close()
	return nil
}
