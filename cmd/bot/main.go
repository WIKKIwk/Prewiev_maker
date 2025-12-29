package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"pro-banana-ai-bot/internal/config"
	"pro-banana-ai-bot/internal/gemini"
	"pro-banana-ai-bot/internal/handlers"
	"pro-banana-ai-bot/internal/httpclient"
	"pro-banana-ai-bot/internal/mediagroup"
	"pro-banana-ai-bot/internal/session"
	"pro-banana-ai-bot/internal/telegram"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger := newLogger(cfg)

	httpClient := httpclient.New(httpclient.Options{
		PreferIPv4: cfg.PreferIPv4,
		Timeout:    cfg.HTTPTimeout,
	})

	tg, err := telegram.New(telegram.Options{
		Token:      cfg.TelegramToken,
		HTTPClient: httpClient,
		Logger:     logger,
		Debug:      cfg.Debug,
	})
	if err != nil {
		logger.Error("telegram init failed", "err", err)
		os.Exit(1)
	}

	gem := gemini.New(gemini.Options{
		APIKey:     cfg.GeminiAPIKey,
		BaseURL:    cfg.GeminiBaseURL,
		APIVersion: cfg.GeminiAPIVersion,
		HTTPClient: httpClient,
		Logger:     logger,
	})

	sessions := session.NewStore(session.Options{
		MaxMessages: cfg.MaxHistoryMessages,
	})

	handler := handlers.New(handlers.Options{
		Telegram: tg,
		Gemini:   gem,
		Sessions: sessions,
		Logger:   logger,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sem := make(chan struct{}, cfg.MaxConcurrent)
	onGroupFlush := func(group mediagroup.Group) {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return
		}

		go func() {
			defer func() { <-sem }()

			reqCtx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
			defer cancel()

			handler.HandleMediaGroup(reqCtx, group)
		}()
	}

	aggregator := mediagroup.New(mediagroup.Options{
		Debounce: cfg.MediaGroupDebounce,
		OnFlush:  onGroupFlush,
	})
	handler.SetMediaGroupAggregator(aggregator)

	logger.Info("bot started", "username", tg.Username())

	updates := tg.Updates(telegram.UpdatesOptions{
		Timeout: 30 * time.Second,
	})
	defer tg.StopUpdates()

	for {
		select {
		case <-ctx.Done():
			logger.Info("shutting down")
			return
		case update, ok := <-updates:
			if !ok {
				logger.Info("updates channel closed")
				return
			}

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}

			go func(update telegram.Update) {
				defer func() { <-sem }()

				reqCtx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
				defer cancel()

				if err := handler.HandleUpdate(reqCtx, update); err != nil && !errors.Is(err, context.Canceled) {
					logger.Error("handle update failed", "err", err)
				}
			}(update)
		}
	}
}

func newLogger(cfg config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}
