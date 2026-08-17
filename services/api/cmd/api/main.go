package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nhathao-nguyen/NH-Media/services/api/internal/auth"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/config"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/health"
	"github.com/nhathao-nguyen/NH-Media/services/api/internal/httpapi"
)

func main() {
	value, err := config.LoadAPI(os.Getenv)
	if err != nil {
		log.Fatalf("invalid API configuration: %v", err)
	}
	provider, err := auth.NewLocalAuthProvider(value.AdminUsername, value.AdminPassword)
	if err != nil {
		log.Fatalf("invalid LocalAuth configuration: %v", err)
	}
	server, err := httpapi.NewServer(value, provider, health.NewRegistry(nil, 2*time.Second))
	if err != nil {
		log.Fatalf("invalid API server: %v", err)
	}
	go func() {
		log.Printf("NH-Media API shell listening on %s (profile=%s)", value.Bind, value.Profile)
		if err := server.HTTPServer.ListenAndServe(); err != nil {
			log.Printf("API stopped: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	server.Health.SetDraining(true)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.HTTPServer.Shutdown(ctx); err != nil {
		log.Printf("API drain failed: %v", err)
	}
}
