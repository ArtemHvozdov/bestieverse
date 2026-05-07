package repository

import (
	"context"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

type GameRepository interface {
	Create(ctx context.Context, game *entity.Game) (*entity.Game, error)
	GetByChatID(ctx context.Context, chatID int64) (*entity.Game, error)
	GetByID(ctx context.Context, id uint64) (*entity.Game, error)
	GetByActivePollID(ctx context.Context, pollID string) (*entity.Game, error)
	UpdateStatus(ctx context.Context, id uint64, status entity.GameStatus) error
	UpdateCurrentTask(ctx context.Context, id uint64, order int, publishedAt time.Time) error
	SetActivePollID(ctx context.Context, id uint64, pollID string) error
	GetAllActive(ctx context.Context) ([]*entity.Game, error)
	SetFinished(ctx context.Context, id uint64) error
	Delete(ctx context.Context, id uint64) error
}
