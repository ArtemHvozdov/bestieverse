package repository

import (
	"context"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

type PlayerStateRepository interface {
	Upsert(ctx context.Context, state *entity.PlayerState) error
	GetByPlayerAndGame(ctx context.Context, gameID, playerID uint64) (*entity.PlayerState, error)
	GetAllAwaitingByGame(ctx context.Context, gameID uint64) ([]*entity.PlayerState, error)
	SetIdle(ctx context.Context, gameID, playerID uint64) error
}
