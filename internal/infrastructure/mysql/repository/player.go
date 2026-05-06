package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

type PlayerRepo struct {
	db *sql.DB
}

func NewPlayerRepo(db *sql.DB) *PlayerRepo {
	return &PlayerRepo{db: db}
}

func (r *PlayerRepo) Create(ctx context.Context, player *entity.Player) (*entity.Player, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO players (game_id, telegram_user_id, username, first_name)
		 VALUES (?, ?, ?, ?)`,
		player.GameID, player.TelegramUserID, player.Username, player.FirstName,
	)
	if err != nil {
		return nil, fmt.Errorf("mysql/player.Create: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("mysql/player.Create: last insert id: %w", err)
	}
	player.ID = uint64(id)
	return player, nil
}

func (r *PlayerRepo) GetByGameAndTelegramID(ctx context.Context, gameID uint64, telegramUserID int64) (*entity.Player, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, game_id, telegram_user_id, COALESCE(username,''), COALESCE(first_name,''), skip_count, joined_at
		 FROM players WHERE game_id = ? AND telegram_user_id = ?`,
		gameID, telegramUserID,
	)
	return scanPlayer(row)
}

func (r *PlayerRepo) GetAllByGame(ctx context.Context, gameID uint64) ([]*entity.Player, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, game_id, telegram_user_id, COALESCE(username,''), COALESCE(first_name,''), skip_count, joined_at
		 FROM players WHERE game_id = ? ORDER BY joined_at`,
		gameID,
	)
	if err != nil {
		return nil, fmt.Errorf("mysql/player.GetAllByGame: %w", err)
	}
	defer rows.Close()

	var players []*entity.Player
	for rows.Next() {
		var p entity.Player
		if err := rows.Scan(&p.ID, &p.GameID, &p.TelegramUserID, &p.Username, &p.FirstName, &p.SkipCount, &p.JoinedAt); err != nil {
			return nil, fmt.Errorf("mysql/player.GetAllByGame scan: %w", err)
		}
		players = append(players, &p)
	}
	return players, rows.Err()
}

func (r *PlayerRepo) IncrementSkipCount(ctx context.Context, playerID uint64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE players SET skip_count = skip_count + 1 WHERE id = ?`, playerID,
	)
	if err != nil {
		return fmt.Errorf("mysql/player.IncrementSkipCount: %w", err)
	}
	return nil
}

func (r *PlayerRepo) Delete(ctx context.Context, playerID uint64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM players WHERE id = ?`, playerID)
	if err != nil {
		return fmt.Errorf("mysql/player.Delete: %w", err)
	}
	return nil
}

func scanPlayer(row *sql.Row) (*entity.Player, error) {
	var p entity.Player
	err := row.Scan(&p.ID, &p.GameID, &p.TelegramUserID, &p.Username, &p.FirstName, &p.SkipCount, &p.JoinedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mysql/player.scan: %w", err)
	}
	return &p, nil
}
