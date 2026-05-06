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

func newSkipper(ctrl *gomock.Controller, sender *mockSender) (*task.Skipper, *mocks.MockTaskResponseRepository, *mocks.MockPlayerRepository) {
	trRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	s := task.NewSkipper(trRepo, playerRepo, sender, testMsgs(), testTimings(), zerolog.Nop())
	return s, trRepo, playerRepo
}

func playerWithSkips(n int) *entity.Player {
	p := *testPlayer
	p.SkipCount = n
	return &p
}

func TestSkip_FirstSkip_IncrementAndSendRemaining2(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	s, trRepo, playerRepo := newSkipper(ctrl, sender)
	ctx := context.Background()

	trRepo.EXPECT().GetByPlayerAndTask(ctx, testGameID, testPlayerID, testTaskID).Return(nil, nil)
	playerRepo.EXPECT().IncrementSkipCount(ctx, testPlayerID).Return(nil)
	trRepo.EXPECT().Create(ctx, gomock.Any()).Return(nil)

	err := s.Skip(ctx, testGame, playerWithSkips(0), testTaskID)
	require.NoError(t, err)
	require.Len(t, sender.sent, 1)
	assert.Contains(t, sender.sent[0], "2") // skip_with_remaining_2
}

func TestSkip_SecondSkip_SendRemaining1(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	s, trRepo, playerRepo := newSkipper(ctrl, sender)
	ctx := context.Background()

	trRepo.EXPECT().GetByPlayerAndTask(ctx, testGameID, testPlayerID, testTaskID).Return(nil, nil)
	playerRepo.EXPECT().IncrementSkipCount(ctx, testPlayerID).Return(nil)
	trRepo.EXPECT().Create(ctx, gomock.Any()).Return(nil)

	err := s.Skip(ctx, testGame, playerWithSkips(1), testTaskID)
	require.NoError(t, err)
	require.Len(t, sender.sent, 1)
	assert.Contains(t, sender.sent[0], "1") // skip_with_remaining_1
}

func TestSkip_ThirdSkip_SendSkipLast(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	s, trRepo, playerRepo := newSkipper(ctrl, sender)
	ctx := context.Background()

	trRepo.EXPECT().GetByPlayerAndTask(ctx, testGameID, testPlayerID, testTaskID).Return(nil, nil)
	playerRepo.EXPECT().IncrementSkipCount(ctx, testPlayerID).Return(nil)
	trRepo.EXPECT().Create(ctx, gomock.Any()).Return(nil)

	err := s.Skip(ctx, testGame, playerWithSkips(2), testTaskID)
	require.NoError(t, err)
	require.Len(t, sender.sent, 1)
	assert.Equal(t, testMsgs().SkipLast, sender.sent[0]) // skip_last
}

func TestSkip_FourthSkip_BlockedNoIncrement(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	s, trRepo, _ := newSkipper(ctrl, sender)
	ctx := context.Background()

	trRepo.EXPECT().GetByPlayerAndTask(ctx, testGameID, testPlayerID, testTaskID).Return(nil, nil)
	// IncrementSkipCount must NOT be called.

	err := s.Skip(ctx, testGame, playerWithSkips(3), testTaskID)
	require.NoError(t, err)
	require.Len(t, sender.sent, 1)
	assert.Equal(t, testMsgs().SkipNoRemaining, sender.sent[0])
}

func TestSkip_AlreadySkipped_SendsAlreadySkipped(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	s, trRepo, _ := newSkipper(ctrl, sender)
	ctx := context.Background()

	existing := &entity.TaskResponse{Status: entity.ResponseSkipped}
	trRepo.EXPECT().GetByPlayerAndTask(ctx, testGameID, testPlayerID, testTaskID).Return(existing, nil)
	// IncrementSkipCount must NOT be called.

	err := s.Skip(ctx, testGame, testPlayer, testTaskID)
	require.NoError(t, err)
	require.Len(t, sender.sent, 1)
	assert.Equal(t, testMsgs().AlreadySkipped, sender.sent[0])
}

func TestSkip_AlreadyAnswered_SendsAlreadyAnswered(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	s, trRepo, _ := newSkipper(ctrl, sender)
	ctx := context.Background()

	existing := &entity.TaskResponse{Status: entity.ResponseAnswered}
	trRepo.EXPECT().GetByPlayerAndTask(ctx, testGameID, testPlayerID, testTaskID).Return(existing, nil)

	err := s.Skip(ctx, testGame, testPlayer, testTaskID)
	require.NoError(t, err)
	assert.Len(t, sender.sent, 1) // already_answered
}
