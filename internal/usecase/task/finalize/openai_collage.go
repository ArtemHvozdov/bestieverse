package finalize

import (
	"context"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
)

// OpenAICollageFinalizer handles summary.type == "openai_collage".
// Full implementation in Stage 11.
type OpenAICollageFinalizer struct {
	taskResultRepo repository.TaskResultRepository
	sender         Sender
}

func NewOpenAICollageFinalizer(taskResultRepo repository.TaskResultRepository, sender Sender) *OpenAICollageFinalizer {
	return &OpenAICollageFinalizer{taskResultRepo: taskResultRepo, sender: sender}
}

func (f *OpenAICollageFinalizer) SupportedSummaryType() string { return SummaryTypeOpenAICollage }

func (f *OpenAICollageFinalizer) Finalize(
	_ context.Context,
	_ *entity.Game,
	_ *config.Task,
	_ []*entity.TaskResponse,
) error {
	return nil
}
