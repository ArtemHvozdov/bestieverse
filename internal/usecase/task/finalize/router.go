package finalize

import (
	"context"
	"fmt"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/media"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/logger"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// FinalizeRouter dispatches finalization to the correct TaskFinalizer based on summary.type.
type FinalizeRouter struct {
	finalizers       map[string]TaskFinalizer
	taskResponseRepo repository.TaskResponseRepository
	taskResultRepo   repository.TaskResultRepository
	gameRepo         repository.GameRepository
	sender           Sender
	media            media.Storage
	cfg              *config.Config
	log              zerolog.Logger
}

func NewFinalizeRouter(
	taskResponseRepo repository.TaskResponseRepository,
	taskResultRepo repository.TaskResultRepository,
	gameRepo repository.GameRepository,
	sender Sender,
	mediaStorage media.Storage,
	cfg *config.Config,
	log zerolog.Logger,
	finalizers ...TaskFinalizer,
) *FinalizeRouter {
	r := &FinalizeRouter{
		finalizers:       make(map[string]TaskFinalizer, len(finalizers)),
		taskResponseRepo: taskResponseRepo,
		taskResultRepo:   taskResultRepo,
		gameRepo:         gameRepo,
		sender:           sender,
		media:            mediaStorage,
		cfg:              cfg,
		log:              log,
	}
	for _, f := range finalizers {
		r.finalizers[f.SupportedSummaryType()] = f
	}
	return r
}

// Finalize is the entry point called by the scheduler.
func (r *FinalizeRouter) Finalize(ctx context.Context, game *entity.Game, task *config.Task) error {
	// Idempotency guard: skip if already finalized.
	existing, err := r.taskResultRepo.GetByTask(ctx, game.ID, task.ID)
	if err != nil {
		return fmt.Errorf("finalize.Router: check existing result: %w", err)
	}
	if existing != nil {
		r.log.Debug().
			Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
			Uint64("game", game.ID).
			Str("task", task.ID).
			Msg("task already finalized, skipping")
		return nil
	}

	responses, err := r.taskResponseRepo.GetAllByTask(ctx, game.ID, task.ID)
	if err != nil {
		return fmt.Errorf("finalize.Router: get responses: %w", err)
	}

	chat := &tele.Chat{ID: game.ChatID}

	if len(responses) == 0 {
		text := config.Random(r.cfg.Messages.NaAnswers)
		r.sender.Send(chat, text, formatter.ParseMode) //nolint:errcheck
		r.log.Info().Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).Uint64("game", game.ID).Str("task", task.ID).Msg("task finalized: no answers")
		return nil
	}

	f, ok := r.finalizers[task.Summary.Type]
	if !ok {
		r.log.Error().Str("summary_type", task.Summary.Type).Msg("finalize.Router: unknown summary type")
		return fmt.Errorf("finalize.Router: unknown summary type: %s", task.Summary.Type)
	}

	if err := f.Finalize(ctx, game, task, responses); err != nil {
		return fmt.Errorf("finalize.Router: %w", err)
	}

	r.log.Info().
		Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
		Uint64("game", game.ID).
		Str("task", task.ID).
		Msg("task finalized")

	// Check if this was the last task.
	if r.cfg.TaskByOrder(task.Order+1) == nil {
		return r.finishGame(ctx, game)
	}

	return nil
}
