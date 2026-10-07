// Standalone public-data relay. Deploy behind an HTTPS reverse proxy; it does not
// load the CYLedger server, database, user sessions, or private accounting services.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/marketrelay"
)

func main() {
	relay, err := marketrelay.New(marketrelay.Config{Token: os.Getenv("MARKET_RELAY_TOKEN"), CoinGeckoAPIKey: os.Getenv("CYLEDGER_COINGECKO_DEMO_API_KEY")})
	if err != nil {
		log.Fatal(err)
	}
	address := os.Getenv("MARKET_RELAY_LISTEN")
	if address == "" {
		address = "127.0.0.1:8787"
	}
	server := &http.Server{Addr: address, Handler: relay, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 12 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 8192}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		work, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(work)
	}()
	log.Print("public market relay started; configure HTTPS at the reverse proxy")
	if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("market relay stopped")
	}
}
