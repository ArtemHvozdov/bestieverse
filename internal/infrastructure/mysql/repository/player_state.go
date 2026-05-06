package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

type PlayerStateRepo struct {
	db *sql.DB
}

func NewPlayerStateRepo(db *sql.DB) *PlayerStateRepo {
	return &PlayerStateRepo{db: db}
}

func (r *PlayerStateRepo) Upsert(ctx context.Context, state *entity.PlayerState) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO player_states (game_id, player_id, state, task_id)
		 VALUES (?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE state = VALUES(state), task_id = VALUES(task_id)`,
		state.GameID, state.PlayerID, state.State, state.TaskID,
	)
	if err != nil {
		return fmt.Errorf("mysql/player_state.Upsert: %w", err)
	}
	return nil
}

func (r *PlayerStateRepo) GetByPlayerAndGame(ctx context.Context, gameID, playerID uint64) (*entity.PlayerState, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, game_id, player_id, state, COALESCE(task_id,''), updated_at
		 FROM player_states WHERE game_id = ? AND player_id = ?`,
		gameID, playerID,
	)
	var s entity.PlayerState
	err := row.Scan(&s.ID, &s.GameID, &s.PlayerID, &s.State, &s.TaskID, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mysql/player_state.GetByPlayerAndGame: %w", err)
	}
	return &s, nil
}

func (r *PlayerStateRepo) GetAllAwaitingByGame(ctx context.Context, gameID uint64) ([]*entity.PlayerState, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, game_id, player_id, state, COALESCE(task_id,''), updated_at
		 FROM player_states WHERE game_id = ? AND state = 'awaiting_answer'`,
		gameID,
	)
	if err != nil {
		return nil, fmt.Errorf("mysql/player_state.GetAllAwaitingByGame: %w", err)
	}
	defer rows.Close()

	var states []*entity.PlayerState
	for rows.Next() {
		var s entity.PlayerState
		if err := rows.Scan(&s.ID, &s.GameID, &s.PlayerID, &s.State, &s.TaskID, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("mysql/player_state.GetAllAwaitingByGame scan: %w", err)
		}
		states = append(states, &s)
	}
	return states, rows.Err()
}

func (r *PlayerStateRepo) SetIdle(ctx context.Context, gameID, playerID uint64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE player_states SET state = 'idle', task_id = NULL
		 WHERE game_id = ? AND player_id = ?`,
		gameID, playerID,
	)
	if err != nil {
		return fmt.Errorf("mysql/player_state.SetIdle: %w", err)
	}
	return nil
}
