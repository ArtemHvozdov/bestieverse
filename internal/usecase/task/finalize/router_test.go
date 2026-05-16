package finalize_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository/mocks"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task/finalize"
	"github.com/rs/zerolog"
	"go.uber.org/mock/gomock"
)

// stubFinalizer is a test double for TaskFinalizer.
type stubFinalizer struct {
	summaryType string
	called      bool
	returnErr   error
}

func (s *stubFinalizer) SupportedSummaryType() string { return s.summaryType }
func (s *stubFinalizer) Finalize(_ context.Context, _ *entity.Game, _ *config.Task, _ []*entity.TaskResponse) error {
	s.called = true
	return s.returnErr
}

func makeRouter(
	ctrl *gomock.Controller,
	taskResponseRepo *mocks.MockTaskResponseRepository,
	taskResultRepo *mocks.MockTaskResultRepository,
	gameRepo *mocks.MockGameRepository,
	sender *mockSender,
	tasks []config.Task,
	finalizers ...finalize.TaskFinalizer,
) *finalize.FinalizeRouter {
	cfg := &config.Config{
		Tasks: tasks,
		Messages: config.Messages{
			NaAnswers: []string{"no answers"},
		},
		Game: config.GameMessages{
			FinalMessage1: "Final 1",
			FinalMessage2: "Final 2",
		},
		Timings: config.Timings{TaskInfoInterval: time.Millisecond},
	}
	log := zerolog.Nop()
	return finalize.NewFinalizeRouter(taskResponseRepo, taskResultRepo, gameRepo, sender, noopMedia{}, cfg, log, finalizers...)
}

func TestRouter_NoResponses_SendsNaAnswers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	gameRepo := mocks.NewMockGameRepository(ctrl)
	sender := &mockSender{}
	stub := &stubFinalizer{summaryType: "text"}

	game := &entity.Game{ID: 1, ChatID: 100}
	task := &config.Task{ID: "task_01", Order: 1, Summary: config.TaskSummary{Type: "text"}}

	taskResultRepo.EXPECT().GetByTask(gomock.Any(), game.ID, task.ID).Return(nil, nil)
	taskResponseRepo.EXPECT().GetAllByTask(gomock.Any(), game.ID, task.ID).Return(nil, nil)
	taskResultRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	router := makeRouter(ctrl, taskResponseRepo, taskResultRepo, gameRepo, sender,
		[]config.Task{{Order: 1}, {Order: 2}}, stub)

	err := router.Finalize(context.Background(), game, task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sender.sent))
	}
	if stub.called {
		t.Error("finalizer should not be called when no responses")
	}
}

func TestRouter_DispatchesToFinalizer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	gameRepo := mocks.NewMockGameRepository(ctrl)
	sender := &mockSender{}
	stub := &stubFinalizer{summaryType: "text"}

	game := &entity.Game{ID: 1, ChatID: 100}
	task := &config.Task{ID: "task_01", Order: 1, Summary: config.TaskSummary{Type: "text"}}
	responses := []*entity.TaskResponse{{ID: 1}}

	taskResultRepo.EXPECT().GetByTask(gomock.Any(), game.ID, task.ID).Return(nil, nil)
	taskResponseRepo.EXPECT().GetAllByTask(gomock.Any(), game.ID, task.ID).Return(responses, nil)

	router := makeRouter(ctrl, taskResponseRepo, taskResultRepo, gameRepo, sender,
		[]config.Task{{Order: 1}, {Order: 2}}, stub)

	err := router.Finalize(context.Background(), game, task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stub.called {
		t.Error("finalizer should have been called")
	}
}

func TestRouter_UnknownSummaryType_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	gameRepo := mocks.NewMockGameRepository(ctrl)
	sender := &mockSender{}

	game := &entity.Game{ID: 1, ChatID: 100}
	task := &config.Task{ID: "task_01", Order: 1, Summary: config.TaskSummary{Type: "unknown_type"}}
	responses := []*entity.TaskResponse{{ID: 1}}

	taskResultRepo.EXPECT().GetByTask(gomock.Any(), game.ID, task.ID).Return(nil, nil)
	taskResponseRepo.EXPECT().GetAllByTask(gomock.Any(), game.ID, task.ID).Return(responses, nil)

	router := makeRouter(ctrl, taskResponseRepo, taskResultRepo, gameRepo, sender, []config.Task{{Order: 1}, {Order: 2}})

	err := router.Finalize(context.Background(), game, task)
	if err == nil {
		t.Fatal("expected error for unknown summary type")
	}
	if len(sender.sent) != 0 {
		t.Error("should not send message on unknown type")
	}
}

func TestRouter_LastTask_CallsSetFinished(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	gameRepo := mocks.NewMockGameRepository(ctrl)
	sender := &mockSender{}
	stub := &stubFinalizer{summaryType: "text"}

	game := &entity.Game{ID: 1, ChatID: 100}
	// Order 2 is the last task (no task with order 3 exists).
	task := &config.Task{ID: "task_02", Order: 2, Summary: config.TaskSummary{Type: "text"}}
	responses := []*entity.TaskResponse{{ID: 1}}

	taskResultRepo.EXPECT().GetByTask(gomock.Any(), game.ID, task.ID).Return(nil, nil)
	taskResponseRepo.EXPECT().GetAllByTask(gomock.Any(), game.ID, task.ID).Return(responses, nil)
	gameRepo.EXPECT().SetFinished(gomock.Any(), game.ID).Return(nil)

	router := makeRouter(ctrl, taskResponseRepo, taskResultRepo, gameRepo, sender,
		[]config.Task{{Order: 1}, {Order: 2}}, stub)

	err := router.Finalize(context.Background(), game, task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRouter_NotLastTask_DoesNotCallSetFinished(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	gameRepo := mocks.NewMockGameRepository(ctrl)
	sender := &mockSender{}
	stub := &stubFinalizer{summaryType: "text"}

	game := &entity.Game{ID: 1, ChatID: 100}
	task := &config.Task{ID: "task_01", Order: 1, Summary: config.TaskSummary{Type: "text"}}
	responses := []*entity.TaskResponse{{ID: 1}}

	taskResultRepo.EXPECT().GetByTask(gomock.Any(), game.ID, task.ID).Return(nil, nil)
	taskResponseRepo.EXPECT().GetAllByTask(gomock.Any(), game.ID, task.ID).Return(responses, nil)
	// gameRepo.SetFinished should NOT be called.

	router := makeRouter(ctrl, taskResponseRepo, taskResultRepo, gameRepo, sender,
		[]config.Task{{Order: 1}, {Order: 2}}, stub)

	err := router.Finalize(context.Background(), game, task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRouter_FinalizerError_Propagates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	gameRepo := mocks.NewMockGameRepository(ctrl)
	sender := &mockSender{}
	stub := &stubFinalizer{summaryType: "text", returnErr: errors.New("boom")}

	game := &entity.Game{ID: 1, ChatID: 100}
	task := &config.Task{ID: "task_01", Order: 1, Summary: config.TaskSummary{Type: "text"}}
	responses := []*entity.TaskResponse{{ID: 1}}

	taskResultRepo.EXPECT().GetByTask(gomock.Any(), game.ID, task.ID).Return(nil, nil)
	taskResponseRepo.EXPECT().GetAllByTask(gomock.Any(), game.ID, task.ID).Return(responses, nil)

	router := makeRouter(ctrl, taskResponseRepo, taskResultRepo, gameRepo, sender,
		[]config.Task{{Order: 1}, {Order: 2}}, stub)

	err := router.Finalize(context.Background(), game, task)
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}

func TestRouter_AlreadyFinalized_Skips(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	gameRepo := mocks.NewMockGameRepository(ctrl)
	sender := &mockSender{}
	stub := &stubFinalizer{summaryType: "text"}

	game := &entity.Game{ID: 1, ChatID: 100}
	task := &config.Task{ID: "task_01", Order: 1, Summary: config.TaskSummary{Type: "text"}}

	// Existing result — task already finalized.
	existing := &entity.TaskResult{ID: 1, GameID: game.ID, TaskID: task.ID}
	taskResultRepo.EXPECT().GetByTask(gomock.Any(), game.ID, task.ID).Return(existing, nil)
	// GetAllByTask and finalizer.Finalize must NOT be called.

	router := makeRouter(ctrl, taskResponseRepo, taskResultRepo, gameRepo, sender,
		[]config.Task{{Order: 1}, {Order: 2}}, stub)

	err := router.Finalize(context.Background(), game, task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.called {
		t.Error("finalizer should not be called for already-finalized task")
	}
	if len(sender.sent) != 0 {
		t.Error("no message should be sent for already-finalized task")
	}
}
