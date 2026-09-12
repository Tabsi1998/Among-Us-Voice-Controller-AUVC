package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/pairing"
	"github.com/Tabsi1998/Among-Us-Voice-Controller-AUVC/bot/pkg/transport"
)

// startCaptureListener brings up the direct capture connection, if one is
// configured. It returns a function that shuts the listener down, which is a
// no-op when no listener was started.
//
// The listener is off unless AUVC_CAPTURE_ADDR names an address. A bot that is
// still driven by the legacy transport must not start opening ports nobody
// asked for.
func startCaptureListener(pairingService *pairing.Service, handler transport.Handler) func() {
	address := os.Getenv("AUVC_CAPTURE_ADDR")
	if address == "" {
		log.Println("AUVC_CAPTURE_ADDR is not set; the direct capture connection is disabled")
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

	server := transport.NewServer(pairingService, handler, nil)

	httpServer := &http.Server{
		Addr:    address,
		Handler: server.Routes(),
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
