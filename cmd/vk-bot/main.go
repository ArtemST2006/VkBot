package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ArtemST2006/VkBot/internal/lib/logger"
	"github.com/ArtemST2006/VkBot/internal/service"
	"github.com/ArtemST2006/VkBot/internal/storage/postgres"
	"github.com/ArtemST2006/VkBot/internal/subscription"
	"github.com/ArtemST2006/VkBot/internal/vkbot"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	log := logger.New(os.Getenv("LOG_LEVEL"), os.Getenv("ENV"))
	if err := run(log); err != nil {
		log.Error("VK bot stopped with error", logger.Err(err))
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	token := os.Getenv("VK_GROUP_TOKEN")
	if token == "" {
		return errors.New("VK_GROUP_TOKEN is required")
	}
	groupID, err := strconv.ParseInt(os.Getenv("VK_GROUP_ID"), 10, 64)
	if err != nil || groupID <= 0 {
		return errors.New("VK_GROUP_ID must be a positive number")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = pool.Ping(connectCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}

	client := vkbot.NewClient(token, groupID, "", nil)
	repo := postgres.New(pool)
	svc := service.New(repo, subscription.NewStubChecker())
	handler := vkbot.NewHandler(svc, client, log)
	go vkbot.RunNotifications(ctx, repo, client, log)
	go service.RunDrawScheduler(ctx, repo, log)

	log.Info("starting VK bot", slog.String("mode", "polling"), slog.Int64("group_id", groupID))
	if err := vkbot.RunPolling(ctx, client, handler); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	log.Info("VK bot stopped")
	return nil
}
