package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

type NotificationRepo struct {
	db *sql.DB
}

func NewNotificationRepo(db *sql.DB) *NotificationRepo {
	return &NotificationRepo{db: db}
}

func (r *NotificationRepo) Create(ctx context.Context, log *entity.NotificationLog) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO notifications_log (game_id, player_id, task_id) VALUES (?, ?, ?)`,
		log.GameID, log.PlayerID, log.TaskID,
	)
	if err != nil {
		return fmt.Errorf("mysql/notification.Create: %w", err)
	}
	return nil
}

func (r *NotificationRepo) Exists(ctx context.Context, gameID, playerID uint64, taskID string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notifications_log WHERE game_id = ? AND player_id = ? AND task_id = ?`,
		gameID, playerID, taskID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("mysql/notification.Exists: %w", err)
	}
	return count > 0, nil
}

// GetUnnotifiedPlayers returns players who have not yet answered the task and
// have not received a notification for it.
func (r *NotificationRepo) GetUnnotifiedPlayers(ctx context.Context, gameID uint64, taskID string) ([]*entity.Player, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT p.id, p.game_id, p.telegram_user_id, COALESCE(p.username,''), COALESCE(p.first_name,''), p.skip_count, p.joined_at
		 FROM players p
		 LEFT JOIN notifications_log nl
		   ON nl.game_id = p.game_id AND nl.player_id = p.id AND nl.task_id = ?
		 LEFT JOIN task_responses tr
		   ON tr.game_id = p.game_id AND tr.player_id = p.id AND tr.task_id = ?
		 WHERE p.game_id = ?
		   AND nl.id IS NULL
		   AND tr.id IS NULL`,
		taskID, taskID, gameID,
	)
	if err != nil {
		return nil, fmt.Errorf("mysql/notification.GetUnnotifiedPlayers: %w", err)
	}
	defer rows.Close()

	var players []*entity.Player
	for rows.Next() {
		var p entity.Player
		if err := rows.Scan(&p.ID, &p.GameID, &p.TelegramUserID, &p.Username, &p.FirstName, &p.SkipCount, &p.JoinedAt); err != nil {
			return nil, fmt.Errorf("mysql/notification.GetUnnotifiedPlayers scan: %w", err)
		}
		players = append(players, &p)
	}
	return players, rows.Err()
}
