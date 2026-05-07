package finalize

import (
	"context"
	"fmt"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/rs/zerolog"
)

// OpenAICollageFinalizer handles summary.type == "openai_collage" (task_12).
// The collage and task_result are created by the admin_only subtask handler at answer time.
// This finalizer only verifies the result exists before the router calls finishGame.
type OpenAICollageFinalizer struct {
	taskResultRepo repository.TaskResultRepository
	sender         Sender
	log            zerolog.Logger
}

func NewOpenAICollageFinalizer(
	taskResultRepo repository.TaskResultRepository,
	sender Sender,
	log zerolog.Logger,
) *OpenAICollageFinalizer {
	return &OpenAICollageFinalizer{
		taskResultRepo: taskResultRepo,
		sender:         sender,
		log:            log,
	}
}

func (f *OpenAICollageFinalizer) SupportedSummaryType() string { return SummaryTypeOpenAICollage }

// Finalize verifies that the admin_only handler has already generated and saved the collage.
// Final game messages are sent by FinalizeRouter.finishGame after this returns nil.
func (f *OpenAICollageFinalizer) Finalize(
	ctx context.Context,
	game *entity.Game,
	task *config.Task,
	_ []*entity.TaskResponse,
) error {
	result, err := f.taskResultRepo.GetByTask(ctx, game.ID, task.ID)
	if err != nil {
		return fmt.Errorf("openai_collage.Finalize: get task result: %w", err)
	}
	if result == nil {
		f.log.Error().
			Int64("chat", game.ChatID).
			Uint64("game", game.ID).
			Str("task", task.ID).
			Msg("openai_collage: task_result not found — admin_only handler may not have completed")
		return fmt.Errorf("openai_collage.Finalize: task_result missing for task %s", task.ID)
	}

	f.log.Info().
		Int64("chat", game.ChatID).
		Uint64("game", game.ID).
		Str("task", task.ID).
		Msg("openai_collage: collage verified, ready for game finish")

	return nil
}
