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
)

func newRequestAnswerer(ctrl *gomock.Controller, sender *mockSender) (*task.RequestAnswerer, *mocks.MockTaskResponseRepository, *mocks.MockPlayerStateRepository) {
	trRepo := mocks.NewMockTaskResponseRepository(ctrl)
	psRepo := mocks.NewMockPlayerStateRepository(ctrl)
	ra := task.NewRequestAnswerer(trRepo, psRepo, sender, testMsgs(), testTimings(), zerolog.Nop())
	return ra, trRepo, psRepo
}

func TestRequestAnswer_Success_SetsAwaitingState(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	ra, trRepo, psRepo := newRequestAnswerer(ctrl, sender)
	ctx := context.Background()

	trRepo.EXPECT().GetByPlayerAndTask(ctx, testGameID, testPlayerID, testTaskID).Return(nil, nil)
	psRepo.EXPECT().Upsert(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, s *entity.PlayerState) error {
		assert.Equal(t, entity.PlayerStateAwaitingAnswer, s.State)
		assert.Equal(t, testTaskID, s.TaskID)
		return nil
	})

	err := ra.RequestAnswer(ctx, testGame, testPlayer, testTask)
	require.NoError(t, err)
	assert.Len(t, sender.sent, 1) // awaiting_answer message sent
}

func TestRequestAnswer_AlreadyAnswered_SendsErrorMsg(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	ra, trRepo, _ := newRequestAnswerer(ctrl, sender)
	ctx := context.Background()

	existing := &entity.TaskResponse{Status: entity.ResponseAnswered}
	trRepo.EXPECT().GetByPlayerAndTask(ctx, testGameID, testPlayerID, testTaskID).Return(existing, nil)
	// Upsert must NOT be called.

	err := ra.RequestAnswer(ctx, testGame, testPlayer, testTask)
	require.NoError(t, err)
	assert.Len(t, sender.sent, 1) // already_answered message sent
}

func TestRequestAnswer_AlreadySkipped_SendsErrorMsg(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	ra, trRepo, _ := newRequestAnswerer(ctrl, sender)
	ctx := context.Background()

	existing := &entity.TaskResponse{Status: entity.ResponseSkipped}
	trRepo.EXPECT().GetByPlayerAndTask(ctx, testGameID, testPlayerID, testTaskID).Return(existing, nil)

	err := ra.RequestAnswer(ctx, testGame, testPlayer, testTask)
	require.NoError(t, err)
	assert.Len(t, sender.sent, 1) // already_answered message sent (same key for both)
}
