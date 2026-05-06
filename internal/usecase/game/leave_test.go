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

func newLeaver(ctrl *gomock.Controller, sender *mockSender) (*game.Leaver, *mocks.MockPlayerRepository) {
	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	msgs := &config.Messages{
		LeaveConfirm:      "confirm {{.Mention}}",
		LeaveSuccess:      "success {{.Mention}}",
		CancelLeave:       "cancel {{.Mention}}",
		LeaveAdminBlocked: "admin_blocked {{.Mention}}",
	}
	timings := &config.Timings{DeleteMessageDelay: time.Millisecond}
	leaver := game.NewLeaver(playerRepo, sender, msgs, timings, zerolog.Nop())
	return leaver, playerRepo
}

var (
	leaveGame         = &entity.Game{ID: 1, ChatID: testChatID, AdminUserID: 999}
	regularPlayer     = &entity.Player{ID: 2, TelegramUserID: 55, Username: "user", GameID: 1}
	adminPlayerLeave  = &entity.Player{ID: 1, TelegramUserID: 999, Username: "admin", GameID: 1}
)

func TestInitiateLeave_NonAdmin_SendsConfirm(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	leaver, _ := newLeaver(ctrl, sender)

	err := leaver.InitiateLeave(context.Background(), leaveGame, regularPlayer, &tele.ReplyMarkup{})
	require.NoError(t, err)
	assert.Len(t, sender.messages, 1)
}

func TestInitiateLeave_Admin_SendsBlocked(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	leaver, _ := newLeaver(ctrl, sender)

	err := leaver.InitiateLeave(context.Background(), leaveGame, adminPlayerLeave, &tele.ReplyMarkup{})
	require.NoError(t, err)
	assert.Len(t, sender.messages, 1)
}

func TestConfirmLeave_DeletesPlayerAndSendsSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	leaver, playerRepo := newLeaver(ctrl, sender)
	ctx := context.Background()

	playerRepo.EXPECT().Delete(ctx, regularPlayer.ID).Return(nil)

	err := leaver.ConfirmLeave(ctx, leaveGame, regularPlayer, &tele.Message{ID: 10})
	require.NoError(t, err)
	assert.Len(t, sender.messages, 1) // leave_success sent
}

func TestCancelLeave_DeletesConfirmAndSendsCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	leaver, _ := newLeaver(ctrl, sender)

	confirmMsg := &tele.Message{ID: 10}
	err := leaver.CancelLeave(context.Background(), leaveGame, regularPlayer, confirmMsg)
	require.NoError(t, err)
	assert.Equal(t, 1, sender.deleted) // confirm message deleted
	assert.Len(t, sender.messages, 1) // cancel_leave sent
}
