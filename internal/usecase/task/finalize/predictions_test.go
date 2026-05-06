package finalize_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository/mocks"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task/finalize"
	"go.uber.org/mock/gomock"
)

func TestPredictionsFinalizer_SupportedSummaryType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	f := finalize.NewPredictionsFinalizer(
		mocks.NewMockPlayerRepository(ctrl),
		mocks.NewMockTaskResultRepository(ctrl),
		&mockSender{},
	)
	if f.SupportedSummaryType() != finalize.SummaryTypePredictions {
		t.Errorf("expected %q, got %q", finalize.SummaryTypePredictions, f.SupportedSummaryType())
	}
}

func TestPredictionsFinalizer_SendsHeaderAndPredictions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	sender := &mockSender{}

	game := &entity.Game{ID: 1, ChatID: 100}
	task := &config.Task{
		ID: "task_03",
		Summary: config.TaskSummary{
			Type:        "predictions",
			HeaderText:  "Here are predictions",
			Predictions: []string{"Hello {{.Mention}}!"},
		},
	}

	players := []*entity.Player{
		{ID: 10, GameID: 1, TelegramUserID: 111, Username: "alice"},
		{ID: 20, GameID: 1, TelegramUserID: 222, Username: "bob"},
		{ID: 30, GameID: 1, TelegramUserID: 333, Username: "carol"},
	}
	responses := []*entity.TaskResponse{
		{ID: 1, GameID: 1, PlayerID: 10},
		{ID: 2, GameID: 1, PlayerID: 20},
		{ID: 3, GameID: 1, PlayerID: 30},
	}

	playerRepo.EXPECT().GetAllByGame(gomock.Any(), game.ID).Return(players, nil)
	taskResultRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	f := finalize.NewPredictionsFinalizer(playerRepo, taskResultRepo, sender)
	err := f.Finalize(context.Background(), game, task, responses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1 header + 3 predictions
	if len(sender.sent) != 4 {
		t.Fatalf("expected 4 messages (1 header + 3 predictions), got %d", len(sender.sent))
	}
	if sender.sent[0] != "Here are predictions" {
		t.Errorf("first message should be header, got %q", sender.sent[0])
	}
}

func TestPredictionsFinalizer_MentionInPrediction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	sender := &mockSender{}

	game := &entity.Game{ID: 1, ChatID: 100}
	task := &config.Task{
		ID: "task_03",
		Summary: config.TaskSummary{
			Type:        "predictions",
			HeaderText:  "Header",
			Predictions: []string{"Hey {{.Mention}}!"},
		},
	}

	players := []*entity.Player{
		{ID: 10, GameID: 1, TelegramUserID: 111, Username: "alice"},
	}
	responses := []*entity.TaskResponse{{ID: 1, GameID: 1, PlayerID: 10}}

	playerRepo.EXPECT().GetAllByGame(gomock.Any(), game.ID).Return(players, nil)
	taskResultRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	f := finalize.NewPredictionsFinalizer(playerRepo, taskResultRepo, sender)
	err := f.Finalize(context.Background(), game, task, responses)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	predMsg, ok := sender.sent[1].(string)
	if !ok {
		t.Fatal("prediction message should be a string")
	}
	if !strings.Contains(predMsg, "@alice") {
		t.Errorf("prediction should contain @alice mention, got %q", predMsg)
	}
}

func TestPredictionsFinalizer_SavesTaskResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	sender := &mockSender{}

	game := &entity.Game{ID: 1, ChatID: 100}
	task := &config.Task{
		ID: "task_03",
		Summary: config.TaskSummary{
			Type:        "predictions",
			HeaderText:  "H",
			Predictions: []string{"Prediction"},
		},
	}
	responses := []*entity.TaskResponse{{ID: 1, GameID: 1, PlayerID: 10}}

	playerRepo.EXPECT().GetAllByGame(gomock.Any(), game.ID).Return([]*entity.Player{
		{ID: 10, TelegramUserID: 111, Username: "user"},
	}, nil)

	// Verify Create is called with correct fields.
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

	f := finalize.NewPredictionsFinalizer(playerRepo, taskResultRepo, sender)
	if err := f.Finalize(context.Background(), game, task, responses); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
