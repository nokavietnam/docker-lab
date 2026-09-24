package scheduler

import (
	"context"
	"log"
	"math/rand"
	"time"

	"muaban-crawler/internal/config"
)

type Scheduler struct {
	cfg       *config.Config
	crawlFunc func(ctx context.Context) error
	randGen   *rand.Rand
}

func New(cfg *config.Config, crawlFunc func(ctx context.Context) error) *Scheduler {
	return &Scheduler{
		cfg:       cfg,
		crawlFunc: crawlFunc,
		randGen:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NextRunTime calculates a randomized time between startHour:00 and endHour:00
func (s *Scheduler) NextRunTime(now time.Time, startHour, endHour int) time.Time {
	loc := now.Location()
	windowStartToday := time.Date(now.Year(), now.Month(), now.Day(), startHour, 0, 0, 0, loc)
	windowEndToday := time.Date(now.Year(), now.Month(), now.Day(), endHour, 0, 0, 0, loc)

	var targetDate time.Time
	if now.Before(windowEndToday) && now.Before(windowStartToday) {
		// Today's window has not started yet
		targetDate = windowStartToday
	} else {
		// Today's window is ongoing or has passed -> schedule for tomorrow
		targetDate = windowStartToday.AddDate(0, 0, 1)
	}

	windowDurationSec := int64((endHour - startHour) * 3600)
	if windowDurationSec <= 0 {
		windowDurationSec = 3600 // fallback to 1 hour
	}

	randomOffsetSec := s.randGen.Int63n(windowDurationSec)
	return targetDate.Add(time.Duration(randomOffsetSec) * time.Second)
}

// Start runs the scheduler loop, triggering the crawler daily at a randomized time between 3 AM and 5 AM
func (s *Scheduler) Start(ctx context.Context) error {
	if s.cfg.RunMode == "once" {
		log.Println("[Scheduler] RUN_MODE='once'. Executing single crawl run...")
		return s.crawlFunc(ctx)
	}

	log.Printf("[Scheduler] Starting Daily Random Cron Daemon (Window: %02d:00 - %02d:00)...",
		s.cfg.CronStartHour, s.cfg.CronEndHour)

	// Execute initial crawl on boot if desired
	log.Println("[Scheduler] Executing initial crawl on startup...")
	if err := s.crawlFunc(ctx); err != nil {
		log.Printf("[Scheduler] Warning: initial crawl ended with: %v", err)
	}

	for {
		now := time.Now()
		nextRun := s.NextRunTime(now, s.cfg.CronStartHour, s.cfg.CronEndHour)
		waitDuration := time.Until(nextRun)

		log.Printf("[Scheduler] Next crawl scheduled for: %s (in %s)",
			nextRun.Format("2006-01-02 15:04:05 MST"), waitDuration.Round(time.Second))

		timer := time.NewTimer(waitDuration)

		select {
		case <-ctx.Done():
			timer.Stop()
			log.Println("[Scheduler] Shutting down scheduler gracefully...")
			return nil
		case <-timer.C:
			log.Println("[Scheduler] Triggering scheduled daily crawl...")
			if err := s.crawlFunc(ctx); err != nil {
				log.Printf("[Scheduler] Crawl error: %v", err)
			} else {
				log.Println("[Scheduler] Daily crawl completed successfully.")
			}
		}
	}
}
