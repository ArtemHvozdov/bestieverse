package repository

import (
	"context"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

type TaskLockRepository interface {
	Acquire(ctx context.Context, gameID uint64, taskID string, playerID uint64, expiresAt time.Time) error
	Get(ctx context.Context, gameID uint64, taskID string) (*entity.TaskLock, error)
	Release(ctx context.Context, gameID uint64, taskID string) error
	ReleaseExpired(ctx context.Context) error
}
