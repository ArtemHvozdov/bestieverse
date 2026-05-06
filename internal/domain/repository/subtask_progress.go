package repository

import (
	"context"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

type SubtaskProgressRepository interface {
	Upsert(ctx context.Context, progress *entity.SubtaskProgress) error
	Get(ctx context.Context, gameID, playerID uint64, taskID string) (*entity.SubtaskProgress, error)
	Delete(ctx context.Context, gameID, playerID uint64, taskID string) error
}
