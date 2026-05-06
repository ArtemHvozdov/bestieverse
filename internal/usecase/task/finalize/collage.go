package finalize

import (
	"context"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
)

// CollageFinalizer handles summary.type == "collage".
// Full implementation in Stage 7.
type CollageFinalizer struct {
	taskResultRepo repository.TaskResultRepository
	sender         Sender
}

func NewCollageFinalizer(taskResultRepo repository.TaskResultRepository, sender Sender) *CollageFinalizer {
	return &CollageFinalizer{taskResultRepo: taskResultRepo, sender: sender}
}

func (f *CollageFinalizer) SupportedSummaryType() string { return SummaryTypeCollage }

func (f *CollageFinalizer) Finalize(
	_ context.Context,
	_ *entity.Game,
	_ *config.Task,
	_ []*entity.TaskResponse,
) error {
	return nil
}
