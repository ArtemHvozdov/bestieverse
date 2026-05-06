package task_test

import (
	"context"
	"testing"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository/mocks"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	tele "gopkg.in/telebot.v3"
)

// stubMedia satisfies media.Storage; all methods return nil objects and no error
// so caller falls back to sending text only (no GIF on disk in tests).
type stubMedia struct{}

func (s *stubMedia) GetFile(_ string) (*tele.Document, error)    { return nil, assert.AnError }
func (s *stubMedia) GetPhoto(_ string) (*tele.Photo, error)      { return nil, assert.AnError }
func (s *stubMedia) GetAnimation(_ string) (*tele.Animation, error) { return nil, assert.AnError }

func newPublisher(ctrl *gomock.Controller, sender *mockSender, tasks []config.Task) (*task.Publisher, *mocks.MockGameRepository) {
	gameRepo := mocks.NewMockGameRepository(ctrl)
	cfg := &config.Config{Tasks: tasks}
	pub := task.NewPublisher(gameRepo, &stubMedia{}, sender, cfg, zerolog.Nop())
	return pub, gameRepo
}

func TestPublish_FirstTask_UpdatesCurrentTaskAndSendsMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	tasks := []config.Task{
		{ID: "task_01", Order: 1, Type: "question_answer", MediaFile: "tasks/task_01.gif", Text: "Task text"},
	}
	pub, gameRepo := newPublisher(ctrl, sender, tasks)
	ctx := context.Background()

	game := &entity.Game{ID: testGameID, ChatID: testChatID, Status: entity.GameActive, CurrentTaskOrder: 0}

	gameRepo.EXPECT().UpdateCurrentTask(ctx, testGameID, 1, gomock.Any()).Return(nil)

	err := pub.Publish(ctx, game)
	require.NoError(t, err)
	// Media unavailable in tests, so the text fallback is sent.
	assert.Len(t, sender.sent, 1)
}

func TestPublish_AllTasksDone_NoAction(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	tasks := []config.Task{
		{ID: "task_01", Order: 1, Type: "question_answer"},
	}
	pub, _ := newPublisher(ctrl, sender, tasks)
	ctx := context.Background()

	now := time.Now()
	game := &entity.Game{
		ID:                     testGameID,
		ChatID:                 testChatID,
		Status:                 entity.GameActive,
		CurrentTaskOrder:       1,
		CurrentTaskPublishedAt: &now,
	}

	err := pub.Publish(ctx, game)
	require.NoError(t, err)
	// No next task (order 2 doesn't exist) — nothing sent, no repo calls.
	assert.Empty(t, sender.sent)
}
