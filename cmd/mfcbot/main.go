package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"mfcbot/internal/commands"
	"mfcbot/internal/config"
	"mfcbot/internal/database"
	"mfcbot/internal/discord"
	"mfcbot/internal/logging"
	"mfcbot/internal/mfc"
	"mfcbot/internal/notifier"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println("Starting MFC Discord Bot")

	cfg := config.Load()
	logging.SetVerbosity(cfg.Debug, cfg.Trace)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// MFC adapter: websocket monitor plus /online, /status, /healthz, /version
	adapter := mfc.NewAdapter(cfg)
	if err := adapter.Start(ctx); err != nil {
		log.Fatalf("Failed to start MFC adapter: %v", err)
	}
	defer adapter.Stop()

	// Discord
	dc, err := discord.NewClient(cfg.Token)
	if err != nil {
		log.Fatalf("Failed to create Discord client: %v", err)
	}
	if err := dc.Connect(); err != nil {
		log.Fatalf("Failed to connect to Discord: %v", err)
	}
	defer dc.Disconnect()

	if err := dc.RegisterCommands(); err != nil {
		log.Printf("Failed to register commands: %v", err)
	}

	// Notifications
	notify := notifier.New(db, dc, adapter.GetOnline, adapter.OnlineReady, cfg.OnlineCheckInterval)
	handler := &commands.Handler{
		DB:           db,
		LookupID:     adapter.LookupModelID,
		TriggerCheck: notify.Trigger,
	}
	dc.AddHandler(handler.Handle)

	go notify.Run(ctx)
	log.Println("Bot is running")

	// Wait for SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down MFC Discord Bot")
	cancel()
	adapter.Stop()
}
