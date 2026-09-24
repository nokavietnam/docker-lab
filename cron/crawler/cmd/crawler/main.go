package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"muaban-crawler/internal/config"
	"muaban-crawler/internal/crawler"
	"muaban-crawler/internal/scheduler"
	"muaban-crawler/internal/storage"
)

func main() {
	log.Println("[Main] Initializing Muaban.net High-Performance Crawler...")

	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown on SIGINT / SIGTERM
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("[Main] Received signal %v. Initiating graceful shutdown...", sig)
		cancel()
	}()

	// Initialize Storage (PostgreSQL)
	log.Printf("[Main] Connecting to PostgreSQL at %s", cfg.DatabaseURL)
	st, err := storage.NewStorage(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[Main] Failed to initialize PostgreSQL storage: %v", err)
	}
	defer st.Close()

	// Ensure DB Schema is applied
	if err := st.InitSchema(ctx); err != nil {
		log.Fatalf("[Main] Failed to initialize database schema: %v", err)
	}
	log.Println("[Main] Database schema verified and ready.")

	// Initialize HTTP Client and Pipeline Coordinator
	client := crawler.NewHTTPClient(cfg)
	coordinator := crawler.NewCoordinator(cfg, client, st)

	// Start Scheduler (Daily random execution between 3 AM - 5 AM or immediate single run)
	sched := scheduler.New(cfg, coordinator.Run)
	if err := sched.Start(ctx); err != nil {
		log.Printf("[Main] Scheduler ended with error: %v", err)
	} else {
		log.Println("[Main] Crawler service finished.")
	}
}
