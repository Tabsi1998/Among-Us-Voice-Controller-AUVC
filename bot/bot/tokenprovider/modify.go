package tokenprovider

import (
	"context"
	"encoding/json"
	"github.com/automuteus/automuteus/v8/pkg/rediskey"
	"github.com/automuteus/automuteus/v8/pkg/task"
	"log"
)

func (tokenProvider *TokenProvider) attemptOnCaptureBot(guildID, connectCode string, gid uint64, request task.UserModify) bool {
	// this is cheeky, but use the connect code as part of the lock; don't issue too many requests on the capture client w/ this code
	if tokenProvider.IncrAndTestGuildTokenComboLock(guildID, connectCode) {
		// if the secondary token didn't work, then next we try the client-side capture request
		taskObj := task.NewModifyTask(gid, request.UserID, task.PatchParams{
			Deaf: request.Deaf,
			Mute: request.Mute,
		})
		jBytes, err := json.Marshal(taskObj)
		if err != nil {
			log.Println(err)
			return false
		}
		acked := make(chan bool)
		// now we wait for an ack with respect to actually performing the mute
		pubsub := tokenProvider.client.Subscribe(context.Background(), rediskey.CompleteTask(taskObj.TaskID))
		err = tokenProvider.client.Publish(context.Background(), rediskey.TasksList(connectCode), jBytes).Err()
		if err != nil {
			log.Println("Error in publishing task to " + rediskey.TasksList(connectCode))
			log.Println(err)
		} else {
			go tokenProvider.waitForAck(pubsub, acked)
			res := <-acked
			if res {
				log.Println("Successful mute/deafen using client capture bot!")

				// hooray! we did the mute with a client token!
				return true
			}
			err = tokenProvider.BlacklistTokenForDuration(guildID, connectCode, UnresponsiveCaptureBlacklistDuration)
			if err == nil {
				log.Printf("No ack from capture clients; blacklisting capture client for gamecode \"%s\" for %s\n", connectCode, UnresponsiveCaptureBlacklistDuration.String())
			}
		}
	} else {
		log.Println("Capture client is probably rate-limited. Deferring to main bot instead")
	}
	return false
}
