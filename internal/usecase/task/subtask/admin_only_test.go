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
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	tele "gopkg.in/telebot.v3"
)

// ---- test doubles ----

type mockImageGenerator struct {
	imageBytes []byte
	err        error
}

func (m *mockImageGenerator) GenerateCollage(_ context.Context, _ string) ([]byte, error) {
	return m.imageBytes, m.err
}

// ---- helpers ----

func testAdminTask() *config.Task {
	return &config.Task{
		ID:    "task_12",
		Order: 12,
		Type:  "admin_only",
		Questions: []config.TaskQuestion{
			{ID: "city", Text: "Куди поїдемо?", ButtonLabel: "Поділитися мрією"},
			{ID: "concert", Text: "Який концерт?", ButtonLabel: "Поділитися мрією"},
		},
		OpenAI: &config.TaskOpenAI{
			PromptTemplate: "Місто: {{index .Answers \"city\"}}",
		},
		Summary: config.TaskSummary{
			Type:        "openai_collage",
			SendingText: "Генеруємо...",
			ReadyText:   "Готово!",
		},
	}
}

func testAdminMsgs() *config.Messages {
	return &config.Messages{
		Task12OnlyAdmin:      "{{.Mention}} тільки адмін",
		Task12AwaitingAnswer: []string{"Пиши!"},
		Task12Reply:          []string{"Дякую!"},
		AlreadyAnswered:      []string{"{{.Mention}} вже відповів"},
	}
}

func testAdminGame() *entity.Game {
	return &entity.Game{
		ID:          testGameID,
		ChatID:      testChatID,
		Status:      entity.GameActive,
		AdminUserID: 55, // same as testPlayer().TelegramUserID
	}
}

func testNonAdminPlayer() *entity.Player {
	return &entity.Player{
		ID:             20,
		GameID:         testGameID,
		TelegramUserID: 99, // different from admin
		Username:       "other",
		FirstName:      "Other",
	}
}

func makeAdminHandler(
	progressRepo *mocks.MockSubtaskProgressRepository,
	taskResponseRepo *mocks.MockTaskResponseRepository,
	taskResultRepo *mocks.MockTaskResultRepository,
	playerStateRepo *mocks.MockPlayerStateRepository,
	gen *mockImageGenerator,
	sender *testSender,
) *subtask.AdminOnlyHandler {
	return subtask.NewAdminOnlyHandler(
		progressRepo,
		taskResponseRepo,
		taskResultRepo,
		playerStateRepo,
		gen,
		sender,
		testAdminMsgs(),
		&config.Timings{DeleteMessageDelay: time.Millisecond},
		zerolog.Nop(),
	)
}

// ---- HandleRequestAnswer tests ----

func TestAdminHandleRequestAnswer_NonAdmin_SendsDismissal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	// Non-admin player should not trigger any DB writes
	progressRepo.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	taskResponseRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Times(0)

	h := makeAdminHandler(progressRepo, taskResponseRepo, taskResultRepo, playerStateRepo, nil, sender)

	game := testAdminGame()
	nonAdmin := testNonAdminPlayer()
	task := testAdminTask()

	err := h.HandleRequestAnswer(context.Background(), game, nonAdmin, task)
	require.NoError(t, err)

	// A dismissal message should be sent and scheduled for deletion
	assert.Len(t, sender.sent, 1)
	time.Sleep(5 * time.Millisecond)
	assert.Equal(t, 1, sender.deleted)
}

func TestAdminHandleRequestAnswer_Admin_SendsFirstQuestion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	player := testPlayer() // TelegramUserID=55 matches AdminUserID=55

	taskResponseRepo.EXPECT().
		GetByPlayerAndTask(gomock.Any(), testGameID, testPlayerID, "task_12").
		Return(nil, nil)

	progressRepo.EXPECT().
		Get(gomock.Any(), testGameID, testPlayerID, "task_12").
		Return(nil, nil)

	playerStateRepo.EXPECT().
		Upsert(gomock.Any(), gomock.Any()).
		Return(nil)

	// First question send + progress save
	progressRepo.EXPECT().
		Upsert(gomock.Any(), gomock.Any()).
		Return(nil)

	h := makeAdminHandler(progressRepo, taskResponseRepo, taskResultRepo, playerStateRepo, nil, sender)

	game := testAdminGame()
	task := testAdminTask()

	err := h.HandleRequestAnswer(context.Background(), game, player, task)
	require.NoError(t, err)

	// One message sent: the first question with button
	assert.Len(t, sender.sent, 1)
	assert.Equal(t, 0, sender.deleted)
}

func TestAdminHandleRequestAnswer_AlreadyAnswered_SendsDismissal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	player := testPlayer()

	taskResponseRepo.EXPECT().
		GetByPlayerAndTask(gomock.Any(), testGameID, testPlayerID, "task_12").
		Return(&entity.TaskResponse{Status: entity.ResponseAnswered}, nil)

	// No progress or state changes expected
	progressRepo.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	playerStateRepo.EXPECT().Upsert(gomock.Any(), gomock.Any()).Times(0)

	h := makeAdminHandler(progressRepo, taskResponseRepo, taskResultRepo, playerStateRepo, nil, sender)

	game := testAdminGame()
	task := testAdminTask()

	err := h.HandleRequestAnswer(context.Background(), game, player, task)
	require.NoError(t, err)
	assert.Len(t, sender.sent, 1)
	time.Sleep(5 * time.Millisecond)
	assert.Equal(t, 1, sender.deleted)
}

// ---- HandleButtonPress tests ----

func TestAdminHandleButtonPress_SendsAwaitingAnswer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	h := makeAdminHandler(progressRepo, taskResponseRepo, taskResultRepo, playerStateRepo, nil, sender)

	game := testAdminGame()
	player := testPlayer()
	task := testAdminTask()

	err := h.HandleButtonPress(context.Background(), game, player, task, "city")
	require.NoError(t, err)

	// Sends awaiting_answer, does NOT delete it
	assert.Len(t, sender.sent, 1)
	assert.Equal(t, 0, sender.deleted)
}

// ---- HandleAnswer tests ----

func TestAdminHandleAnswer_IntermediateQuestion_SendsNext(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	player := testPlayer()

	initialPD, _ := json.Marshal(map[string]interface{}{
		"answers":  map[string]string{},
		"q_msg_id": 100,
	})
	progress := &entity.SubtaskProgress{
		GameID:        testGameID,
		PlayerID:      testPlayerID,
		TaskID:        "task_12",
		QuestionIndex: 0,
		AnswersData:   initialPD,
	}

	progressRepo.EXPECT().
		Get(gomock.Any(), testGameID, testPlayerID, "task_12").
		Return(progress, nil)

	// Save progress after advancing to next question
	progressRepo.EXPECT().
		Upsert(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, p *entity.SubtaskProgress) error {
			assert.Equal(t, 1, p.QuestionIndex)
			return nil
		})

	h := makeAdminHandler(progressRepo, taskResponseRepo, taskResultRepo, playerStateRepo, nil, sender)

	game := testAdminGame()
	task := testAdminTask()

	msg := &tele.Message{Text: "Київ"}
	err := h.HandleAnswer(context.Background(), game, player, task, msg)
	require.NoError(t, err)

	// Deleted question msg + reply + next question = 2 sends, 1 delete
	assert.Equal(t, 1, sender.deleted) // question message deleted
	assert.Len(t, sender.sent, 2)      // reply + next question
}

func TestAdminHandleAnswer_LastQuestion_CallsOpenAIAndSavesResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	progressRepo := mocks.NewMockSubtaskProgressRepository(ctrl)
	taskResponseRepo := mocks.NewMockTaskResponseRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	playerStateRepo := mocks.NewMockPlayerStateRepository(ctrl)
	sender := &testSender{}

	player := testPlayer()

	// Progress at last question (index=1, only 2 questions)
	initialPD, _ := json.Marshal(map[string]interface{}{
		"answers":  map[string]string{"city": "Київ"},
		"q_msg_id": 200,
	})
	progress := &entity.SubtaskProgress{
		GameID:        testGameID,
		PlayerID:      testPlayerID,
		TaskID:        "task_12",
		QuestionIndex: 1,
		AnswersData:   initialPD,
	}

	progressRepo.EXPECT().
		Get(gomock.Any(), testGameID, testPlayerID, "task_12").
		Return(progress, nil)

	var capturedResponse *entity.TaskResponse
	taskResponseRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, r *entity.TaskResponse) error {
			capturedResponse = r
			return nil
		})

	taskResultRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(nil)

	progressRepo.EXPECT().
		Delete(gomock.Any(), testGameID, testPlayerID, "task_12").
		Return(nil)

	playerStateRepo.EXPECT().
		SetIdle(gomock.Any(), testGameID, testPlayerID).
		Return(nil)

	gen := &mockImageGenerator{imageBytes: []byte("fake-png-data")}
	h := makeAdminHandler(progressRepo, taskResponseRepo, taskResultRepo, playerStateRepo, gen, sender)

	game := testAdminGame()
	task := testAdminTask()

	msg := &tele.Message{Text: "The Beatles"}
	err := h.HandleAnswer(context.Background(), game, player, task, msg)
	require.NoError(t, err)

	// Response created with correct status
	require.NotNil(t, capturedResponse)
	assert.Equal(t, entity.ResponseAnswered, capturedResponse.Status)
	assert.Equal(t, "task_12", capturedResponse.TaskID)

	// Response data contains both answers
	var respData map[string]interface{}
	require.NoError(t, json.Unmarshal(capturedResponse.ResponseData, &respData))
	answers, ok := respData["answers"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Київ", answers["city"])
	assert.Equal(t, "The Beatles", answers["concert"])

	// Messages: delete question + reply + sending_text + photo
	assert.Equal(t, 1, sender.deleted)
	assert.GreaterOrEqual(t, len(sender.sent), 2)
}
