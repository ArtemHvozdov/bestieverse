package finalize_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository/mocks"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task/finalize"
	"github.com/rs/zerolog"
	"go.uber.org/mock/gomock"
)

func TestOpenAICollageFinalizer_SupportedSummaryType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	f := finalize.NewOpenAICollageFinalizer(
		mocks.NewMockTaskResultRepository(ctrl),
		&mockSender{},
		zerolog.Nop(),
	)
	if f.SupportedSummaryType() != finalize.SummaryTypeOpenAICollage {
		t.Errorf("expected %q, got %q", finalize.SummaryTypeOpenAICollage, f.SupportedSummaryType())
	}
}

func TestOpenAICollageFinalizer_Finalize_ResultExists_ReturnsNil(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	sender := &mockSender{}

	game := &entity.Game{ID: 1, ChatID: 100}
	task := &config.Task{
		ID:      "task_12",
		Order:   12,
		Summary: config.TaskSummary{Type: "openai_collage"},
	}

	resultData, _ := json.Marshal(map[string]interface{}{"image_generated": true})
	existingResult := &entity.TaskResult{
		ID:          1,
		GameID:      1,
		TaskID:      "task_12",
		ResultData:  resultData,
		FinalizedAt: time.Now(),
	}

	taskResultRepo.EXPECT().
		GetByTask(gomock.Any(), uint64(1), "task_12").
		Return(existingResult, nil)

	f := finalize.NewOpenAICollageFinalizer(taskResultRepo, sender, zerolog.Nop())

	err := f.Finalize(context.Background(), game, task, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No messages sent by this finalizer — it only verifies the result
	if len(sender.sent) != 0 {
		t.Errorf("expected no messages sent, got %d", len(sender.sent))
	}
}

func TestOpenAICollageFinalizer_Finalize_ResultMissing_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	sender := &mockSender{}

	game := &entity.Game{ID: 1, ChatID: 100}
	task := &config.Task{
		ID:      "task_12",
		Order:   12,
		Summary: config.TaskSummary{Type: "openai_collage"},
	}

	taskResultRepo.EXPECT().
		GetByTask(gomock.Any(), uint64(1), "task_12").
		Return(nil, nil) // nil means not found

	f := finalize.NewOpenAICollageFinalizer(taskResultRepo, sender, zerolog.Nop())

	err := f.Finalize(context.Background(), game, task, nil)
	if err == nil {
		t.Fatal("expected error when task_result is missing, got nil")
	}
}
