package subtask_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository/mocks"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task/subtask"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/lock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func testWhoIsWhoTask() *config.Task {
	return &config.Task{
		ID:    "task_04",
		Order: 4,
		Type:  "question_answer",
		Questions: []config.TaskQuestion{
			{ID: "q1", Text: "Хто найбільший оптиміст?"},
			{ID: "q2", Text: "Хто найкраще готує?"},
		},
		Summary: config.TaskSummary{
			Type:       "who_is_who_results",
			HeaderText: "Ось результати!",
		},
		Followup: []string{"{{.Mention}} дякую!"},
	}
}

func testPlayers() []*entity.Player {
	return []*entity.Player{
		{
			ID:             testPlayerID,
			GameID:         testGameID,
			TelegramUserID: 55,
			Username:       "testuser",
			FirstName:      "Test",
		},
		{
			ID:             20,
			GameID:         testGameID,
			TelegramUserID: 66,
			Username:       "seconduser",
			FirstName:      "Second",
		},
	}
}

func makeWhoIsWhoHandler(
	lockMgr *lock.Manager,
	progressRepo *mocks.MockSubtaskProgressRepository,
	taskResponseRepo *mocks.MockTaskResponseRepository,
	playerStateRepo *mocks.MockPlayerStateRepository,
	playerRepo *mocks.MockPlayerRepository,
	sender *testSender,
) *subtask.WhoIsWhoHandler {
	return subtask.NewWhoIsWhoHandler(
		lockMgr,
		progressRepo,
		taskResponseRepo,
		playerStateRepo,
		playerRepo,
		sender,
		testMsgs(),
		testTimings(),
		zerolog.Nop(),
	)
}

// ---- HandleRequestAnswer tests ----

func TestWhoIsWhoHandleRequestAnswer_LockFree_AcquiresAndSendsFirstQuestion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testWhoIsWhoTask()
	ctx := context.Background()

	// Not yet answered.
	taskResponseRepo.EXPECT().GetByPlayerAndTask(ctx, game.ID, player.ID, task.ID).Return(nil, nil)

	// Lock acquisition — success.
	lockRepo.EXPECT().ReleaseExpired(ctx).Return(nil)
	lockRepo.EXPECT().Acquire(ctx, game.ID, task.ID, player.ID, gomock.Any()).Return(nil)
	lockRepo.EXPECT().Get(ctx, game.ID, task.ID).Return(&entity.TaskLock{PlayerID: player.ID}, nil)

	// No existing progress → create new.
	progressRepo.EXPECT().Get(ctx, game.ID, player.ID, task.ID).Return(nil, nil)
	progressRepo.EXPECT().Upsert(ctx, gomock.Any()).Return(nil)

	// State upsert.
	playerStateRepo.EXPECT().Upsert(ctx, gomock.Any()).Return(nil)

	// Get players for the question keyboard.
	playerRepo.EXPECT().GetAllByGame(ctx, game.ID).Return(testPlayers(), nil)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeWhoIsWhoHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, playerRepo, sender)

	err := h.HandleRequestAnswer(ctx, game, player, task)
	require.NoError(t, err)
	// First question text + keyboard sent.
	assert.Equal(t, 1, len(sender.sent))
}

func TestWhoIsWhoHandleRequestAnswer_LockHeld_SendsLockedMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testWhoIsWhoTask()
	ctx := context.Background()

	taskResponseRepo.EXPECT().GetByPlayerAndTask(ctx, game.ID, player.ID, task.ID).Return(nil, nil)

	// Lock held by another player (ID 99).
	lockRepo.EXPECT().ReleaseExpired(ctx).Return(nil)
	lockRepo.EXPECT().Acquire(ctx, game.ID, task.ID, player.ID, gomock.Any()).Return(nil)
	lockRepo.EXPECT().Get(ctx, game.ID, task.ID).Return(&entity.TaskLock{PlayerID: 99}, nil)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeWhoIsWhoHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, playerRepo, sender)

	err := h.HandleRequestAnswer(ctx, game, player, task)
	require.NoError(t, err)
	// subtask_locked sent (then auto-deleted, but sender captures it).
	assert.Equal(t, 1, len(sender.sent))
}

func TestWhoIsWhoHandleRequestAnswer_AlreadyAnswered_SendsErrorMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testWhoIsWhoTask()
	ctx := context.Background()

	existing := &entity.TaskResponse{Status: entity.ResponseAnswered}
	taskResponseRepo.EXPECT().GetByPlayerAndTask(ctx, game.ID, player.ID, task.ID).Return(existing, nil)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeWhoIsWhoHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, playerRepo, sender)

	err := h.HandleRequestAnswer(ctx, game, player, task)
	require.NoError(t, err)
	assert.Equal(t, 1, len(sender.sent))
}

// ---- HandlePlayerChoice tests ----

func TestWhoIsWhoHandlePlayerChoice_Intermediate_UpdatesProgressAndSendsNext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testWhoIsWhoTask()
	ctx := context.Background()

	// Lock still held by this player.
	lockRepo.EXPECT().ReleaseExpired(ctx).Return(nil)
	lockRepo.EXPECT().Acquire(ctx, game.ID, task.ID, player.ID, gomock.Any()).Return(nil)
	lockRepo.EXPECT().Get(ctx, game.ID, task.ID).Return(&entity.TaskLock{PlayerID: player.ID}, nil)

	initialAnswers, _ := json.Marshal(map[string]int64{})
	progress := &entity.SubtaskProgress{
		GameID:        game.ID,
		PlayerID:      player.ID,
		TaskID:        task.ID,
		QuestionIndex: 0,
		AnswersData:   initialAnswers,
	}
	progressRepo.EXPECT().Get(ctx, game.ID, player.ID, task.ID).Return(progress, nil)
	progressRepo.EXPECT().Upsert(ctx, gomock.Any()).Return(nil)

	// Get players for the next question keyboard.
	playerRepo.EXPECT().GetAllByGame(ctx, game.ID).Return(testPlayers(), nil)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeWhoIsWhoHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, playerRepo, sender)

	// Choose first question (q1 → player 66). QuestionIndex becomes 1, still < 2.
	err := h.HandlePlayerChoice(ctx, game, player, task, "q1", 66)
	require.NoError(t, err)
	// Next question sent.
	assert.Equal(t, 1, len(sender.sent))
}

func TestWhoIsWhoHandlePlayerChoice_LastQuestion_FinalizesAndSetsIdle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testWhoIsWhoTask()
	ctx := context.Background()

	// Lock held by this player.
	lockRepo.EXPECT().ReleaseExpired(ctx).Return(nil)
	lockRepo.EXPECT().Acquire(ctx, game.ID, task.ID, player.ID, gomock.Any()).Return(nil)
	lockRepo.EXPECT().Get(ctx, game.ID, task.ID).Return(&entity.TaskLock{PlayerID: player.ID}, nil)

	// Player is on the last question (QuestionIndex = 1, len(questions) = 2).
	existingAnswers, _ := json.Marshal(map[string]int64{"q1": 66})
	progress := &entity.SubtaskProgress{
		GameID:        game.ID,
		PlayerID:      player.ID,
		TaskID:        task.ID,
		QuestionIndex: 1,
		AnswersData:   existingAnswers,
	}
	progressRepo.EXPECT().Get(ctx, game.ID, player.ID, task.ID).Return(progress, nil)
	progressRepo.EXPECT().Upsert(ctx, gomock.Any()).Return(nil)

	// After last choice: create response, delete progress, release lock, set idle.
	taskResponseRepo.EXPECT().Create(ctx, gomock.Any()).Return(nil)
	progressRepo.EXPECT().Delete(ctx, game.ID, player.ID, task.ID).Return(nil)
	lockRepo.EXPECT().Release(ctx, game.ID, task.ID).Return(nil)
	playerStateRepo.EXPECT().SetIdle(ctx, game.ID, player.ID).Return(nil)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeWhoIsWhoHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, playerRepo, sender)

	err := h.HandlePlayerChoice(ctx, game, player, task, "q2", 55)
	require.NoError(t, err)
	// Follow-up message sent.
	assert.Equal(t, 1, len(sender.sent))
}

func TestWhoIsWhoHandlePlayerChoice_LockHeldByOther_EarlyExit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	playerRepo := mocks.NewMockPlayerRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testWhoIsWhoTask()
	ctx := context.Background()

	// Lock held by another player.
	lockRepo.EXPECT().ReleaseExpired(ctx).Return(nil)
	lockRepo.EXPECT().Acquire(ctx, game.ID, task.ID, player.ID, gomock.Any()).Return(nil)
	lockRepo.EXPECT().Get(ctx, game.ID, task.ID).Return(&entity.TaskLock{PlayerID: 99}, nil)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeWhoIsWhoHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, playerRepo, sender)

	err := h.HandlePlayerChoice(ctx, game, player, task, "q1", 66)
	require.NoError(t, err)
	// subtask_locked sent; nothing else changed.
	assert.Equal(t, 1, len(sender.sent))
}

// ---- ParsePlayerChoice tests ----

func TestWhoIsWhoParsePlayerChoice_Valid(t *testing.T) {
	questionID, uid, err := subtask.ParsePlayerChoice("q1:123456")
	require.NoError(t, err)
	assert.Equal(t, "q1", questionID)
	assert.Equal(t, int64(123456), uid)
}

func TestWhoIsWhoParsePlayerChoice_Invalid(t *testing.T) {
	_, _, err := subtask.ParsePlayerChoice("nodot")
	assert.Error(t, err)
}
