package notification_test

import (
	"context"
	"testing"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository/mocks"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/notification"
	"github.com/rs/zerolog"
	"go.uber.org/mock/gomock"
	tele "gopkg.in/telebot.v3"
)

type mockSender struct {
	sent int
}

func (m *mockSender) Send(_ tele.Recipient, _ interface{}, _ ...interface{}) (*tele.Message, error) {
	m.sent++
	return &tele.Message{ID: m.sent}, nil
}

func testCfg(reminderDelay time.Duration) *config.Config {
	return &config.Config{
		Tasks: []config.Task{
			{ID: "task_01", Order: 1},
		},
		Messages: config.Messages{
			Reminder: []string{"Hey {{.Mention}}!"},
		},
		Timings: config.Timings{ReminderDelay: reminderDelay},
	}
}

func TestSendReminders_PlayerWithoutResponse_ReceivesReminder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gameRepo := mocks.NewMockGameRepository(ctrl)
	notifRepo := mocks.NewMockNotificationRepository(ctrl)
	sender := &mockSender{}
	log := zerolog.Nop()

	published := time.Now().Add(-2 * time.Hour)
	game := &entity.Game{
		ID:                   1,
		ChatID:               100,
		Status:               entity.GameActive,
		CurrentTaskOrder:     1,
		CurrentTaskPublishedAt: &published,
	}
	player := &entity.Player{ID: 10, GameID: 1, TelegramUserID: 111, Username: "alice"}

	gameRepo.EXPECT().GetAllActive(gomock.Any()).Return([]*entity.Game{game}, nil)
	notifRepo.EXPECT().GetUnnotifiedPlayers(gomock.Any(), game.ID, "task_01").Return([]*entity.Player{player}, nil)
	notifRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	rs := notification.NewReminderSender(gameRepo, notifRepo, sender, testCfg(time.Hour), log)
	err := rs.SendReminders(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.sent != 1 {
		t.Errorf("expected 1 reminder, got %d", sender.sent)
	}
}

func TestSendReminders_ReminderDelayNotElapsed_NoReminder(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gameRepo := mocks.NewMockGameRepository(ctrl)
	notifRepo := mocks.NewMockNotificationRepository(ctrl)
	sender := &mockSender{}
	log := zerolog.Nop()

	// Published 10 minutes ago, but delay is 1 hour.
	published := time.Now().Add(-10 * time.Minute)
	game := &entity.Game{
		ID:                   1,
		ChatID:               100,
		Status:               entity.GameActive,
		CurrentTaskOrder:     1,
		CurrentTaskPublishedAt: &published,
	}

	gameRepo.EXPECT().GetAllActive(gomock.Any()).Return([]*entity.Game{game}, nil)
	// notifRepo.GetUnnotifiedPlayers must NOT be called.

	rs := notification.NewReminderSender(gameRepo, notifRepo, sender, testCfg(time.Hour), log)
	err := rs.SendReminders(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.sent != 0 {
		t.Errorf("expected 0 reminders, got %d", sender.sent)
	}
}

func TestSendReminders_AlreadyNotified_NoRepeat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gameRepo := mocks.NewMockGameRepository(ctrl)
	notifRepo := mocks.NewMockNotificationRepository(ctrl)
	sender := &mockSender{}
	log := zerolog.Nop()

	published := time.Now().Add(-2 * time.Hour)
	game := &entity.Game{
		ID:                   1,
		ChatID:               100,
		Status:               entity.GameActive,
		CurrentTaskOrder:     1,
		CurrentTaskPublishedAt: &published,
	}

	// GetUnnotifiedPlayers returns empty — all players are already notified.
	gameRepo.EXPECT().GetAllActive(gomock.Any()).Return([]*entity.Game{game}, nil)
	notifRepo.EXPECT().GetUnnotifiedPlayers(gomock.Any(), game.ID, "task_01").Return([]*entity.Player{}, nil)

	rs := notification.NewReminderSender(gameRepo, notifRepo, sender, testCfg(time.Hour), log)
	err := rs.SendReminders(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.sent != 0 {
		t.Errorf("expected 0 reminders, got %d", sender.sent)
	}
}

func TestSendReminders_ChecksGamePublishedAt_NotPlayerJoinedAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	gameRepo := mocks.NewMockGameRepository(ctrl)
	notifRepo := mocks.NewMockNotificationRepository(ctrl)
	sender := &mockSender{}
	log := zerolog.Nop()

	// Game published recently — reminder delay not elapsed.
	published := time.Now().Add(-5 * time.Minute)
	game := &entity.Game{
		ID:                   1,
		ChatID:               100,
		Status:               entity.GameActive,
		CurrentTaskOrder:     1,
		CurrentTaskPublishedAt: &published,
	}

	gameRepo.EXPECT().GetAllActive(gomock.Any()).Return([]*entity.Game{game}, nil)
	// GetUnnotifiedPlayers must NOT be called — we check game.CurrentTaskPublishedAt.

	rs := notification.NewReminderSender(gameRepo, notifRepo, sender, testCfg(time.Hour), log)
	err := rs.SendReminders(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sender.sent != 0 {
		t.Errorf("expected 0 reminders (game published recently), got %d", sender.sent)
	}
}
