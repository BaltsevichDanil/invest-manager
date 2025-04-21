package scheduler

import (
	"context"
	"fmt"
	"invest-manager/internal/analysis"
	"invest-manager/internal/config"
	"invest-manager/internal/invest"
	"invest-manager/internal/news"
	"invest-manager/internal/telegram"
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

// Job contains all dependencies needed for scheduled jobs
type Job struct {
	config    *config.Config
	logger    *log.Logger
	investor  *invest.Client
	newsFetcher *news.Fetcher
	analyzer  *analysis.Analyzer
	telegramBot *telegram.Bot
}

// Scheduler handles scheduling of portfolio analysis tasks
type Scheduler struct {
	job        *Job
	cron       *cron.Cron
	timezone   *time.Location
	logger     *log.Logger
}

// NewScheduler creates a new scheduler
func NewScheduler(
	cfg *config.Config,
	logger *log.Logger,
	investor *invest.Client,
	newsFetcher *news.Fetcher,
	analyzer *analysis.Analyzer,
	telegramBot *telegram.Bot,
) *Scheduler {
	job := &Job{
		config:      cfg,
		logger:      logger,
		investor:    investor,
		newsFetcher: newsFetcher,
		analyzer:    analyzer,
		telegramBot: telegramBot,
	}

	// Create cron scheduler with the specified timezone
	cronOptions := cron.WithLocation(cfg.Timezone)
	cronScheduler := cron.New(cronOptions)
	
	return &Scheduler{
		job:      job,
		cron:     cronScheduler,
		timezone: cfg.Timezone,
		logger:   logger,
	}
}

// Start begins the scheduler
func (s *Scheduler) Start() error {
	// Daily job at 7:00 MSK
	_, err := s.cron.AddFunc("0 7 * * *", func() {
		s.logger.Printf("Running daily portfolio analysis job")
		
		// Check if today is the 5th of the month
		now := time.Now().In(s.timezone)
		isMonthlyReminder := now.Day() == 5
		
		// Run the portfolio analysis
		if err := s.runPortfolioAnalysis(isMonthlyReminder); err != nil {
			s.logger.Printf("Error running portfolio analysis: %v", err)
		}
	})
	
	if err != nil {
		return fmt.Errorf("failed to schedule daily job: %w", err)
	}
	
	// Start the cron scheduler
	s.cron.Start()
	s.logger.Printf("Scheduler started. Timezone: %s", s.timezone.String())
	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	s.logger.Printf("Scheduler stopped")
}

// RunNow runs portfolio analysis immediately
func (s *Scheduler) RunNow(isMonthlyReminder bool) error {
	s.logger.Printf("Running portfolio analysis now (manual trigger)")
	return s.runPortfolioAnalysis(isMonthlyReminder)
}

// runPortfolioAnalysis runs the complete portfolio analysis workflow
func (s *Scheduler) runPortfolioAnalysis(isMonthlyReminder bool) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	// Step 1: Get portfolio data
	s.logger.Printf("Getting portfolio data")
	portfolio, err := s.job.investor.GetPortfolio(ctx)
	if err != nil {
		return fmt.Errorf("failed to get portfolio: %w", err)
	}
	
	// Step 2: Fetch news about Russia
	s.logger.Printf("Fetching fresh news about Russia")
	articles, err := s.job.newsFetcher.FetchNews("Russia", 5)
	if err != nil {
		s.logger.Printf("Warning: failed to fetch news: %v. Continuing without news data", err)
		articles = []news.Article{} // Empty but continue
	}
	
	// Step 3: Analyze portfolio and news
	s.logger.Printf("Analyzing portfolio with OpenAI")
	analysis, err := s.job.analyzer.AnalyzePortfolio(ctx, portfolio, articles, isMonthlyReminder)
	if err != nil {
		return fmt.Errorf("failed to analyze portfolio: %w", err)
	}
	
	// Step 4: Send results to Telegram with fresh news
	s.logger.Printf("Sending analysis to Telegram")
	if err := s.job.telegramBot.SendPortfolioAnalysis(portfolio, analysis, articles); err != nil {
		return fmt.Errorf("failed to send analysis to Telegram: %w", err)
	}
	
	s.logger.Printf("Portfolio analysis completed successfully")
	return nil
} 