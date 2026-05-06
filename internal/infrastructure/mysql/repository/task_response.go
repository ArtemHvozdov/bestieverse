package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

type TaskResponseRepo struct {
	db *sql.DB
}

func NewTaskResponseRepo(db *sql.DB) *TaskResponseRepo {
	return &TaskResponseRepo{db: db}
}

func (r *TaskResponseRepo) Create(ctx context.Context, resp *entity.TaskResponse) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO task_responses (game_id, player_id, task_id, status, response_data)
		 VALUES (?, ?, ?, ?, ?)`,
		resp.GameID, resp.PlayerID, resp.TaskID, resp.Status, nullJSON(resp.ResponseData),
	)
	if err != nil {
		return fmt.Errorf("mysql/task_response.Create: %w", err)
	}
	return nil
}

func (r *TaskResponseRepo) GetByPlayerAndTask(ctx context.Context, gameID, playerID uint64, taskID string) (*entity.TaskResponse, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, game_id, player_id, task_id, status, response_data, created_at
		 FROM task_responses WHERE game_id = ? AND player_id = ? AND task_id = ?`,
		gameID, playerID, taskID,
	)
	var resp entity.TaskResponse
	err := row.Scan(&resp.ID, &resp.GameID, &resp.PlayerID, &resp.TaskID, &resp.Status, &resp.ResponseData, &resp.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mysql/task_response.GetByPlayerAndTask: %w", err)
	}
	return &resp, nil
}

func (r *TaskResponseRepo) GetAllByTask(ctx context.Context, gameID uint64, taskID string) ([]*entity.TaskResponse, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, game_id, player_id, task_id, status, response_data, created_at
		 FROM task_responses WHERE game_id = ? AND task_id = ? AND status = 'answered'
		 ORDER BY created_at`,
		gameID, taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("mysql/task_response.GetAllByTask: %w", err)
	}
	defer rows.Close()

	var responses []*entity.TaskResponse
	for rows.Next() {
		var resp entity.TaskResponse
		if err := rows.Scan(&resp.ID, &resp.GameID, &resp.PlayerID, &resp.TaskID, &resp.Status, &resp.ResponseData, &resp.CreatedAt); err != nil {
			return nil, fmt.Errorf("mysql/task_response.GetAllByTask scan: %w", err)
		}
		responses = append(responses, &resp)
	}
	return responses, rows.Err()
}

func (r *TaskResponseRepo) CountAnsweredByTask(ctx context.Context, gameID uint64, taskID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM task_responses WHERE game_id = ? AND task_id = ? AND status = 'answered'`,
		gameID, taskID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("mysql/task_response.CountAnsweredByTask: %w", err)
	}
	return count, nil
}
