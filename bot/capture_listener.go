package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/bot"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/pairing"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/transport"
)

// defaultCaptureAddr is where the capture app looks when nothing says
// otherwise. Localhost covers the common case, which is one person running
// the bot and the game on the same PC.
const defaultCaptureAddr = "127.0.0.1:8123"

// startCaptureListener brings up the direct capture connection, if one is
// configured. It returns a function that shuts the listener down, which is a
// no-op when no listener was started.
//
// AUVC_CAPTURE_ADDR sets the address. It defaults to localhost, which is the
// answer when the bot and the game run on the same machine, and is the safe
// one everywhere else: reaching a capture on another machine is a decision an
// operator makes deliberately. AUVC_CAPTURE_ADDR=off turns the listener off,
// which leaves a bot no capture can reach and is only useful for testing.
func startCaptureListener(pairingService *pairing.Service, controller *bot.Bot) func() {
	address := os.Getenv("AUVC_CAPTURE_ADDR")
	if address == "" {
		address = defaultCaptureAddr
	}
	if strings.EqualFold(address, "off") {
		log.Println("AUVC_CAPTURE_ADDR is off; no capture app can connect")
		return func() {}
	}

	certFile := os.Getenv("AUVC_CAPTURE_TLS_CERT")
	keyFile := os.Getenv("AUVC_CAPTURE_TLS_KEY")

	// The requirements call for a secure connection. Terminating TLS at a
	// reverse proxy is a legitimate way to get one and is how most self-hosted
	// deployments already run, so a plain listener is allowed rather than
	// refused. It is said out loud, every start, because a plain listener that
	// is not behind a proxy carries credentials in the clear and nothing else
	// would ever tell the operator.
	secure := certFile != "" && keyFile != ""
	if !secure {
		log.Printf("WARNING: the capture listener on %s is not terminating TLS itself. "+
			"Put it behind a reverse proxy that does, or set AUVC_CAPTURE_TLS_CERT and "+
			"AUVC_CAPTURE_TLS_KEY. Without one of the two, capture credentials travel in the clear.",
			address)
	}

	server := transport.NewServer(pairingService, controller, nil)

	httpServer := &http.Server{
		Addr:    address,
		Handler: routes(server, controller),
		// A handshake that stalls must not hold a connection open forever. The
		// WebSocket itself manages its own deadlines once it is upgraded.
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		var err error
		if secure {
			log.Printf("Capture listener accepting secure connections on %s", address)
			err = httpServer.ListenAndServeTLS(certFile, keyFile)
		} else {
			log.Printf("Capture listener accepting connections on %s", address)
			err = httpServer.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Println("Capture listener stopped:", err)
		}
	}()

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Println("Capture listener did not shut down cleanly:", err)
		}
	}
}

// routes puts the health endpoint beside the capture endpoints.
//
// It lives here rather than in pkg/transport because it is about this process
// being able to do its job, not about the capture protocol.
func routes(server *transport.Server, controller *bot.Bot) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", server.Routes())
	mux.HandleFunc("/healthz", health(controller))
	return mux
}

// health answers whether AUVC can actually work right now.
//
// A container health check that only proves the process is running is worth
// little: a bot that lost its Discord connection or its database is exactly as
// useless as one that crashed, and only the crash restarts itself. So this
// checks the two things the bot cannot do without.
//
// It says which one failed, because "unhealthy" on its own sends an operator
// reading logs they may not have kept.
func health(controller *bot.Bot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var problems []string

		if controller.PrimarySession == nil || controller.PrimarySession.State == nil ||
			controller.PrimarySession.State.User == nil {
			problems = append(problems, "no Discord connection")
		}
		if _, err := controller.AUVC.SchemaVersion(); err != nil {
			problems = append(problems, "database unreachable")
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if len(problems) > 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, "unhealthy: "+strings.Join(problems, ", "))
			return
		}
		fmt.Fprintln(w, "ok")
	}
}
