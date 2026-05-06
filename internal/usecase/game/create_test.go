package game_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository/mocks"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/game"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	tele "gopkg.in/telebot.v3"
)

func newCreator(ctrl *gomock.Controller) (*game.Creator, *mocks.MockGameRepository, *mocks.MockPlayerRepository, *mocks.MockPlayerStateRepository) {
	gameRepo := mocks.NewMockGameRepository(ctrl)
	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	psRepo := mocks.NewMockPlayerStateRepository(ctrl)
	creator := game.NewCreator(gameRepo, playerRepo, psRepo, zerolog.Nop())
	return creator, gameRepo, playerRepo, psRepo
}

var (
	testAdminUser = tele.User{ID: 42, Username: "admin", FirstName: "Admin"}
	testChatID    = int64(100)
)

func TestCreate_NewChat_CallsAllRepos(t *testing.T) {
	ctrl := gomock.NewController(t)
	creator, gameRepo, playerRepo, psRepo := newCreator(ctrl)
	ctx := context.Background()

	gameRepo.EXPECT().GetByChatID(ctx, testChatID).Return(nil, nil)
	gameRepo.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, g *entity.Game) (*entity.Game, error) {
		g.ID = 1
		return g, nil
	})
	playerRepo.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.Player) (*entity.Player, error) {
		p.ID = 1
		return p, nil
	})
	psRepo.EXPECT().Upsert(ctx, gomock.Any()).Return(nil)

	result, err := creator.Create(ctx, testChatID, "Test Chat", testAdminUser)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, uint64(1), result.ID)
}

func TestCreate_GameAlreadyExists_ReturnsNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	creator, gameRepo, _, _ := newCreator(ctrl)
	ctx := context.Background()

	existing := &entity.Game{ID: 1, ChatID: testChatID, Status: entity.GamePending}
	gameRepo.EXPECT().GetByChatID(ctx, testChatID).Return(existing, nil)
	// No further expectations — any unexpected call fails the test

	result, err := creator.Create(ctx, testChatID, "Test Chat", testAdminUser)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestCreate_GameRepoError_ReturnsWrappedError(t *testing.T) {
	ctrl := gomock.NewController(t)
	creator, gameRepo, playerRepo, _ := newCreator(ctrl)
	ctx := context.Background()

	dbErr := errors.New("db error")
	gameRepo.EXPECT().GetByChatID(ctx, testChatID).Return(nil, nil)
	gameRepo.EXPECT().Create(ctx, gomock.Any()).Return(nil, dbErr)
	// playerRepo.Create must NOT be called
	_ = playerRepo

	_, err := creator.Create(ctx, testChatID, "Test Chat", testAdminUser)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "game.Create")
}
