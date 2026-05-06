package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

type TaskLockRepo struct {
	db *sql.DB
}

func NewTaskLockRepo(db *sql.DB) *TaskLockRepo {
	return &TaskLockRepo{db: db}
}

// Acquire tries to insert a lock row using INSERT IGNORE so that only the first caller wins.
func (r *TaskLockRepo) Acquire(ctx context.Context, gameID uint64, taskID string, playerID uint64, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT IGNORE INTO task_locks (game_id, task_id, player_id, expires_at)
		 VALUES (?, ?, ?, ?)`,
		gameID, taskID, playerID, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("mysql/task_lock.Acquire: %w", err)
	}
	return nil
}

func (r *TaskLockRepo) Get(ctx context.Context, gameID uint64, taskID string) (*entity.TaskLock, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, game_id, task_id, player_id, acquired_at, expires_at
		 FROM task_locks WHERE game_id = ? AND task_id = ?`,
		gameID, taskID,
	)
	var l entity.TaskLock
	err := row.Scan(&l.ID, &l.GameID, &l.TaskID, &l.PlayerID, &l.AcquiredAt, &l.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mysql/task_lock.Get: %w", err)
	}
	return &l, nil
}

func (r *TaskLockRepo) Release(ctx context.Context, gameID uint64, taskID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM task_locks WHERE game_id = ? AND task_id = ?`, gameID, taskID,
	)
	if err != nil {
		return fmt.Errorf("mysql/task_lock.Release: %w", err)
	}
	return nil
}

func (r *TaskLockRepo) ReleaseExpired(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM task_locks WHERE expires_at < NOW()`)
	if err != nil {
		return fmt.Errorf("mysql/task_lock.ReleaseExpired: %w", err)
	}
	return nil
}
