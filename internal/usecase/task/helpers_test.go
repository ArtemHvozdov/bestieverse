package task_test

import (
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	tele "gopkg.in/telebot.v3"
)

const (
	testChatID   int64  = 100
	testGameID   uint64 = 1
	testPlayerID uint64 = 2
	testTaskID          = "task_01"
)

var (
	testGame = &entity.Game{
		ID:     testGameID,
		ChatID: testChatID,
		Status: entity.GameActive,
	}
	testPlayer = &entity.Player{
		ID:             testPlayerID,
		GameID:         testGameID,
		TelegramUserID: 55,
		Username:       "testuser",
	}
	testTask = &config.Task{
		ID:    testTaskID,
		Order: 1,
		Type:  "question_answer",
	}
)

func testMsgs() *config.Messages {
	return &config.Messages{
		AwaitingAnswer:     []string{"Чекаємо відповідь {{.Mention}}"},
		AnswerAccepted:     "Відповідь прийнята!",
		AlreadyAnswered:    []string{"Ти вже відповів"},
		AlreadySkipped:     "Ти вже пропустив",
		SkipWithRemaining2: "Пропуск! Залишилось 2",
		SkipWithRemaining1: "Пропуск! Залишилось 1",
		SkipLast:           "Останній пропуск!",
		SkipNoRemaining:    "Пропуски вичерпано",
	}
}

func testTimings() *config.Timings {
	return &config.Timings{DeleteMessageDelay: time.Millisecond}
}

// mockSender records Send calls and returns stub messages.
type mockSender struct {
	sent    []interface{}
	deleted int
}

func (m *mockSender) Send(_ tele.Recipient, what interface{}, _ ...interface{}) (*tele.Message, error) {
	m.sent = append(m.sent, what)
	return &tele.Message{ID: len(m.sent)}, nil
}

func (m *mockSender) Delete(_ tele.Editable) error {
	m.deleted++
	return nil
}
