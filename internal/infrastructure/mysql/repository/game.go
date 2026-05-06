package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
)

type GameRepo struct {
	db *sql.DB
}

func NewGameRepo(db *sql.DB) *GameRepo {
	return &GameRepo{db: db}
}

func (r *GameRepo) Create(ctx context.Context, game *entity.Game) (*entity.Game, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO games (chat_id, chat_name, admin_user_id, admin_username, status)
		 VALUES (?, ?, ?, ?, ?)`,
		game.ChatID, game.ChatName, game.AdminUserID, game.AdminUsername, game.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("mysql/game.Create: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("mysql/game.Create: last insert id: %w", err)
	}
	game.ID = uint64(id)
	return game, nil
}

func (r *GameRepo) GetByChatID(ctx context.Context, chatID int64) (*entity.Game, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+gameColumns+` FROM games WHERE chat_id = ?`, chatID)
	return scanGame(row)
}

func (r *GameRepo) GetByID(ctx context.Context, id uint64) (*entity.Game, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+gameColumns+` FROM games WHERE id = ?`, id)
	return scanGame(row)
}

func (r *GameRepo) GetByActivePollID(ctx context.Context, pollID string) (*entity.Game, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+gameColumns+` FROM games WHERE active_poll_id = ?`, pollID)
	return scanGame(row)
}

func (r *GameRepo) UpdateStatus(ctx context.Context, id uint64, status entity.GameStatus) error {
	var extra string
	if status == entity.GameActive {
		extra = ", started_at = NOW()"
	} else if status == entity.GameFinished {
		extra = ", finished_at = NOW()"
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE games SET status = ?`+extra+` WHERE id = ?`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("mysql/game.UpdateStatus: %w", err)
	}
	return nil
}

func (r *GameRepo) UpdateCurrentTask(ctx context.Context, id uint64, order int, publishedAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE games SET current_task_order = ?, current_task_published_at = ? WHERE id = ?`,
		order, publishedAt, id,
	)
	if err != nil {
		return fmt.Errorf("mysql/game.UpdateCurrentTask: %w", err)
	}
	return nil
}

func (r *GameRepo) SetActivePollID(ctx context.Context, id uint64, pollID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE games SET active_poll_id = ? WHERE id = ?`,
		sql.NullString{String: pollID, Valid: pollID != ""}, id,
	)
	if err != nil {
		return fmt.Errorf("mysql/game.SetActivePollID: %w", err)
	}
	return nil
}

func (r *GameRepo) GetAllActive(ctx context.Context) ([]*entity.Game, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+gameColumns+` FROM games WHERE status = 'active'`,
	)
	if err != nil {
		return nil, fmt.Errorf("mysql/game.GetAllActive: %w", err)
	}
	defer rows.Close()
	return scanGames(rows)
}

func (r *GameRepo) SetFinished(ctx context.Context, id uint64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE games SET status = 'finished', finished_at = NOW() WHERE id = ?`, id,
	)
	if err != nil {
		return fmt.Errorf("mysql/game.SetFinished: %w", err)
	}
	return nil
}

const gameColumns = `id, chat_id, chat_name, admin_user_id, admin_username, status,
	current_task_order, current_task_published_at, COALESCE(active_poll_id, '') as active_poll_id,
	created_at, started_at, finished_at`

func scanGame(row *sql.Row) (*entity.Game, error) {
	var g entity.Game
	err := row.Scan(
		&g.ID, &g.ChatID, &g.ChatName, &g.AdminUserID, &g.AdminUsername, &g.Status,
		&g.CurrentTaskOrder, &g.CurrentTaskPublishedAt, &g.ActivePollID,
		&g.CreatedAt, &g.StartedAt, &g.FinishedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("mysql/game.scan: %w", err)
	}
	return &g, nil
}

func scanGames(rows *sql.Rows) ([]*entity.Game, error) {
	var games []*entity.Game
	for rows.Next() {
		var g entity.Game
		err := rows.Scan(
			&g.ID, &g.ChatID, &g.ChatName, &g.AdminUserID, &g.AdminUsername, &g.Status,
			&g.CurrentTaskOrder, &g.CurrentTaskPublishedAt, &g.ActivePollID,
			&g.CreatedAt, &g.StartedAt, &g.FinishedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("mysql/game.scan rows: %w", err)
		}
		games = append(games, &g)
	}
	return games, rows.Err()
}
