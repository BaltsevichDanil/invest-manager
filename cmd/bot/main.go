package main

import (
	"context"
	"flag"
	"invest-manager/internal/analysis"
	"invest-manager/internal/config"
	"invest-manager/internal/invest"
	"invest-manager/internal/news"
	"invest-manager/internal/scheduler"
	"invest-manager/internal/telegram"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Parse command line flags
	runOnce := flag.Bool("run-once", false, "Run analysis once and exit")
	monthlyReminder := flag.Bool("monthly", false, "Include monthly reminder (only with -run-once)")
	flag.Parse()

	// Initialize logger
	logger := log.New(os.Stdout, "[INVEST-BOT] ", log.LstdFlags)
	logger.Println("Starting Invest Manager Bot")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	// Set up context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize components
	investClient, err := invest.NewClient(cfg, logger)
	if err != nil {
		logger.Fatalf("Failed to initialize Tinkoff Invest client: %v", err)
	}
	defer investClient.Close()

	newsFetcher := news.NewFetcher(cfg)
	analyzer := analysis.NewAnalyzer(cfg)

	telegramBot, err := telegram.NewBot(cfg, logger)
	if err != nil {
		logger.Fatalf("Failed to initialize Telegram bot: %v", err)
	}

	// Run analysis once if requested
	if *runOnce {
		logger.Println("Running one-time analysis")
		
		// Set up scheduler for one-time run
		sched := scheduler.NewScheduler(cfg, logger, investClient, newsFetcher, analyzer, telegramBot)
		
		// Run portfolio analysis
		if err := sched.RunNow(*monthlyReminder); err != nil {
			logger.Fatalf("Error running portfolio analysis: %v", err)
		}
		
		logger.Println("One-time analysis completed, exiting")
		return
	}

	// Initialize scheduler
	sched := scheduler.NewScheduler(cfg, logger, investClient, newsFetcher, analyzer, telegramBot)
	if err := sched.Start(); err != nil {
		logger.Fatalf("Failed to start scheduler: %v", err)
	}
	defer sched.Stop()

	// Send startup notification
	if err := telegramBot.SendMessage("🤖 Invest Manager Bot started successfully."); err != nil {
		logger.Printf("Failed to send startup notification: %v", err)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until signal received
	sig := <-sigChan
	logger.Printf("Received signal %v, shutting down...", sig)

	// Give services time to clean up
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()

	// Wait for shutdown to complete or timeout
	select {
	case <-shutdownCtx.Done():
		if shutdownCtx.Err() == context.DeadlineExceeded {
			logger.Println("Shutdown timed out, forcing exit")
		}
	case <-time.After(time.Second):
		// Add a brief delay to allow logging to finish
	}

	logger.Println("Invest Manager Bot stopped")
} 