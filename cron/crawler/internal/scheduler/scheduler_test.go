package scheduler

import (
	"context"
	"testing"
	"time"

	"muaban-crawler/internal/config"
)

func TestNextRunTime_BeforeWindow(t *testing.T) {
	cfg := &config.Config{CronStartHour: 3, CronEndHour: 5}
	s := New(cfg, func(ctx context.Context) error { return nil })

	// Simulated current time: 01:30 AM today
	now := time.Date(2026, 9, 23, 1, 30, 0, 0, time.UTC)
	next := s.NextRunTime(now, 3, 5)

	if next.Day() != 23 {
		t.Errorf("expected run today (day 23), got day %d", next.Day())
	}
	if next.Hour() < 3 || next.Hour() >= 5 {
		t.Errorf("expected hour between 3 and 5, got %d", next.Hour())
	}
}

func TestNextRunTime_AfterWindow(t *testing.T) {
	cfg := &config.Config{CronStartHour: 3, CronEndHour: 5}
	s := New(cfg, func(ctx context.Context) error { return nil })

	// Simulated current time: 11:30 AM today
	now := time.Date(2026, 9, 23, 11, 30, 0, 0, time.UTC)
	next := s.NextRunTime(now, 3, 5)

	if next.Day() != 24 {
		t.Errorf("expected run tomorrow (day 24), got day %d", next.Day())
	}
	if next.Hour() < 3 || next.Hour() >= 5 {
		t.Errorf("expected hour between 3 and 5, got %d", next.Hour())
	}
}
