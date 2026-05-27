package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/KangVin/TeleRelayBot/internal/app"
	"github.com/KangVin/TeleRelayBot/internal/domain"
)

const (
	EventMessage = domain.RateEventTypeMessage
	EventLimited = domain.RateEventTypeLimited
)

type Store interface {
	AddRateEvent(ctx context.Context, telegramID int64, eventType domain.RateEventType) error
	CountRateEventsSince(ctx context.Context, telegramID int64, eventType domain.RateEventType, since time.Time) (int64, error)
	IncrementLimitedCount(ctx context.Context, telegramID int64) error
}

type Window struct {
	Name     string
	Duration time.Duration
	Max      int
}

type Result struct {
	Allowed       bool
	Exceeded      *Window
	Counts        map[string]int
	LimitedLogged bool
}

type Limiter struct {
	store   Store
	windows []Window
	now     func() time.Time
}

func New(cfg app.Config, rawStore any) (*Limiter, error) {
	store, ok := normalizeStore(rawStore)
	if !ok {
		return nil, errors.New("rate limit store does not implement ratelimit.Store")
	}
	return NewWithClock(store, DefaultWindows(
		cfg.RateLimitShortWindow,
		cfg.RateLimitShortMax,
		cfg.RateLimitMinuteWindow,
		cfg.RateLimitMinuteMax,
		cfg.RateLimitHourWindow,
		cfg.RateLimitHourMax,
	), time.Now)
}

func normalizeStore(rawStore any) (Store, bool) {
	if store, ok := rawStore.(Store); ok {
		return store, true
	}
	value := reflect.ValueOf(rawStore)
	if value.Kind() == reflect.Pointer && !value.IsNil() && value.Elem().Kind() == reflect.Interface {
		if store, ok := value.Elem().Interface().(Store); ok {
			return store, true
		}
	}
	return nil, false
}

func NewWithClock(store Store, windows []Window, now func() time.Time) (*Limiter, error) {
	if store == nil {
		return nil, errors.New("rate limit store is required")
	}
	if len(windows) == 0 {
		return nil, errors.New("at least one rate limit window is required")
	}
	if now == nil {
		return nil, errors.New("clock is required")
	}
	copied := make([]Window, len(windows))
	for i, window := range windows {
		if window.Name == "" {
			return nil, fmt.Errorf("window %d name is required", i)
		}
		if window.Duration <= 0 {
			return nil, fmt.Errorf("window %q duration must be positive", window.Name)
		}
		if window.Max <= 0 {
			return nil, fmt.Errorf("window %q max must be positive", window.Name)
		}
		copied[i] = window
	}
	return &Limiter{store: store, windows: copied, now: now}, nil
}

func DefaultWindows(shortWindow time.Duration, shortMax int, minuteWindow time.Duration, minuteMax int, hourWindow time.Duration, hourMax int) []Window {
	return []Window{
		{Name: "short", Duration: shortWindow, Max: shortMax},
		{Name: "minute", Duration: minuteWindow, Max: minuteMax},
		{Name: "hour", Duration: hourWindow, Max: hourMax},
	}
}

func (l *Limiter) Allow(ctx context.Context, telegramID int64) (bool, error) {
	result, err := l.Check(ctx, telegramID)
	return result.Allowed, err
}

func (l *Limiter) Check(ctx context.Context, telegramID int64) (Result, error) {
	if telegramID <= 0 {
		return Result{}, errors.New("telegramID must be positive")
	}

	now := l.now().UTC()
	if err := l.store.AddRateEvent(ctx, telegramID, EventMessage); err != nil {
		return Result{}, err
	}

	result := Result{
		Allowed: true,
		Counts:  make(map[string]int, len(l.windows)),
	}
	for _, window := range l.windows {
		count, err := l.store.CountRateEventsSince(ctx, telegramID, EventMessage, now.Add(-window.Duration))
		if err != nil {
			return Result{}, err
		}
		result.Counts[window.Name] = int(count)
		if count > int64(window.Max) {
			exceeded := window
			result.Allowed = false
			result.Exceeded = &exceeded
			break
		}
	}

	if result.Allowed {
		return result, nil
	}
	if err := l.store.AddRateEvent(ctx, telegramID, EventLimited); err != nil {
		return Result{}, err
	}
	if err := l.store.IncrementLimitedCount(ctx, telegramID); err != nil {
		return Result{}, err
	}
	result.LimitedLogged = true
	return result, nil
}
