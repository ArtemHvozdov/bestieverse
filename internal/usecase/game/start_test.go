package game_test

import (
	"context"
	"errors"
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

// stubMedia is a no-op media.Storage that always returns an error (no file on disk).
type stubMedia struct{}

func (s *stubMedia) GetFile(name string) (*tele.Document, error)    { return nil, errNotFound }
func (s *stubMedia) GetPhoto(name string) (*tele.Photo, error)      { return nil, errNotFound }
func (s *stubMedia) GetAnimation(name string) (*tele.Animation, error) { return nil, errNotFound }

var errNotFound = errors.New("not found")

// stubPublisher satisfies game.TaskPublisher.
type stubPublisher struct{ called bool }

func (s *stubPublisher) Publish(_ context.Context, _ *entity.Game) error {
	s.called = true
	return nil
}

func newStarter(ctrl *gomock.Controller, sender *mockSender) (*game.Starter, *mocks.MockGameRepository) {
	gameRepo := mocks.NewMockGameRepository(ctrl)
	cfg := &config.Config{
		Messages: config.Messages{
			StartOnlyAdmin: "only_admin {{.Mention}}",
		},
		Game: config.GameMessages{
			StartMessage1: "intro text",
			StartMessage2: "rules text",
		},
		Timings: config.Timings{
			DeleteMessageDelay: time.Millisecond,
			TaskInfoInterval:   time.Millisecond,
		},
	}
	starter := game.NewStarter(gameRepo, &stubMedia{}, sender, &stubPublisher{}, cfg, zerolog.Nop())
	return starter, gameRepo
}

var (
	startGame        = &entity.Game{ID: 1, ChatID: testChatID, AdminUserID: 999, Status: entity.GamePending}
	startAdminPlayer = &entity.Player{ID: 1, TelegramUserID: 999, Username: "admin"}
	startNonAdmin    = &entity.Player{ID: 2, TelegramUserID: 55, Username: "user"}
)

func TestStart_NonAdmin_NoStatusUpdate(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	starter, _ := newStarter(ctrl, sender)
	// gameRepo.UpdateStatus must NOT be called

	err := starter.Start(context.Background(), startGame, startNonAdmin, &tele.Message{ID: 5})
	require.NoError(t, err)
	assert.Len(t, sender.messages, 1) // start_only_admin sent
}

func TestStart_Admin_UpdatesStatusAndSendsTwoMessages(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &mockSender{}
	starter, gameRepo := newStarter(ctrl, sender)
	ctx := context.Background()

	gameRepo.EXPECT().UpdateStatus(ctx, startGame.ID, entity.GameActive).Return(nil)

	err := starter.Start(ctx, startGame, startAdminPlayer, &tele.Message{ID: 5})
	require.NoError(t, err)
	assert.Len(t, sender.messages, 2) // start_game_message_1 + start_game_message_2
	assert.Equal(t, 1, sender.deleted) // start button message deleted
}
