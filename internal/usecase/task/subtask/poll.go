package subtask

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// PollHandler handles the lifecycle of a poll_then_task (task_10).
// It is invoked when the Telegram poll closes, determines the winner,
// saves the result, and publishes the corresponding follow-up task.
type PollHandler struct {
	gameRepo       repository.GameRepository
	taskResultRepo repository.TaskResultRepository
	sender         Sender
	cfg            *config.Config
	log            zerolog.Logger
}

func NewPollHandler(
	gameRepo repository.GameRepository,
	taskResultRepo repository.TaskResultRepository,
	sender Sender,
	cfg *config.Config,
	log zerolog.Logger,
) *PollHandler {
	return &PollHandler{
		gameRepo:       gameRepo,
		taskResultRepo: taskResultRepo,
		sender:         sender,
		cfg:            cfg,
		log:            log,
	}
}

// HandlePollClosed processes a closed Telegram poll: determines the winner,
// saves task_result, clears active_poll_id, and publishes the follow-up.
func (h *PollHandler) HandlePollClosed(ctx context.Context, poll *tele.Poll) error {
	game, err := h.gameRepo.GetByActivePollID(ctx, poll.ID)
	if err != nil {
		return fmt.Errorf("subtask/poll.HandlePollClosed: get game: %w", err)
	}
	if game == nil {
		h.log.Debug().Str("poll_id", poll.ID).Msg("poll closed: no active game found")
		return nil
	}

	task := h.cfg.TaskByOrder(game.CurrentTaskOrder)
	if task == nil || task.Poll == nil {
		return fmt.Errorf("subtask/poll.HandlePollClosed: task %d not found or has no poll config", game.CurrentTaskOrder)
	}

	winner := determineWinner(poll.Options, task.Poll.Options)
	if winner == nil {
		return fmt.Errorf("subtask/poll.HandlePollClosed: no poll options configured for task %s", task.ID)
	}

	resultData, err := json.Marshal(map[string]string{"winning_option": winner.ID})
	if err != nil {
		return fmt.Errorf("subtask/poll.HandlePollClosed: marshal result: %w", err)
	}
	if err := h.taskResultRepo.Create(ctx, &entity.TaskResult{
		GameID:     game.ID,
		TaskID:     task.ID,
		ResultData: resultData,
	}); err != nil {
		return fmt.Errorf("subtask/poll.HandlePollClosed: save result: %w", err)
	}

	if err := h.gameRepo.SetActivePollID(ctx, game.ID, ""); err != nil {
		return fmt.Errorf("subtask/poll.HandlePollClosed: clear active poll id: %w", err)
	}

	h.log.Info().
		Int64("chat", game.ChatID).
		Uint64("game", game.ID).
		Str("task", task.ID).
		Str("winner", winner.ID).
		Msg("poll closed, winner determined")

	return h.publishFollowUp(ctx, game, task, winner)
}

// publishFollowUp sends the follow-up content for the winning poll option.
func (h *PollHandler) publishFollowUp(_ context.Context, game *entity.Game, task *config.Task, winner *config.PollOption) error {
	chat := &tele.Chat{ID: game.ChatID}

	switch winner.ResultType {
	case "question_answer":
		kbd := buildPollTaskKeyboard(task.ID)
		if _, err := h.sender.Send(chat, winner.PreparedText, formatter.ParseMode, kbd); err != nil {
			return fmt.Errorf("subtask/poll.publishFollowUp: send: %w", err)
		}
	case "meme_voiceover":
		// Stub: full meme voiceover flow implemented in Stage 10.
		if _, err := h.sender.Send(chat, "🎬 Виграли мемаси! Готуйтесь до озвучки... 🎤", formatter.ParseMode); err != nil {
			return fmt.Errorf("subtask/poll.publishFollowUp: send meme stub: %w", err)
		}
	default:
		return fmt.Errorf("subtask/poll.publishFollowUp: unknown result_type %q for option %s", winner.ResultType, winner.ID)
	}
	return nil
}

// determineWinner returns the winning poll option.
// Highest VoterCount wins; on a tie or all-zero votes, first option in YAML order wins.
func determineWinner(pollResults []tele.PollOption, configOptions []config.PollOption) *config.PollOption {
	if len(configOptions) == 0 {
		return nil
	}
	winnerIdx := 0
	maxVotes := -1
	for i, r := range pollResults {
		if i >= len(configOptions) {
			break
		}
		if r.VoterCount > maxVotes {
			maxVotes = r.VoterCount
			winnerIdx = i
		}
	}
	return &configOptions[winnerIdx]
}

// buildPollTaskKeyboard constructs the inline keyboard for a poll follow-up task.
func buildPollTaskKeyboard(taskID string) *tele.ReplyMarkup {
	kbd := &tele.ReplyMarkup{}
	answer := kbd.Data("Хочу відповісти ✍️", "task:request", taskID)
	skip := kbd.Data("Пропустити ⏭️", "task:skip", taskID)
	kbd.Inline(kbd.Row(answer, skip))
	return kbd
}
