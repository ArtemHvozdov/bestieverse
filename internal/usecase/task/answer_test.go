package task_test

import (
	"context"
	"testing"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository/mocks"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	tele "gopkg.in/telebot.v3"
)

func newAnswerer(ctrl *gomock.Controller, sender *mockSender) (*task.Answerer, *mocks.MockTaskResponseRepository, *mocks.MockPlayerStateRepository) {
	trRepo := mocks.NewMockTaskResponseRepository(ctrl)
	psRepo := mocks.NewMockPlayerStateRepository(ctrl)
	a := task.NewAnswerer(trRepo, psRepo, sender, testMsgs(), testTimings(), zerolog.Nop())
	return a, trRepo, psRepo
}

func TestAnswer_AwaitingState_RecordsResponseAndSetsIdle(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	a, trRepo, psRepo := newAnswerer(ctrl, sender)
	ctx := context.Background()

	state := &entity.PlayerState{
		GameID:   testGameID,
		PlayerID: testPlayerID,
		State:    entity.PlayerStateAwaitingAnswer,
		TaskID:   testTaskID,
	}
	psRepo.EXPECT().GetByPlayerAndGame(ctx, testGameID, testPlayerID).Return(state, nil)
	trRepo.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.TaskResponse) error {
		assert.Equal(t, entity.ResponseAnswered, r.Status)
		assert.Equal(t, testTaskID, r.TaskID)
		return nil
	})
	psRepo.EXPECT().SetIdle(ctx, testGameID, testPlayerID).Return(nil)

	err := a.Answer(ctx, testGame, testPlayer, &tele.Message{})
	require.NoError(t, err)
	assert.Len(t, sender.sent, 1) // answer_accepted sent
}

func TestAnswer_IdleState_IgnoresMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	a, trRepo, psRepo := newAnswerer(ctrl, sender)
	ctx := context.Background()

	state := &entity.PlayerState{State: entity.PlayerStateIdle}
	psRepo.EXPECT().GetByPlayerAndGame(ctx, testGameID, testPlayerID).Return(state, nil)
	// taskResponseRepo.Create must NOT be called.
	_ = trRepo

	err := a.Answer(ctx, testGame, testPlayer, &tele.Message{})
	require.NoError(t, err)
	assert.Empty(t, sender.sent)
}

func TestAnswer_NoState_IgnoresMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	a, trRepo, psRepo := newAnswerer(ctrl, sender)
	ctx := context.Background()

	psRepo.EXPECT().GetByPlayerAndGame(ctx, testGameID, testPlayerID).Return(nil, nil)
	_ = trRepo

	err := a.Answer(ctx, testGame, testPlayer, &tele.Message{})
	require.NoError(t, err)
	assert.Empty(t, sender.sent)
}

func TestAnswer_CreateError_PropagatesError(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	a, trRepo, psRepo := newAnswerer(ctrl, sender)
	ctx := context.Background()

	state := &entity.PlayerState{State: entity.PlayerStateAwaitingAnswer, TaskID: testTaskID}
	psRepo.EXPECT().GetByPlayerAndGame(ctx, testGameID, testPlayerID).Return(state, nil)
	trRepo.EXPECT().Create(ctx, gomock.Any()).Return(assert.AnError)

	err := a.Answer(ctx, testGame, testPlayer, &tele.Message{})
	require.Error(t, err)
}
