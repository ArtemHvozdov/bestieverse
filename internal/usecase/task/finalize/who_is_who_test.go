package finalize_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository/mocks"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task/finalize"
	"go.uber.org/mock/gomock"
)

func whoIsWhoTask() *config.Task {
	return &config.Task{
		ID:    "task_04",
		Order: 4,
		Questions: []config.TaskQuestion{
			{ID: "q1", Text: "Хто найбільший оптиміст?"},
			{ID: "q2", Text: "Хто найкраще готує?"},
		},
		Summary: config.TaskSummary{
			Type:       "who_is_who_results",
			HeaderText: "Ось результати!",
		},
	}
}

func whoIsWhoPlayers() []*entity.Player {
	return []*entity.Player{
		{ID: 10, GameID: 1, TelegramUserID: 111, Username: "alice", FirstName: "Alice"},
		{ID: 20, GameID: 1, TelegramUserID: 222, Username: "bob", FirstName: "Bob"},
		{ID: 30, GameID: 1, TelegramUserID: 333, Username: "carol", FirstName: "Carol"},
	}
}

func marshalAnswers(m map[string]int64) []byte {
	b, _ := json.Marshal(m)
	return b
}

func TestWhoIsWhoFinalizer_SupportedSummaryType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	f := finalize.NewWhoIsWhoFinalizer(
		mocks.NewMockPlayerRepository(ctrl),
		mocks.NewMockTaskResultRepository(ctrl),
		&mockSender{},
	)
	if f.SupportedSummaryType() != finalize.SummaryTypeWhoIsWho {
		t.Errorf("expected %q, got %q", finalize.SummaryTypeWhoIsWho, f.SupportedSummaryType())
	}
}

func TestWhoIsWhoFinalizer_CountsVotesAndSendsResults(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	sender := &mockSender{}

	game := &entity.Game{ID: 1, ChatID: 100}
	task := whoIsWhoTask()
	players := whoIsWhoPlayers()

	// 3 players all vote for alice (111) on q1.
	responses := []*entity.TaskResponse{
		{ID: 1, GameID: 1, PlayerID: 10, ResponseData: marshalAnswers(map[string]int64{"q1": 111, "q2": 222})},
		{ID: 2, GameID: 1, PlayerID: 20, ResponseData: marshalAnswers(map[string]int64{"q1": 111, "q2": 222})},
		{ID: 3, GameID: 1, PlayerID: 30, ResponseData: marshalAnswers(map[string]int64{"q1": 111, "q2": 333})},
	}

	playerRepo.EXPECT().GetAllByGame(gomock.Any(), game.ID).Return(players, nil)
	taskResultRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	f := finalize.NewWhoIsWhoFinalizer(playerRepo, taskResultRepo, sender)
	err := f.Finalize(context.Background(), game, task, responses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1 header + 1 results message.
	if len(sender.sent) != 2 {
		t.Fatalf("expected 2 messages (header + results), got %d", len(sender.sent))
	}
	if sender.sent[0] != "Ось результати!" {
		t.Errorf("first message should be header, got %q", sender.sent[0])
	}
	resultsMsg, ok := sender.sent[1].(string)
	if !ok {
		t.Fatal("results message should be a string")
	}
	if !strings.Contains(resultsMsg, "@alice") {
		t.Errorf("results should contain @alice, got %q", resultsMsg)
	}
}

func TestWhoIsWhoFinalizer_TieBreaking_FirstPlayerWins(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	sender := &mockSender{}

	game := &entity.Game{ID: 1, ChatID: 100}
	task := whoIsWhoTask()
	players := whoIsWhoPlayers() // alice, bob, carol

	// 1 vote for alice, 1 vote for bob on q1 — tie; alice is first in player list.
	responses := []*entity.TaskResponse{
		{ID: 1, GameID: 1, PlayerID: 10, ResponseData: marshalAnswers(map[string]int64{"q1": 111, "q2": 333})},
		{ID: 2, GameID: 1, PlayerID: 20, ResponseData: marshalAnswers(map[string]int64{"q1": 222, "q2": 333})},
	}

	playerRepo.EXPECT().GetAllByGame(gomock.Any(), game.ID).Return(players, nil)
	taskResultRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	f := finalize.NewWhoIsWhoFinalizer(playerRepo, taskResultRepo, sender)
	err := f.Finalize(context.Background(), game, task, responses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sender.sent) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(sender.sent))
	}
	resultsMsg, ok := sender.sent[1].(string)
	if !ok {
		t.Fatal("results message should be a string")
	}
	// In a tie, alice wins because she appears first in the player list.
	if !strings.Contains(resultsMsg, "@alice") {
		t.Errorf("tie-breaking: alice (first player) should win, got %q", resultsMsg)
	}
}

func TestWhoIsWhoFinalizer_SavesTaskResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	sender := &mockSender{}

	game := &entity.Game{ID: 1, ChatID: 100}
	task := whoIsWhoTask()

	responses := []*entity.TaskResponse{
		{ID: 1, GameID: 1, PlayerID: 10, ResponseData: marshalAnswers(map[string]int64{"q1": 111, "q2": 222})},
	}

	playerRepo.EXPECT().GetAllByGame(gomock.Any(), game.ID).Return(whoIsWhoPlayers(), nil)

	taskResultRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ interface{}, result *entity.TaskResult) error {
			if result.GameID != game.ID {
				t.Errorf("wrong GameID: %d", result.GameID)
			}
			if result.TaskID != task.ID {
				t.Errorf("wrong TaskID: %s", result.TaskID)
			}
			return nil
		},
	)

	f := finalize.NewWhoIsWhoFinalizer(playerRepo, taskResultRepo, sender)
	if err := f.Finalize(context.Background(), game, task, responses); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWhoIsWhoFinalizer_ResultContainsWinnerMention(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	sender := &mockSender{}

	game := &entity.Game{ID: 1, ChatID: 100}
	task := whoIsWhoTask()
	players := whoIsWhoPlayers()

	responses := []*entity.TaskResponse{
		{ID: 1, GameID: 1, PlayerID: 10, ResponseData: marshalAnswers(map[string]int64{"q1": 222, "q2": 111})},
		{ID: 2, GameID: 1, PlayerID: 20, ResponseData: marshalAnswers(map[string]int64{"q1": 222, "q2": 111})},
	}

	playerRepo.EXPECT().GetAllByGame(gomock.Any(), game.ID).Return(players, nil)
	taskResultRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	f := finalize.NewWhoIsWhoFinalizer(playerRepo, taskResultRepo, sender)
	if err := f.Finalize(context.Background(), game, task, responses); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sender.sent) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(sender.sent))
	}
	resultsMsg, ok := sender.sent[1].(string)
	if !ok {
		t.Fatal("results message should be a string")
	}
	// Both voted for bob (222) on q1 and alice (111) on q2.
	if !strings.Contains(resultsMsg, "@bob") {
		t.Errorf("q1 winner should be @bob, got %q", resultsMsg)
	}
	if !strings.Contains(resultsMsg, "@alice") {
		t.Errorf("q2 winner should be @alice, got %q", resultsMsg)
	}
}
