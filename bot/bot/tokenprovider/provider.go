package tokenprovider

import (
	"context"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"github.com/bsm/redislock"
	"github.com/bwmarrin/discordgo"
	"github.com/go-redis/redis/v8"
	"log"
	"strconv"
	"sync"
	"time"
)

type TokenProvider struct {
	client         *redis.Client
	primarySession *discordgo.Session

	maxRequests5Seconds int64
	taskTimeoutMs       time.Duration
}

func NewTokenProvider(client *redis.Client, sess *discordgo.Session, taskTimeout time.Duration, maxReq int64) *TokenProvider {
	return &TokenProvider{
		client:              client,
		primarySession:      sess,
		maxRequests5Seconds: maxReq,
		taskTimeoutMs:       taskTimeout,
	}
}

func (tp *TokenProvider) Init(client *redis.Client, sess *discordgo.Session) {
	tp.client = client
	tp.primarySession = sess
}

func (tokenProvider *TokenProvider) IncrAndTestGuildTokenComboLock(guildID, hashToken string) bool {
	i, err := tokenProvider.client.Incr(context.Background(), rediskey.GuildTokenLock(guildID, hashToken)).Result()
	if err != nil {
		log.Println(err)
	}
	usable := i < tokenProvider.maxRequests5Seconds
	log.Printf("Token/capture %s on guild %s is at count %d. Using?: %v", hashToken, guildID, i, usable)
	if !usable {
		return false
	}

	// set the expiry only if the mute/deafen was successful, because we want to preserve any existing blacklist expiries
	err = tokenProvider.client.Expire(context.Background(), rediskey.GuildTokenLock(guildID, hashToken), time.Second*5).Err()
	if err != nil {
		log.Println(err)
	}

	return true
}

// BlacklistTokenForDuration sets a guild token (or connect code ala capture bot) to the maximum value allowed before
// attempting other non-rate-limited mute/deafen methods.
// NOTE: this will manifest as the capture/token in question appearing like it "has been used <maxnum> times" in logs,
// even if this is not technically accurate. A more accurate approach would probably use a totally separate Redis key,
// as opposed to this approach, which simply uses the ratelimiting counter key(s) to achieve blacklisting
func (tokenProvider *TokenProvider) BlacklistTokenForDuration(guildID, hashToken string, duration time.Duration) error {
	return tokenProvider.client.Set(context.Background(), rediskey.GuildTokenLock(guildID, hashToken), tokenProvider.maxRequests5Seconds, duration).Err()
}

const DefaultMaxWorkers = 8

var UnresponsiveCaptureBlacklistDuration = time.Minute * time.Duration(5)

func (tokenProvider *TokenProvider) ModifyUsers(guildID, connectCode string, request task.UserModifyRequest, voicelock *redislock.Lock) error {
	if voicelock != nil {
		defer voicelock.Release(context.Background())
	}

	gid, gerr := strconv.ParseUint(guildID, 10, 64)
	if gerr != nil {
		return gerr
	}

	tasksChannel := make(chan task.UserModify, len(request.Users))
	wg := sync.WaitGroup{}

	lock := sync.Mutex{}

	var latestErr error
	// start a handful of workers to handle the tasks
	for i := 0; i < DefaultMaxWorkers; i++ {
		go func() {
			for req := range tasksChannel {
				userIDStr := strconv.FormatUint(req.UserID, 10)
				if !tokenProvider.attemptOnCaptureBot(guildID, connectCode, gid, req) {
					log.Printf("Applying mute=%v, deaf=%v using primary bot\n", req.Mute, req.Deaf)
					err := task.ApplyMuteDeaf(tokenProvider.primarySession, guildID, userIDStr, req.Mute, req.Deaf)
					if err != nil {
						lock.Lock()
						latestErr = err
						lock.Unlock()
						log.Println("Error on primary bot:")
						log.Println(err)
					}
				}
				wg.Done()
			}
		}()
	}

	for _, modifyReq := range request.Users {
		wg.Add(1)
		tasksChannel <- modifyReq
	}
	wg.Wait()
	close(tasksChannel)

	return latestErr
}

func (tokenProvider *TokenProvider) waitForAck(pubsub *redis.PubSub, result chan<- bool) {
	t := time.NewTimer(tokenProvider.taskTimeoutMs)
	defer pubsub.Close()
	channel := pubsub.Channel()

	for {
		select {
		case <-t.C:
			t.Stop()
			result <- false
			return
		case val := <-channel:
			t.Stop()
			result <- val.Payload == "true"
			return
		}
	}
}

func (tokenProvider *TokenProvider) Close() {
	tokenProvider.primarySession.Close()
}
