package finalize

import (
	"context"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
)

// WhoIsWhoFinalizer handles summary.type == "who_is_who_results".
// Full implementation in Stage 8.
type WhoIsWhoFinalizer struct {
	taskResultRepo repository.TaskResultRepository
	sender         Sender
}

func NewWhoIsWhoFinalizer(taskResultRepo repository.TaskResultRepository, sender Sender) *WhoIsWhoFinalizer {
	return &WhoIsWhoFinalizer{taskResultRepo: taskResultRepo, sender: sender}
}

func (f *WhoIsWhoFinalizer) SupportedSummaryType() string { return SummaryTypeWhoIsWho }

func (f *WhoIsWhoFinalizer) Finalize(
	_ context.Context,
	_ *entity.Game,
	_ *config.Task,
	_ []*entity.TaskResponse,
) error {
	return nil
}
