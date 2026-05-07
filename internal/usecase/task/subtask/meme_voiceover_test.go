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
	tele "gopkg.in/telebot.v3"
)

// testMemeTask returns a task_10 config with two meme files for concise testing.
func testMemeTask() *config.Task {
	return &config.Task{
		ID:    "task_10",
		Order: 10,
		Type:  "poll_then_task",
		Poll: &config.SubtaskPoll{
			Title: "Голосування",
			Options: []config.PollOption{
				{ID: "dance", Label: "Танець", ResultType: "question_answer"},
				{
					ID:         "memes",
					Label:      "Мемаси",
					ResultType: "meme_voiceover",
					MemeFiles:  []string{"task_10/meme_01.gif", "task_10/meme_02.gif"},
				},
			},
		},
	}
}

func testMemeMsgs() *config.Messages {
	return &config.Messages{
		SubtaskLocked:     "{{.Mention}} зачекайте",
		AlreadyAnswered:   []string{"{{.Mention}} вже відповів"},
		MemeVoiceoverDone: []string{"{{.Mention}} чудово!"},
	}
}

func makeMemeHandler(
	lockMgr *lock.Manager,
	progressRepo *mocks.MockSubtaskProgressRepository,
	taskResponseRepo *mocks.MockTaskResponseRepository,
	playerStateRepo *mocks.MockPlayerStateRepository,
	sender *testSender,
) *subtask.MemeVoiceoverHandler {
	return subtask.NewMemeVoiceoverHandler(
		lockMgr,
		progressRepo,
		taskResponseRepo,
		playerStateRepo,
		testMedia{},
		sender,
		testMemeMsgs(),
		&config.Timings{DeleteMessageDelay: time.Millisecond},
		zerolog.Nop(),
	)
}

// ---- HandleRequestAnswer tests ----

func TestMemeHandleRequestAnswer_LockFree_AcquiresAndSendsFirstMeme(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testMemeTask()
	ctx := context.Background()

	taskResponseRepo.EXPECT().GetByPlayerAndTask(ctx, game.ID, player.ID, task.ID).Return(nil, nil)

	lockRepo.EXPECT().ReleaseExpired(ctx).Return(nil)
	lockRepo.EXPECT().Acquire(ctx, game.ID, task.ID, player.ID, gomock.Any()).Return(nil)
	lockRepo.EXPECT().Get(ctx, game.ID, task.ID).Return(&entity.TaskLock{PlayerID: player.ID}, nil)

	progressRepo.EXPECT().Get(ctx, game.ID, player.ID, task.ID).Return(nil, nil)
	progressRepo.EXPECT().Upsert(ctx, gomock.Any()).Return(nil)

	playerStateRepo.EXPECT().Upsert(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, s *entity.PlayerState) error {
			assert.Equal(t, "task_10:meme", s.TaskID)
			return nil
		},
	)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeMemeHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, sender)

	err := h.HandleRequestAnswer(ctx, game, player, task)
	require.NoError(t, err)
	// First meme sent.
	assert.Equal(t, 1, len(sender.sent))
}

func TestMemeHandleRequestAnswer_LockHeldByOther_SendsLockedMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testMemeTask()
	ctx := context.Background()

	taskResponseRepo.EXPECT().GetByPlayerAndTask(ctx, game.ID, player.ID, task.ID).Return(nil, nil)

	lockRepo.EXPECT().ReleaseExpired(ctx).Return(nil)
	lockRepo.EXPECT().Acquire(ctx, game.ID, task.ID, player.ID, gomock.Any()).Return(nil)
	lockRepo.EXPECT().Get(ctx, game.ID, task.ID).Return(&entity.TaskLock{PlayerID: 99}, nil)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeMemeHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, sender)

	err := h.HandleRequestAnswer(ctx, game, player, task)
	require.NoError(t, err)
	// subtask_locked sent; no progress created.
	assert.Equal(t, 1, len(sender.sent))
}

func TestMemeHandleRequestAnswer_AlreadyAnswered_SendsErrorMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testMemeTask()
	ctx := context.Background()

	existing := &entity.TaskResponse{Status: entity.ResponseAnswered}
	taskResponseRepo.EXPECT().GetByPlayerAndTask(ctx, game.ID, player.ID, task.ID).Return(existing, nil)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeMemeHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, sender)

	err := h.HandleRequestAnswer(ctx, game, player, task)
	require.NoError(t, err)
	assert.Equal(t, 1, len(sender.sent))
}

// ---- HandleAnswer tests ----

func TestMemeHandleAnswer_Intermediate_SendsNextMemeWithoutDeleting(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testMemeTask()
	ctx := context.Background()

	// Lock held by this player.
	lockRepo.EXPECT().ReleaseExpired(ctx).Return(nil)
	lockRepo.EXPECT().Acquire(ctx, game.ID, task.ID, player.ID, gomock.Any()).Return(nil)
	lockRepo.EXPECT().Get(ctx, game.ID, task.ID).Return(&entity.TaskLock{PlayerID: player.ID}, nil)

	// On first meme (index 0).
	existingAnswers, _ := json.Marshal(map[string]string{})
	progress := &entity.SubtaskProgress{
		GameID:        game.ID,
		PlayerID:      player.ID,
		TaskID:        task.ID,
		QuestionIndex: 0,
		AnswersData:   existingAnswers,
	}
	progressRepo.EXPECT().Get(ctx, game.ID, player.ID, task.ID).Return(progress, nil)
	progressRepo.EXPECT().Upsert(ctx, gomock.Any()).Return(nil)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeMemeHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, sender)

	msg := &tele.Message{Text: "Привіт, я озвучую!"}
	err := h.HandleAnswer(ctx, game, player, task, msg)
	require.NoError(t, err)
	// Next meme sent; nothing deleted (deleted == 0).
	assert.Equal(t, 1, len(sender.sent))
	assert.Equal(t, 0, sender.deleted)
}

func TestMemeHandleAnswer_LastMeme_FinalizesAndSendsDoneMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testMemeTask()
	ctx := context.Background()

	// Lock held by this player.
	lockRepo.EXPECT().ReleaseExpired(ctx).Return(nil)
	lockRepo.EXPECT().Acquire(ctx, game.ID, task.ID, player.ID, gomock.Any()).Return(nil)
	lockRepo.EXPECT().Get(ctx, game.ID, task.ID).Return(&entity.TaskLock{PlayerID: player.ID}, nil)

	// On last meme (index 1, total 2 memes → QuestionIndex becomes 2 which equals len).
	existingAnswers, _ := json.Marshal(map[string]string{"meme_1": "озвучка першого"})
	progress := &entity.SubtaskProgress{
		GameID:        game.ID,
		PlayerID:      player.ID,
		TaskID:        task.ID,
		QuestionIndex: 1,
		AnswersData:   existingAnswers,
	}
	progressRepo.EXPECT().Get(ctx, game.ID, player.ID, task.ID).Return(progress, nil)
	progressRepo.EXPECT().Upsert(ctx, gomock.Any()).Return(nil)

	taskResponseRepo.EXPECT().Create(ctx, gomock.Any()).DoAndReturn(
		func(_ context.Context, r *entity.TaskResponse) error {
			assert.Equal(t, entity.ResponseAnswered, r.Status)
			assert.Equal(t, task.ID, r.TaskID)
			// Response data must include both meme answers.
			var data map[string]string
			require.NoError(t, json.Unmarshal(r.ResponseData, &data))
			assert.Contains(t, data, "meme_1")
			assert.Contains(t, data, "meme_2")
			return nil
		},
	)
	progressRepo.EXPECT().Delete(ctx, game.ID, player.ID, task.ID).Return(nil)
	lockRepo.EXPECT().Release(ctx, game.ID, task.ID).Return(nil)
	playerStateRepo.EXPECT().SetIdle(ctx, game.ID, player.ID).Return(nil)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeMemeHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, sender)

	msg := &tele.Message{Text: "Озвучка другого мема!"}
	err := h.HandleAnswer(ctx, game, player, task, msg)
	require.NoError(t, err)
	// Final "done" message sent.
	assert.Equal(t, 1, len(sender.sent))
	assert.Equal(t, 0, sender.deleted)
}

func TestMemeHandleAnswer_NonTextMessage_Accepted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testMemeTask()
	ctx := context.Background()

	lockRepo.EXPECT().ReleaseExpired(ctx).Return(nil)
	lockRepo.EXPECT().Acquire(ctx, game.ID, task.ID, player.ID, gomock.Any()).Return(nil)
	lockRepo.EXPECT().Get(ctx, game.ID, task.ID).Return(&entity.TaskLock{PlayerID: player.ID}, nil)

	existingAnswers, _ := json.Marshal(map[string]string{})
	progress := &entity.SubtaskProgress{
		GameID: game.ID, PlayerID: player.ID, TaskID: task.ID,
		QuestionIndex: 0, AnswersData: existingAnswers,
	}
	progressRepo.EXPECT().Get(ctx, game.ID, player.ID, task.ID).Return(progress, nil)
	progressRepo.EXPECT().Upsert(ctx, gomock.Any()).Return(nil)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeMemeHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, sender)

	// Photo message — Text is empty.
	msg := &tele.Message{Photo: &tele.Photo{}}
	err := h.HandleAnswer(ctx, game, player, task, msg)
	require.NoError(t, err)
	// Next meme sent — non-text accepted, no error.
	assert.Equal(t, 1, len(sender.sent))
}

func TestMemeHandleAnswer_LockHeldByOther_EarlyExit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testMemeTask()
	ctx := context.Background()

	lockRepo.EXPECT().ReleaseExpired(ctx).Return(nil)
	lockRepo.EXPECT().Acquire(ctx, game.ID, task.ID, player.ID, gomock.Any()).Return(nil)
	lockRepo.EXPECT().Get(ctx, game.ID, task.ID).Return(&entity.TaskLock{PlayerID: 99}, nil)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeMemeHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, sender)

	msg := &tele.Message{Text: "озвучка"}
	err := h.HandleAnswer(ctx, game, player, task, msg)
	require.NoError(t, err)
	// Nothing sent; nothing changed.
	assert.Equal(t, 0, len(sender.sent))
}
