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

	"github.com/chintamani/habit-tracker/internal/httpx"
	"github.com/chintamani/habit-tracker/internal/store"
	"github.com/chintamani/habit-tracker/web"
)

func main() {
	dbPath := envOr("DB_PATH", "./tracker.db")
	addr := envOr("ADDR", ":8080")
	tzName := envOr("TZ", "Local")

	loc, err := time.LoadLocation(tzName)
	if err != nil {
		log.Fatalf("load timezone %q: %v", tzName, err)
	}

	s, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer s.Close()

	handler, err := httpx.NewServer(s, loc, web.Templates(), web.Static())
	if err != nil {
		log.Fatalf("build server: %v", err)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on %s (db=%s tz=%s)", addr, dbPath, loc)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
