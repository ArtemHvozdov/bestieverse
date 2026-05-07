package subtask_test

import (
	"context"
	"encoding/json"
	"testing"

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

// ---- helpers ----

func makePollConfig() *config.SubtaskPoll {
	return &config.SubtaskPoll{
		Title: "Голосування",
		Options: []config.PollOption{
			{ID: "dance", Label: "Танцювати", ResultType: "question_answer", PreparedText: "Завдання: танок"},
			{ID: "sing", Label: "Співати", ResultType: "question_answer", PreparedText: "Завдання: спів"},
			{ID: "memes", Label: "Мемаси", ResultType: "meme_voiceover"},
		},
	}
}

func makePollTask() *config.Task {
	return &config.Task{
		ID:    "task_10",
		Order: 10,
		Type:  "poll_then_task",
		Poll:  makePollConfig(),
	}
}

func makePollTestGame() *entity.Game {
	return &entity.Game{
		ID:               1,
		ChatID:           100,
		Status:           entity.GameActive,
		CurrentTaskOrder: 10,
	}
}

func newPollHandler(t *testing.T, ctrl *gomock.Controller, sender *testSender, cfg *config.Config) (*subtask.PollHandler, *mocks.MockGameRepository, *mocks.MockTaskResultRepository) {
	t.Helper()
	gameRepo := mocks.NewMockGameRepository(ctrl)
	taskResultRepo := mocks.NewMockTaskResultRepository(ctrl)
	h := subtask.NewPollHandler(gameRepo, taskResultRepo, sender, cfg, zerolog.Nop())
	return h, gameRepo, taskResultRepo
}

func pollTestConfig() *config.Config {
	task := makePollTask()
	return &config.Config{Tasks: []config.Task{*task}}
}

// ---- tests ----

func TestPollHandler_WinnerByHighestVotes(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &testSender{}
	cfg := pollTestConfig()
	h, gameRepo, taskResultRepo := newPollHandler(t, ctrl, sender, cfg)

	game := makePollTestGame()
	// dance gets 3, sing gets 1 → dance wins
	poll := &tele.Poll{
		ID:     "poll123",
		Closed: true,
		Options: []tele.PollOption{
			{Text: "Танцювати", VoterCount: 3},
			{Text: "Співати", VoterCount: 1},
			{Text: "Мемаси", VoterCount: 0},
		},
	}

	gameRepo.EXPECT().GetByActivePollID(gomock.Any(), "poll123").Return(game, nil)
	taskResultRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.TaskResult) error {
		var result map[string]string
		require.NoError(t, json.Unmarshal(r.ResultData, &result))
		assert.Equal(t, "dance", result["winning_option"])
		return nil
	})
	gameRepo.EXPECT().SetActivePollID(gomock.Any(), game.ID, "").Return(nil)

	err := h.HandlePollClosed(context.Background(), poll)
	require.NoError(t, err)

	require.Len(t, sender.sent, 1)
	assert.Contains(t, sender.sent[0], "Завдання: танок")
}

func TestPollHandler_TieWinnerIsFirst(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &testSender{}
	cfg := pollTestConfig()
	h, gameRepo, taskResultRepo := newPollHandler(t, ctrl, sender, cfg)

	game := makePollTestGame()
	// dance=2, sing=2 → first option (dance) wins
	poll := &tele.Poll{
		ID:     "poll123",
		Closed: true,
		Options: []tele.PollOption{
			{Text: "Танцювати", VoterCount: 2},
			{Text: "Співати", VoterCount: 2},
			{Text: "Мемаси", VoterCount: 0},
		},
	}

	gameRepo.EXPECT().GetByActivePollID(gomock.Any(), "poll123").Return(game, nil)
	taskResultRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.TaskResult) error {
		var result map[string]string
		require.NoError(t, json.Unmarshal(r.ResultData, &result))
		assert.Equal(t, "dance", result["winning_option"])
		return nil
	})
	gameRepo.EXPECT().SetActivePollID(gomock.Any(), game.ID, "").Return(nil)

	err := h.HandlePollClosed(context.Background(), poll)
	require.NoError(t, err)
}

func TestPollHandler_AllZeroVotesFirstWins(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &testSender{}
	cfg := pollTestConfig()
	h, gameRepo, taskResultRepo := newPollHandler(t, ctrl, sender, cfg)

	game := makePollTestGame()
	// all zero → first option wins
	poll := &tele.Poll{
		ID:     "poll123",
		Closed: true,
		Options: []tele.PollOption{
			{Text: "Танцювати", VoterCount: 0},
			{Text: "Співати", VoterCount: 0},
			{Text: "Мемаси", VoterCount: 0},
		},
	}

	gameRepo.EXPECT().GetByActivePollID(gomock.Any(), "poll123").Return(game, nil)
	taskResultRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.TaskResult) error {
		var result map[string]string
		require.NoError(t, json.Unmarshal(r.ResultData, &result))
		assert.Equal(t, "dance", result["winning_option"])
		return nil
	})
	gameRepo.EXPECT().SetActivePollID(gomock.Any(), game.ID, "").Return(nil)

	err := h.HandlePollClosed(context.Background(), poll)
	require.NoError(t, err)
}

func TestPollHandler_QuestionAnswerSendsPreparedText(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &testSender{}
	cfg := pollTestConfig()
	h, gameRepo, taskResultRepo := newPollHandler(t, ctrl, sender, cfg)

	game := makePollTestGame()
	poll := &tele.Poll{
		ID:     "poll456",
		Closed: true,
		Options: []tele.PollOption{
			{Text: "Танцювати", VoterCount: 5},
			{Text: "Співати", VoterCount: 1},
			{Text: "Мемаси", VoterCount: 0},
		},
	}

	gameRepo.EXPECT().GetByActivePollID(gomock.Any(), "poll456").Return(game, nil)
	taskResultRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	gameRepo.EXPECT().SetActivePollID(gomock.Any(), game.ID, "").Return(nil)

	err := h.HandlePollClosed(context.Background(), poll)
	require.NoError(t, err)

	// prepared_text and keyboard (2 send args) should be sent
	require.Len(t, sender.sent, 1)
	assert.Equal(t, "Завдання: танок", sender.sent[0])
}

func TestPollHandler_ActivePollIDClearedAfterProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &testSender{}
	cfg := pollTestConfig()
	h, gameRepo, taskResultRepo := newPollHandler(t, ctrl, sender, cfg)

	game := makePollTestGame()
	poll := &tele.Poll{
		ID:     "poll789",
		Closed: true,
		Options: []tele.PollOption{
			{Text: "Танцювати", VoterCount: 1},
			{Text: "Співати", VoterCount: 0},
			{Text: "Мемаси", VoterCount: 0},
		},
	}

	gameRepo.EXPECT().GetByActivePollID(gomock.Any(), "poll789").Return(game, nil)
	taskResultRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	// SetActivePollID called with empty string to clear the poll
	gameRepo.EXPECT().SetActivePollID(gomock.Any(), game.ID, "").Return(nil)

	err := h.HandlePollClosed(context.Background(), poll)
	require.NoError(t, err)
}

func TestPollHandler_NoGameFound_NoOp(t *testing.T) {
	ctrl := gomock.NewController(t)
	sender := &testSender{}
	cfg := pollTestConfig()
	h, gameRepo, _ := newPollHandler(t, ctrl, sender, cfg)

	poll := &tele.Poll{
		ID:     "unknown_poll",
		Closed: true,
	}

	gameRepo.EXPECT().GetByActivePollID(gomock.Any(), "unknown_poll").Return(nil, nil)

	err := h.HandlePollClosed(context.Background(), poll)
	require.NoError(t, err)
	assert.Empty(t, sender.sent)
}
