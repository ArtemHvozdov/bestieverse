package repository

import (
	"context"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

type TaskResultRepository interface {
	Create(ctx context.Context, result *entity.TaskResult) error
	GetByTask(ctx context.Context, gameID uint64, taskID string) (*entity.TaskResult, error)
}
