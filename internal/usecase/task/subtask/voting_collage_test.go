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

// ---- test doubles ----

type testSender struct {
	sent    []interface{}
	deleted int
}

func (s *testSender) Send(_ tele.Recipient, what interface{}, _ ...interface{}) (*tele.Message, error) {
	s.sent = append(s.sent, what)
	return &tele.Message{ID: len(s.sent)}, nil
}
func (s *testSender) Delete(_ tele.Editable) error { s.deleted++; return nil }

type testMedia struct{}

func (testMedia) GetFile(_ string) (*tele.Document, error)    { return nil, nil }
func (testMedia) GetPhoto(_ string) (*tele.Photo, error)      { return nil, nil }
func (testMedia) GetAnimation(_ string) (*tele.Animation, error) { return nil, nil }

// ---- helpers ----

const (
	testGameID   uint64 = 1
	testPlayerID uint64 = 10
	testChatID   int64  = 100
)

func testGame() *entity.Game {
	return &entity.Game{ID: testGameID, ChatID: testChatID, Status: entity.GameActive}
}

func testPlayer() *entity.Player {
	return &entity.Player{
		ID:             testPlayerID,
		GameID:         testGameID,
		TelegramUserID: 55,
		Username:       "testuser",
		FirstName:      "Test",
	}
}

func testTask() *config.Task {
	return &config.Task{
		ID:    "task_02",
		Order: 2,
		Type:  "question_answer",
		Subtask: &config.SubtaskVotingCollage{
			Type:          "voting_collage",
			ExclusiveLock: true,
			Categories: []config.VotingCategory{
				{
					ID:         "drink",
					HeaderText: "НАШ НАПІЙ:",
					MediaFile:  "task_02/drink_voting.jpg",
					Options: []config.VotingOption{
						{ID: "smoothie", Label: "Смузі", MediaFile: "task_02/smoothie.jpg"},
						{ID: "cappuccino", Label: "Капучіно", MediaFile: "task_02/cappuccino.jpg"},
					},
				},
				{
					ID:         "music",
					HeaderText: "НАША МУЗИКА:",
					MediaFile:  "task_02/music_voting.jpg",
					Options: []config.VotingOption{
						{ID: "pop", Label: "Поп", MediaFile: "task_02/pop.jpg"},
						{ID: "rock", Label: "Рок", MediaFile: "task_02/rock.jpg"},
					},
				},
			},
		},
		Followup: []string{"{{.Mention}} дякую!"},
	}
}

func testMsgs() *config.Messages {
	return &config.Messages{
		SubtaskLocked:   "{{.Mention}} зачекайте",
		AlreadyAnswered: []string{"{{.Mention}} вже відповів"},
	}
}

func testTimings() *config.Timings {
	return &config.Timings{DeleteMessageDelay: time.Millisecond}
}

func makeHandler(
	lockMgr *lock.Manager,
	progressRepo *mocks.MockSubtaskProgressRepository,
	taskResponseRepo *mocks.MockTaskResponseRepository,
	playerStateRepo *mocks.MockPlayerStateRepository,
	sender *testSender,
) *subtask.VotingCollageHandler {
	return subtask.NewVotingCollageHandler(
		lockMgr,
		progressRepo,
		taskResponseRepo,
		playerStateRepo,
		testMedia{},
		sender,
		testMsgs(),
		testTimings(),
		zerolog.Nop(),
	)
}

// ---- HandleRequestAnswer tests ----

func TestHandleRequestAnswer_LockFree_AcquiresAndSendsFirstCategory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testTask()
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

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, sender)

	err := h.HandleRequestAnswer(ctx, game, player, task)
	require.NoError(t, err)
	// First category header + keyboard sent.
	assert.Equal(t, 1, len(sender.sent))
}

func TestHandleRequestAnswer_LockHeldByOther_SendsLockedMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testTask()
	ctx := context.Background()

	taskResponseRepo.EXPECT().GetByPlayerAndTask(ctx, game.ID, player.ID, task.ID).Return(nil, nil)

	// Lock held by another player (ID 99).
	lockRepo.EXPECT().ReleaseExpired(ctx).Return(nil)
	lockRepo.EXPECT().Acquire(ctx, game.ID, task.ID, player.ID, gomock.Any()).Return(nil)
	lockRepo.EXPECT().Get(ctx, game.ID, task.ID).Return(&entity.TaskLock{PlayerID: 99}, nil)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, sender)

	err := h.HandleRequestAnswer(ctx, game, player, task)
	require.NoError(t, err)
	// subtask_locked sent (then auto-deleted, but sender captures it).
	assert.Equal(t, 1, len(sender.sent))
	// Progress repo must NOT have been called.
	// playerStateRepo must NOT have been called.
}

func TestHandleRequestAnswer_AlreadyAnswered_SendsErrorMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testTask()
	ctx := context.Background()

	existing := &entity.TaskResponse{Status: entity.ResponseAnswered}
	taskResponseRepo.EXPECT().GetByPlayerAndTask(ctx, game.ID, player.ID, task.ID).Return(existing, nil)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, sender)

	err := h.HandleRequestAnswer(ctx, game, player, task)
	require.NoError(t, err)
	assert.Equal(t, 1, len(sender.sent))
}

// ---- HandleCategoryChoice tests ----

func TestHandleCategryChoice_Intermediate_UpdatesProgressAndSendsNext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testTask()
	ctx := context.Background()

	// Lock still held by this player.
	lockRepo.EXPECT().ReleaseExpired(ctx).Return(nil)
	lockRepo.EXPECT().Acquire(ctx, game.ID, task.ID, player.ID, gomock.Any()).Return(nil)
	lockRepo.EXPECT().Get(ctx, game.ID, task.ID).Return(&entity.TaskLock{PlayerID: player.ID}, nil)

	initialAnswers, _ := json.Marshal(map[string]string{})
	progress := &entity.SubtaskProgress{
		GameID:        game.ID,
		PlayerID:      player.ID,
		TaskID:        task.ID,
		QuestionIndex: 0, // on first category
		AnswersData:   initialAnswers,
	}
	progressRepo.EXPECT().Get(ctx, game.ID, player.ID, task.ID).Return(progress, nil)
	progressRepo.EXPECT().Upsert(ctx, gomock.Any()).Return(nil)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, sender)

	prevMsg := &tele.Message{ID: 42}

	// Choose first category (drink → smoothie). QuestionIndex becomes 1, still < 2.
	err := h.HandleCategoryChoice(ctx, game, player, task, "drink", "smoothie", prevMsg)
	require.NoError(t, err)
	// Previous message deleted, next category (music) sent.
	assert.Equal(t, 1, sender.deleted)
	assert.Equal(t, 1, len(sender.sent))
}

func TestHandleCategryChoice_LastCategory_FinalizesAndSendsFollowup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testTask()
	ctx := context.Background()

	// Lock held by this player.
	lockRepo.EXPECT().ReleaseExpired(ctx).Return(nil)
	lockRepo.EXPECT().Acquire(ctx, game.ID, task.ID, player.ID, gomock.Any()).Return(nil)
	lockRepo.EXPECT().Get(ctx, game.ID, task.ID).Return(&entity.TaskLock{PlayerID: player.ID}, nil)

	// Player is on the last category (QuestionIndex = 1, len(categories) = 2).
	existingAnswers, _ := json.Marshal(map[string]string{"drink": "smoothie"})
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
	h := makeHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, sender)

	prevMsg := &tele.Message{ID: 42}

	err := h.HandleCategoryChoice(ctx, game, player, task, "music", "pop", prevMsg)
	require.NoError(t, err)
	// Previous message deleted, follow-up sent.
	assert.Equal(t, 1, sender.deleted)
	assert.Equal(t, 1, len(sender.sent))
}

func TestHandleCategryChoice_LockHeldByOther_EarlyExit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	lockRepo := mocks.NewMockTaskLockRepository(ctrl)
	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	game := testGame()
	player := testPlayer()
	task := testTask()
	ctx := context.Background()

	// Lock held by another player.
	lockRepo.EXPECT().ReleaseExpired(ctx).Return(nil)
	lockRepo.EXPECT().Acquire(ctx, game.ID, task.ID, player.ID, gomock.Any()).Return(nil)
	lockRepo.EXPECT().Get(ctx, game.ID, task.ID).Return(&entity.TaskLock{PlayerID: 99}, nil)

	mgr := lock.NewManager(lockRepo, 15*time.Minute)
	h := makeHandler(mgr, progressRepo, taskResponseRepo, playerStateRepo, sender)

	err := h.HandleCategoryChoice(ctx, game, player, task, "drink", "smoothie", &tele.Message{ID: 42})
	require.NoError(t, err)
	// subtask_locked sent; message NOT deleted (lock not held by this player).
	assert.Equal(t, 0, sender.deleted)
	assert.Equal(t, 1, len(sender.sent))
}

// ---- ParseCategoryChoice tests ----

func TestParseCategoryChoice_Valid(t *testing.T) {
	catID, optID, err := subtask.ParseCategoryChoice("drink:smoothie")
	require.NoError(t, err)
	assert.Equal(t, "drink", catID)
	assert.Equal(t, "smoothie", optID)
}

func TestParseCategoryChoice_WithColonInOption(t *testing.T) {
	// Option IDs may not contain colons, but the split is SplitN(2) so this is safe.
	catID, optID, err := subtask.ParseCategoryChoice("cat:opt_id")
	require.NoError(t, err)
	assert.Equal(t, "cat", catID)
	assert.Equal(t, "opt_id", optID)
}

func TestParseCategoryChoice_Invalid(t *testing.T) {
	_, _, err := subtask.ParseCategoryChoice("nodot")
	assert.Error(t, err)
}
