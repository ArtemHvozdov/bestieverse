package main

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/media"
	mysqldb "github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/mysql"
	mysqlrepo "github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/mysql/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/telegram"
	taskuc "github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task/finalize"
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
	taskResponseRepo := mysqlrepo.NewTaskResponseRepo(db)
	taskResultRepo := mysqlrepo.NewTaskResultRepo(db)
	playerRepo := mysqlrepo.NewPlayerRepo(db)

	bot, err := telegram.NewBot(cfg.Bot.Token, tele.Settings{
		Poller: &tele.LongPoller{Timeout: 10},
	})
	if err != nil {
		log.Fatal().Err(err).Msg("bot init")
	}

	mediaStorage := media.NewLocalStorage(cfg.Media.Path)
	publisher := taskuc.NewPublisher(gameRepo, mediaStorage, bot, cfg, log)

	finalizeRouter := finalize.NewFinalizeRouter(
		taskResponseRepo,
		gameRepo,
		bot,
		mediaStorage,
		cfg,
		log,
		finalize.NewTextFinalizer(bot),
		finalize.NewPredictionsFinalizer(playerRepo, taskResultRepo, bot),
		finalize.NewWhoIsWhoFinalizer(taskResultRepo, bot),
		finalize.NewCollageFinalizer(taskResultRepo, mediaStorage, bot, log),
		finalize.NewOpenAICollageFinalizer(taskResultRepo, bot),
	)

	log.Info().Msg("scheduler started")

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		tick(context.Background(), cfg, gameRepo, publisher, finalizeRouter, log)
	}
}

func tick(
	ctx context.Context,
	cfg *config.Config,
	gameRepo repository.GameRepository,
	publisher *taskuc.Publisher,
	finalizeRouter *finalize.FinalizeRouter,
	log zerolog.Logger,
) {
	games, err := gameRepo.GetAllActive(ctx)
	if err != nil {
		log.Error().Err(err).Msg("scheduler: get active games")
		return
	}

	var wg sync.WaitGroup
	now := time.Now()

	for _, g := range games {
		g := g
		wg.Add(1)
		go func() {
			defer wg.Done()
			processGame(ctx, cfg, g, publisher, finalizeRouter, now, log)
		}()
	}

	wg.Wait()
}

func processGame(
	ctx context.Context,
	cfg *config.Config,
	g *entity.Game,
	publisher *taskuc.Publisher,
	finalizeRouter *finalize.FinalizeRouter,
	now time.Time,
	log zerolog.Logger,
) {
	// Publish the first task immediately after game starts (order 0 = not yet published).
	if g.CurrentTaskOrder == 0 {
		if err := publisher.Publish(ctx, g); err != nil {
			log.Error().Err(err).Uint64("game", g.ID).Msg("scheduler: publish first task")
		}
		return
	}

	if g.CurrentTaskPublishedAt == nil {
		return
	}

	// Check if the current task should be finalized.
	finalizeTime := g.CurrentTaskPublishedAt.Add(cfg.Timings.TaskFinalizeOffset)
	if now.After(finalizeTime) {
		task := cfg.TaskByOrder(g.CurrentTaskOrder)
		if task != nil {
			if err := finalizeRouter.Finalize(ctx, g, task); err != nil {
				log.Error().Err(err).Uint64("game", g.ID).Msg("scheduler: finalize task")
			}
		}
	}

	// Publish next task when the publish interval has elapsed.
	publishTime := g.CurrentTaskPublishedAt.Add(cfg.Timings.TaskPublishInterval)
	if now.After(publishTime) {
		if cfg.TaskByOrder(g.CurrentTaskOrder+1) != nil {
			if err := publisher.Publish(ctx, g); err != nil {
				log.Error().Err(err).Uint64("game", g.ID).Msg("scheduler: publish task")
			}
		}
	}
}
