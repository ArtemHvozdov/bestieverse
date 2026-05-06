package repository

import (
	"context"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

type TaskResponseRepository interface {
	Create(ctx context.Context, response *entity.TaskResponse) error
	GetByPlayerAndTask(ctx context.Context, gameID, playerID uint64, taskID string) (*entity.TaskResponse, error)
	GetAllByTask(ctx context.Context, gameID uint64, taskID string) ([]*entity.TaskResponse, error)
	CountAnsweredByTask(ctx context.Context, gameID uint64, taskID string) (int, error)
}
