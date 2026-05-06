package repository

import (
	"context"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

type NotificationRepository interface {
	Create(ctx context.Context, log *entity.NotificationLog) error
	Exists(ctx context.Context, gameID, playerID uint64, taskID string) (bool, error)
	GetUnnotifiedPlayers(ctx context.Context, gameID uint64, taskID string) ([]*entity.Player, error)
}
