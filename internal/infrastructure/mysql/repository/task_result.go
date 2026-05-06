package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

type TaskResultRepo struct {
	db *sql.DB
}

func NewTaskResultRepo(db *sql.DB) *TaskResultRepo {
	return &TaskResultRepo{db: db}
}

func (r *TaskResultRepo) Create(ctx context.Context, result *entity.TaskResult) error {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO task_results (game_id, task_id, result_data) VALUES (?, ?, ?)`,
		result.GameID, result.TaskID, result.ResultData,
	)
	if err != nil {
		return fmt.Errorf("mysql/task_result.Create: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("mysql/task_result.Create: last insert id: %w", err)
	}
	result.ID = uint64(id)
	return nil
}

func (r *TaskResultRepo) GetByTask(ctx context.Context, gameID uint64, taskID string) (*entity.TaskResult, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, game_id, task_id, result_data, finalized_at
		 FROM task_results WHERE game_id = ? AND task_id = ?`,
		gameID, taskID,
	)
	var result entity.TaskResult
	err := row.Scan(&result.ID, &result.GameID, &result.TaskID, &result.ResultData, &result.FinalizedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mysql/task_result.GetByTask: %w", err)
	}
	return &result, nil
}
