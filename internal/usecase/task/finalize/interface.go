package finalize

import (
	"context"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

// SummaryType constants match the summary.type values in task YAML files.
const (
	SummaryTypeText          = "text"
	SummaryTypePredictions   = "predictions"
	SummaryTypeWhoIsWho      = "who_is_who_results"
	SummaryTypeCollage       = "collage"
	SummaryTypeOpenAICollage = "openai_collage"
)

// TaskFinalizer is the strategy interface for finalizing a specific summary type.
// Each implementation is responsible for sending the result message and saving task_result.
type TaskFinalizer interface {
	Finalize(
		ctx context.Context,
		game *entity.Game,
		task *config.Task,
		responses []*entity.TaskResponse,
	) error

	SupportedSummaryType() string
}
