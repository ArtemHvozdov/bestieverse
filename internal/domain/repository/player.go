package repository

import (
	"context"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

type PlayerRepository interface {
	Create(ctx context.Context, player *entity.Player) (*entity.Player, error)
	GetByGameAndTelegramID(ctx context.Context, gameID uint64, telegramUserID int64) (*entity.Player, error)
	GetAllByGame(ctx context.Context, gameID uint64) ([]*entity.Player, error)
	IncrementSkipCount(ctx context.Context, playerID uint64) error
	Delete(ctx context.Context, playerID uint64) error
}
