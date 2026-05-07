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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func testCollageTask() *config.Task {
	return &config.Task{
		ID:    "task_02",
		Order: 2,
		Type:  "question_answer",
		Summary: config.TaskSummary{
			Type:        "collage",
			PendingText: "Чекайте...",
			ReadyText:   "Готово!",
			HqText:      "Висока якість",
		},
		Subtask: &config.SubtaskVotingCollage{
			Type: "voting_collage",
			Categories: []config.VotingCategory{
				{
					ID: "drink",
					Options: []config.VotingOption{
						{ID: "smoothie", MediaFile: "task_02/smoothie.jpg"},
						{ID: "cappuccino", MediaFile: "task_02/cappuccino.jpg"},
					},
				},
				{
					ID: "music",
					Options: []config.VotingOption{
						{ID: "pop", MediaFile: "task_02/pop.jpg"},
						{ID: "rock", MediaFile: "task_02/rock.jpg"},
					},
				},
			},
		},
	}
}

func makeCollageResponse(choices map[string]string) *entity.TaskResponse {
	data, _ := json.Marshal(choices)
	return &entity.TaskResponse{
		Status:       entity.ResponseAnswered,
		ResponseData: data,
	}
}

func TestCollageFinalizer_SupportedType(t *testing.T) {
	f := finalize.NewCollageFinalizer(nil, noopMedia{}, &mockSender{}, zerolog.Nop())
	assert.Equal(t, "collage", f.SupportedSummaryType())
}

// TestCollageFinalizer_VoteCountingAndWinner verifies that the majority winner is chosen
// and the task_result JSON contains the correct category-to-winner mapping.
func TestCollageFinalizer_VoteCountingAndWinner(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	sender := &mockSender{}

	task := testCollageTask()
	game := &entity.Game{ID: 1, ChatID: 100}

	// 3 players vote smoothie for drink, 1 votes cappuccino.
	// All vote pop for music.
	responses := []*entity.TaskResponse{
		makeCollageResponse(map[string]string{"drink": "smoothie", "music": "pop"}),
		makeCollageResponse(map[string]string{"drink": "smoothie", "music": "pop"}),
		makeCollageResponse(map[string]string{"drink": "smoothie", "music": "pop"}),
		makeCollageResponse(map[string]string{"drink": "cappuccino", "music": "pop"}),
	}

	var capturedResult entity.TaskResult
	taskResultRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, r *entity.TaskResult) error {
			capturedResult = *r
			return nil
		})

	f := finalize.NewCollageFinalizer(taskResultRepo, noopMedia{}, sender, zerolog.Nop())
	err := f.Finalize(context.Background(), game, task, responses)
	require.NoError(t, err)

	var winners map[string]string
	require.NoError(t, json.Unmarshal(capturedResult.ResultData, &winners))
	assert.Equal(t, "smoothie", winners["drink"], "smoothie should win with 3 votes")
	assert.Equal(t, "pop", winners["music"], "pop should win unanimously")
}

// TestCollageFinalizer_TieBraking verifies that the first option in YAML order wins on a tie.
func TestCollageFinalizer_TieBraking(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	sender := &mockSender{}

	task := testCollageTask()
	game := &entity.Game{ID: 1, ChatID: 100}

	// 1 vote smoothie, 1 vote cappuccino → tie; first in YAML (smoothie) wins.
	responses := []*entity.TaskResponse{
		makeCollageResponse(map[string]string{"drink": "smoothie", "music": "pop"}),
		makeCollageResponse(map[string]string{"drink": "cappuccino", "music": "pop"}),
	}

	var capturedResult entity.TaskResult
	taskResultRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, r *entity.TaskResult) error {
			capturedResult = *r
			return nil
		})

	f := finalize.NewCollageFinalizer(taskResultRepo, noopMedia{}, sender, zerolog.Nop())
	err := f.Finalize(context.Background(), game, task, responses)
	require.NoError(t, err)

	var winners map[string]string
	require.NoError(t, json.Unmarshal(capturedResult.ResultData, &winners))
	assert.Equal(t, "smoothie", winners["drink"], "first YAML option wins on tie")
}

// TestCollageFinalizer_TaskResultHasCorrectFields checks that the saved result has
// proper GameID and TaskID fields.
func TestCollageFinalizer_TaskResultHasCorrectFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	sender := &mockSender{}

	task := testCollageTask()
	game := &entity.Game{ID: 42, ChatID: 100}
	responses := []*entity.TaskResponse{
		makeCollageResponse(map[string]string{"drink": "smoothie", "music": "pop"}),
	}

	var capturedResult entity.TaskResult
	taskResultRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, r *entity.TaskResult) error {
			capturedResult = *r
			return nil
		})

	f := finalize.NewCollageFinalizer(taskResultRepo, noopMedia{}, sender, zerolog.Nop())
	err := f.Finalize(context.Background(), game, task, responses)
	require.NoError(t, err)

	assert.Equal(t, uint64(42), capturedResult.GameID)
	assert.Equal(t, "task_02", capturedResult.TaskID)
	assert.False(t, capturedResult.FinalizedAt.IsZero())
}

// TestCollageFinalizer_MessagesSent verifies the sequence of messages:
// pending_text, then photo, then document.
func TestCollageFinalizer_MessagesSent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	taskResultRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	sender := &mockSender{}

	task := testCollageTask()
	game := &entity.Game{ID: 1, ChatID: 100}
	responses := []*entity.TaskResponse{
		makeCollageResponse(map[string]string{"drink": "smoothie", "music": "pop"}),
	}

	f := finalize.NewCollageFinalizer(taskResultRepo, noopMedia{}, sender, zerolog.Nop())
	err := f.Finalize(context.Background(), game, task, responses)
	require.NoError(t, err)

	// pending_text (string) + photo (*tele.Photo) + doc (*tele.Document) = 3 messages
	require.Equal(t, 3, len(sender.sent))
	assert.Equal(t, task.Summary.PendingText, sender.sent[0])

	// Wait for the goroutine that schedules temp file cleanup to fire without panicking.
	time.Sleep(10 * time.Millisecond)
}
