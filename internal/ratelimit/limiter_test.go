package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/KangVin/TeleRelayBot/internal/domain"
)

func TestLimiterAllowsWithinLimits(t *testing.T) {
	store := newMemoryStore()
	now := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	limiter := mustLimiter(t, store, now, []Window{
		{Name: "short", Duration: 10 * time.Second, Max: 3},
	})

	for i := 0; i < 3; i++ {
		result, err := limiter.Check(context.Background(), 123)
		if err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
		if !result.Allowed {
			t.Fatalf("Allow() denied at iteration %d: %#v", i, result)
		}
	}
	if store.limitedCount[123] != 0 {
		t.Fatalf("limited count = %d", store.limitedCount[123])
	}
}

func TestLimiterRejectsShortWindow(t *testing.T) {
	store := newMemoryStore()
	limiter := mustLimiter(t, store, time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC), []Window{
		{Name: "short", Duration: 10 * time.Second, Max: 3},
		{Name: "minute", Duration: time.Minute, Max: 100},
		{Name: "hour", Duration: time.Hour, Max: 100},
	})

	result := allowNTimes(t, limiter, 123, 4)
	if result.Allowed || result.Exceeded == nil || result.Exceeded.Name != "short" {
		t.Fatalf("expected short limit denial, got %#v", result)
	}
	if store.limitedCount[123] != 1 {
		t.Fatalf("limited count = %d", store.limitedCount[123])
	}
}

func TestLimiterRejectsMinuteWindow(t *testing.T) {
	store := newMemoryStore()
	limiter := mustLimiter(t, store, time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC), []Window{
		{Name: "short", Duration: 10 * time.Second, Max: 100},
		{Name: "minute", Duration: time.Minute, Max: 3},
		{Name: "hour", Duration: time.Hour, Max: 100},
	})

	result := allowNTimes(t, limiter, 123, 4)
	if result.Allowed || result.Exceeded == nil || result.Exceeded.Name != "minute" {
		t.Fatalf("expected minute limit denial, got %#v", result)
	}
}

func TestLimiterRejectsHourWindow(t *testing.T) {
	store := newMemoryStore()
	limiter := mustLimiter(t, store, time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC), []Window{
		{Name: "short", Duration: 10 * time.Second, Max: 100},
		{Name: "minute", Duration: time.Minute, Max: 100},
		{Name: "hour", Duration: time.Hour, Max: 3},
	})

	result := allowNTimes(t, limiter, 123, 4)
	if result.Allowed || result.Exceeded == nil || result.Exceeded.Name != "hour" {
		t.Fatalf("expected hour limit denial, got %#v", result)
	}
}

func TestLimiterIgnoresEventsOutsideWindow(t *testing.T) {
	store := newMemoryStore()
	current := time.Date(2026, 5, 11, 12, 0, 20, 0, time.UTC)
	store.events = append(store.events, rateEvent{telegramID: 123, eventType: EventMessage, occurredAt: current.Add(-11 * time.Second)})
	limiter := mustLimiter(t, store, current, []Window{
		{Name: "short", Duration: 10 * time.Second, Max: 1},
	})

	result, err := limiter.Check(context.Background(), 123)
	if err != nil {
		t.Fatalf("Allow() error = %v", err)
	}
	if !result.Allowed {
		t.Fatalf("old event should be outside short window: %#v", result)
	}
}

func allowNTimes(t *testing.T, limiter *Limiter, telegramID int64, n int) Result {
	t.Helper()
	var result Result
	for i := 0; i < n; i++ {
		var err error
		result, err = limiter.Check(context.Background(), telegramID)
		if err != nil {
			t.Fatalf("Allow() error = %v", err)
		}
	}
	return result
}

func mustLimiter(t *testing.T, store Store, now time.Time, windows []Window) *Limiter {
	t.Helper()
	if memory, ok := store.(*memoryStore); ok {
		memory.now = now
	}
	limiter, err := NewWithClock(store, windows, func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewWithClock() error = %v", err)
	}
	return limiter
}

type rateEvent struct {
	telegramID int64
	eventType  domain.RateEventType
	occurredAt time.Time
}

type memoryStore struct {
	events       []rateEvent
	limitedCount map[int64]int
	now          time.Time
}

func newMemoryStore() *memoryStore {
	return &memoryStore{limitedCount: make(map[int64]int)}
}

func (s *memoryStore) AddRateEvent(_ context.Context, telegramID int64, eventType domain.RateEventType) error {
	s.events = append(s.events, rateEvent{telegramID: telegramID, eventType: eventType, occurredAt: s.now})
	return nil
}

func (s *memoryStore) CountRateEventsSince(_ context.Context, telegramID int64, eventType domain.RateEventType, since time.Time) (int64, error) {
	var count int64
	for _, event := range s.events {
		if event.telegramID == telegramID && event.eventType == eventType && !event.occurredAt.Before(since) {
			count++
		}
	}
	return count, nil
}

func (s *memoryStore) IncrementLimitedCount(_ context.Context, telegramID int64) error {
	s.limitedCount[telegramID]++
	return nil
}
