package game_test

import (
	"context"
	"testing"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository/mocks"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/game"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	tele "gopkg.in/telebot.v3"
)

func newJoiner(ctrl *gomock.Controller, sender *mockSender) (*game.Joiner, *mocks.MockGameRepository, *mocks.MockPlayerRepository, *mocks.MockPlayerStateRepository) {
	gameRepo := mocks.NewMockGameRepository(ctrl)
	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	psRepo := mocks.NewMockPlayerStateRepository(ctrl)
	msgs := &config.Messages{
		JoinWelcome:       []string{"welcome {{.Mention}}"},
		JoinAlreadyMember: "already {{.Mention}}",
		JoinAdminAlready:  "admin {{.Mention}}",
	}
	timings := &config.Timings{DeleteMessageDelay: time.Millisecond}
	joiner := game.NewJoiner(gameRepo, playerRepo, psRepo, sender, msgs, timings, zerolog.Nop())
	return joiner, gameRepo, playerRepo, psRepo
}

var testPendingGame = &entity.Game{ID: 1, ChatID: testChatID, Status: entity.GamePending, AdminUserID: 999}

func TestJoin_Success_CreatesPlayerAndState(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	joiner, gameRepo, playerRepo, psRepo := newJoiner(ctrl, sender)
	ctx := context.Background()

	user := tele.User{ID: 55, Username: "newplayer", FirstName: "Player"}

	gameRepo.EXPECT().GetByChatID(ctx, testChatID).Return(testPendingGame, nil)
	playerRepo.EXPECT().GetByGameAndTelegramID(ctx, testPendingGame.ID, user.ID).Return(nil, nil)
	playerRepo.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.Player) (*entity.Player, error) {
		p.ID = 2
		return p, nil
	})
	psRepo.EXPECT().Upsert(ctx, gomock.Any()).Return(nil)

	err := joiner.Join(ctx, testChatID, user)
	require.NoError(t, err)
	assert.Len(t, sender.messages, 1) // join_welcome sent
}

func TestJoin_AlreadyMember_NilCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	joiner, gameRepo, playerRepo, _ := newJoiner(ctrl, sender)
	ctx := context.Background()

	user := tele.User{ID: 55, Username: "existing"}
	existingPlayer := &entity.Player{ID: 2, TelegramUserID: 55}

	gameRepo.EXPECT().GetByChatID(ctx, testChatID).Return(testPendingGame, nil)
	playerRepo.EXPECT().GetByGameAndTelegramID(ctx, testPendingGame.ID, user.ID).Return(existingPlayer, nil)
	// playerRepo.Create must NOT be called

	err := joiner.Join(ctx, testChatID, user)
	require.NoError(t, err)
	assert.Len(t, sender.messages, 1) // join_already_member sent
}

func TestJoin_AdminUser_NilCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	joiner, gameRepo, _, _ := newJoiner(ctrl, sender)
	ctx := context.Background()

	adminUser := tele.User{ID: 999, Username: "admin"} // matches testPendingGame.AdminUserID

	gameRepo.EXPECT().GetByChatID(ctx, testChatID).Return(testPendingGame, nil)
	// playerRepo methods must NOT be called

	err := joiner.Join(ctx, testChatID, adminUser)
	require.NoError(t, err)
	assert.Len(t, sender.messages, 1) // join_admin_already sent
}

func TestJoin_GameNotPending_EarlyReturn(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	joiner, gameRepo, _, _ := newJoiner(ctrl, sender)
	ctx := context.Background()

	activeGame := &entity.Game{ID: 1, ChatID: testChatID, Status: entity.GameActive}
	gameRepo.EXPECT().GetByChatID(ctx, testChatID).Return(activeGame, nil)
	// Nothing else should be called

	err := joiner.Join(ctx, testChatID, tele.User{ID: 55})
	require.NoError(t, err)
	assert.Len(t, sender.messages, 0) // no messages sent
}
