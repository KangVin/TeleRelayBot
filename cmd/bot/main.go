package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/KangVin/TeleRelayBot/internal/app"
	tgbot "github.com/KangVin/TeleRelayBot/internal/bot"
	applog "github.com/KangVin/TeleRelayBot/internal/log"
	"github.com/KangVin/TeleRelayBot/internal/store"
	tele "gopkg.in/telebot.v4"
)

func main() {
	logger := applog.New()
	if err := run(logger); err != nil {
		logger.Error("bot stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := app.LoadConfig()
	if err != nil {
		return err
	}

	st, err := store.Open(context.Background(), cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := st.Close(); closeErr != nil {
			logger.Error("close store", slog.Any("error", closeErr))
		}
	}()

	if err := st.Migrate(context.Background()); err != nil {
		return err
	}

	pref := tele.Settings{
		Token: cfg.BotToken,
		Poller: &tele.LongPoller{
			Timeout: 10 * time.Second,
		},
	}
	b, err := tele.NewBot(pref)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	relay, err := tgbot.New(b, cfg, st, logger, ctx)
	if err != nil {
		return err
	}
	relay.Register()

	logger.Info("bot starting", slog.String("username", b.Me.Username))

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 30*time.Second)
			if err := st.DeleteRateEventsBefore(cleanupCtx, time.Now().UTC().Add(-time.Duration(cfg.RateEventRetentionDays)*24*time.Hour)); err != nil {
				logger.Error("cleanup rate events", slog.Any("error", err))
			}
			cleanupCancel()
			select {
			case <-ticker.C:
			case <-stop:
				return
			}
		}
	}()

	go b.Start()
	<-stop
	logger.Info("bot stopping")
	b.Stop()
	cancel()

	return nil
}
