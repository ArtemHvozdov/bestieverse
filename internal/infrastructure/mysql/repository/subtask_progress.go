package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

type SubtaskProgressRepo struct {
	db *sql.DB
}

func NewSubtaskProgressRepo(db *sql.DB) *SubtaskProgressRepo {
	return &SubtaskProgressRepo{db: db}
}

func (r *SubtaskProgressRepo) Upsert(ctx context.Context, p *entity.SubtaskProgress) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO subtask_progress (game_id, player_id, task_id, question_index, answers_data)
		 VALUES (?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE question_index = VALUES(question_index), answers_data = VALUES(answers_data)`,
		p.GameID, p.PlayerID, p.TaskID, p.QuestionIndex, nullJSON(p.AnswersData),
	)
	if err != nil {
		return fmt.Errorf("mysql/subtask_progress.Upsert: %w", err)
	}
	return nil
}

func (r *SubtaskProgressRepo) Get(ctx context.Context, gameID, playerID uint64, taskID string) (*entity.SubtaskProgress, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, game_id, player_id, task_id, question_index, answers_data, updated_at
		 FROM subtask_progress WHERE game_id = ? AND player_id = ? AND task_id = ?`,
		gameID, playerID, taskID,
	)
	var p entity.SubtaskProgress
	var answersData sql.NullString
	err := row.Scan(&p.ID, &p.GameID, &p.PlayerID, &p.TaskID, &p.QuestionIndex, &answersData, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mysql/subtask_progress.Get: %w", err)
	}
	p.AnswersData = scanNullJSON(answersData)
	return &p, nil
}

func (r *SubtaskProgressRepo) Delete(ctx context.Context, gameID, playerID uint64, taskID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM subtask_progress WHERE game_id = ? AND player_id = ? AND task_id = ?`,
		gameID, playerID, taskID,
	)
	if err != nil {
		return fmt.Errorf("mysql/subtask_progress.Delete: %w", err)
	}
	return nil
}
