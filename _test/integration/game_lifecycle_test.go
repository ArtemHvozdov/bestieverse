//go:build integration

package integration

import (
	"context"
	"testing"

	mysqlrepo "github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/mysql/repository"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAndJoin(t *testing.T) {
	ctx := context.Background()
	gameRepo := mysqlrepo.NewGameRepo(testDB)
	playerRepo := mysqlrepo.NewPlayerRepo(testDB)
	playerStateRepo := mysqlrepo.NewPlayerStateRepo(testDB)

	game := &entity.Game{
		ChatID:        -1001111111001,
		ChatName:      "Integration Test Chat",
		AdminUserID:   100001,
		AdminUsername: "admin_test",
		Status:        entity.GamePending,
	}
	created, err := gameRepo.Create(ctx, game)
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	t.Cleanup(func() { cleanupGame(t, testDB, created.ID) })

	// Verify the game is retrievable by chat ID.
	fetched, err := gameRepo.GetByChatID(ctx, game.ChatID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, entity.GamePending, fetched.Status)
	assert.Equal(t, "Integration Test Chat", fetched.ChatName)

	// Create admin as a player.
	admin := &entity.Player{
		GameID:         created.ID,
		TelegramUserID: 100001,
		Username:       "admin_test",
		FirstName:      "Admin",
	}
	adminPlayer, err := playerRepo.Create(ctx, admin)
	require.NoError(t, err)
	require.NotZero(t, adminPlayer.ID)

	// Set admin state to idle.
	err = playerStateRepo.Upsert(ctx, &entity.PlayerState{
		GameID:   created.ID,
		PlayerID: adminPlayer.ID,
		State:    entity.PlayerStateIdle,
	})
	require.NoError(t, err)

	// Join 3 more players.
	players := []struct {
		telegramID int64
		username   string
	}{
		{200001, "player_one"},
		{200002, "player_two"},
		{200003, "player_three"},
	}
	for _, p := range players {
		pl, err := playerRepo.Create(ctx, &entity.Player{
			GameID:         created.ID,
			TelegramUserID: p.telegramID,
			Username:       p.username,
			FirstName:      p.username,
		})
		require.NoError(t, err)
		err = playerStateRepo.Upsert(ctx, &entity.PlayerState{
			GameID:   created.ID,
			PlayerID: pl.ID,
			State:    entity.PlayerStateIdle,
		})
		require.NoError(t, err)
	}

	// Verify all 4 players are in the game (admin + 3).
	allPlayers, err := playerRepo.GetAllByGame(ctx, created.ID)
	require.NoError(t, err)
	assert.Len(t, allPlayers, 4)

	// Verify no player has awaiting state.
	awaiting, err := playerStateRepo.GetAllAwaitingByGame(ctx, created.ID)
	require.NoError(t, err)
	assert.Empty(t, awaiting)
}

func TestStartGame(t *testing.T) {
	ctx := context.Background()
	gameRepo := mysqlrepo.NewGameRepo(testDB)

	game := &entity.Game{
		ChatID:        -1001111111002,
		ChatName:      "Start Test Chat",
		AdminUserID:   100010,
		AdminUsername: "admin_start",
		Status:        entity.GamePending,
	}
	created, err := gameRepo.Create(ctx, game)
	require.NoError(t, err)
	t.Cleanup(func() { cleanupGame(t, testDB, created.ID) })

	// Transition to active.
	err = gameRepo.UpdateStatus(ctx, created.ID, entity.GameActive)
	require.NoError(t, err)

	// Verify status is active.
	fetched, err := gameRepo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, entity.GameActive, fetched.Status)
	assert.NotNil(t, fetched.StartedAt)

	// Verify game appears in GetAllActive.
	active, err := gameRepo.GetAllActive(ctx)
	require.NoError(t, err)
	found := false
	for _, g := range active {
		if g.ID == created.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "started game should be in GetAllActive")
}
