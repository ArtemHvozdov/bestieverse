package main

import (
	"context"
	"os"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/media"
	mysqldb "github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/mysql"
	mysqlrepo "github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/mysql/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/telegram"
	taskuc "github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/logger"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}

	log := logger.New(cfg.Log.Level, cfg.Log.File)

	db, err := mysqldb.Open(cfg.DB.DSN())
	if err != nil {
		log.Fatal().Err(err).Msg("db connect")
	}
	defer db.Close()

	gameRepo := mysqlrepo.NewGameRepo(db)

	bot, err := telegram.NewBot(cfg.Bot.Token, tele.Settings{
		Poller: &tele.LongPoller{Timeout: 10},
	})
	if err != nil {
		log.Fatal().Err(err).Msg("bot init")
	}

	mediaStorage := media.NewLocalStorage(cfg.Media.Path)
	publisher := taskuc.NewPublisher(gameRepo, mediaStorage, bot, cfg, log)

	log.Info().Msg("scheduler started")

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		tick(context.Background(), cfg, gameRepo, publisher, log)
	}
}

func tick(ctx context.Context, cfg *config.Config, gameRepo repository.GameRepository, publisher *taskuc.Publisher, log zerolog.Logger) {
	games, err := gameRepo.GetAllActive(ctx)
	if err != nil {
		log.Error().Err(err).Msg("scheduler: get active games")
		return
	}

	now := time.Now()
	for _, g := range games {
		// Publish the first task immediately after game starts (order 0 = not yet published).
		if g.CurrentTaskOrder == 0 {
			if err := publisher.Publish(ctx, g); err != nil {
				log.Error().Err(err).Uint64("game", g.ID).Msg("scheduler: publish first task")
			}
			continue
		}

		if g.CurrentTaskPublishedAt == nil {
			continue
		}

		// Publish next task when the publish interval has elapsed.
		if now.After(g.CurrentTaskPublishedAt.Add(cfg.Timings.TaskPublishInterval)) {
			if cfg.TaskByOrder(g.CurrentTaskOrder+1) != nil {
				if err := publisher.Publish(ctx, g); err != nil {
					log.Error().Err(err).Uint64("game", g.ID).Msg("scheduler: publish task")
				}
			}
		}
	}
}
