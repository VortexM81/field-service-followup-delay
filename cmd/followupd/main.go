package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"example.com/field-service-followup/internal/followup"
)

func main() {
	key := mustEnv("INFRAI_API_KEY")
	publicURL := mustEnv("PUBLIC_URL")
	listenAddr := envOr("LISTEN_ADDR", ":8080")
	statePath := envOr("STATE_PATH", "./data/followups.json")

	store, err := followup.OpenStore(statePath)
	if err != nil {
		log.Fatal(err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	service := followup.NewService(followup.NewInfraiClient(key), store, publicURL, logger)
	server := &http.Server{
		Addr:              listenAddr,
		Handler:           service.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	logger.Info("follow-up service listening", "address", listenAddr)
	log.Fatal(server.ListenAndServe())
}

func mustEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("%s is required", name)
	}
	return value
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
