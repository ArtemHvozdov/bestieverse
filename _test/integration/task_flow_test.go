//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	mysqlrepo "github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/mysql/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupActiveGame creates a game with one player in active status for task flow tests.
func setupActiveGame(t *testing.T, chatID int64) (game *entity.Game, player *entity.Player) {
	t.Helper()
	ctx := context.Background()
	gameRepo := mysqlrepo.NewGameRepo(testDB)
	playerRepo := mysqlrepo.NewPlayerRepo(testDB)

	g := &entity.Game{
		ChatID:        chatID,
		ChatName:      "Task Flow Chat",
		AdminUserID:   500001,
		AdminUsername: "task_admin",
		Status:        entity.GameActive,
	}
	created, err := gameRepo.Create(ctx, g)
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	t.Cleanup(func() { cleanupGame(t, testDB, created.ID) })

	p, err := playerRepo.Create(ctx, &entity.Player{
		GameID:         created.ID,
		TelegramUserID: 500001,
		Username:       "task_admin",
		FirstName:      "Task Admin",
	})
	require.NoError(t, err)
	return created, p
}

func TestAnswerTask(t *testing.T) {
	ctx := context.Background()
	game, player := setupActiveGame(t, -1001222222001)

	gameRepo := mysqlrepo.NewGameRepo(testDB)
	taskResponseRepo := mysqlrepo.NewTaskResponseRepo(testDB)

	now := time.Now()
	err := gameRepo.UpdateCurrentTask(ctx, game.ID, 1, now)
	require.NoError(t, err)

	resp := &entity.TaskResponse{
		GameID:   game.ID,
		PlayerID: player.ID,
		TaskID:   "task_01",
		Status:   entity.ResponseAnswered,
	}
	err = taskResponseRepo.Create(ctx, resp)
	require.NoError(t, err)

	// Verify response is stored.
	fetched, err := taskResponseRepo.GetByPlayerAndTask(ctx, game.ID, player.ID, "task_01")
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, entity.ResponseAnswered, fetched.Status)

	// CountAnsweredByTask should return 1.
	count, err := taskResponseRepo.CountAnsweredByTask(ctx, game.ID, "task_01")
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestSkipTask(t *testing.T) {
	ctx := context.Background()
	game, player := setupActiveGame(t, -1001222222002)

	playerRepo := mysqlrepo.NewPlayerRepo(testDB)
	taskResponseRepo := mysqlrepo.NewTaskResponseRepo(testDB)

	// Skip tasks 1, 2, 3 (each increments skip_count).
	for i, taskID := range []string{"task_01", "task_02", "task_03"} {
		err := taskResponseRepo.Create(ctx, &entity.TaskResponse{
			GameID:   game.ID,
			PlayerID: player.ID,
			TaskID:   taskID,
			Status:   entity.ResponseSkipped,
		})
		require.NoError(t, err)
		err = playerRepo.IncrementSkipCount(ctx, player.ID)
		require.NoError(t, err)

		// Verify skip_count after each increment.
		players, err := playerRepo.GetAllByGame(ctx, game.ID)
		require.NoError(t, err)
		require.Len(t, players, 1)
		assert.Equal(t, i+1, players[0].SkipCount, "skip_count after skip %d", i+1)
	}

	// The 4th skip should be blocked by business logic (skip_count == 3 means no more skips).
	// Here we verify the DB state reflects 3 skips.
	players, err := playerRepo.GetAllByGame(ctx, game.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, players[0].SkipCount)

	// Verify all 3 skip responses exist.
	for _, taskID := range []string{"task_01", "task_02", "task_03"} {
		resp, err := taskResponseRepo.GetByPlayerAndTask(ctx, game.ID, player.ID, taskID)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, entity.ResponseSkipped, resp.Status)
	}
}

func TestFinalizeText(t *testing.T) {
	ctx := context.Background()
	game, player := setupActiveGame(t, -1001222222003)

	taskResponseRepo := mysqlrepo.NewTaskResponseRepo(testDB)
	taskResultRepo := mysqlrepo.NewTaskResultRepo(testDB)

	// Create an answered response for task_01.
	err := taskResponseRepo.Create(ctx, &entity.TaskResponse{
		GameID:   game.ID,
		PlayerID: player.ID,
		TaskID:   "task_01",
		Status:   entity.ResponseAnswered,
	})
	require.NoError(t, err)

	// Simulate finalization by creating a task_result.
	result := &entity.TaskResult{
		GameID:     game.ID,
		TaskID:     "task_01",
		ResultData: []byte(`{"type":"text"}`),
	}
	err = taskResultRepo.Create(ctx, result)
	require.NoError(t, err)

	// Verify result is retrievable.
	fetched, err := taskResultRepo.GetByTask(ctx, game.ID, "task_01")
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, game.ID, fetched.GameID)
	assert.Equal(t, "task_01", fetched.TaskID)
	assert.JSONEq(t, `{"type":"text"}`, string(fetched.ResultData))
}
