package subtask

import (
	"context"
	"fmt"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/logger"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// PollHandler handles the lifecycle of a poll_then_task (task_10).
// It is invoked when the Telegram poll closes, determines the winner,
// clears active_poll_id, and publishes the corresponding follow-up task.
// task_result is NOT created here — FinalizeRouter/TextFinalizer does that later.
type PollHandler struct {
	gameRepo repository.GameRepository
	sender   Sender
	cfg      *config.Config
	log      zerolog.Logger
}

func NewPollHandler(
	gameRepo repository.GameRepository,
	sender Sender,
	cfg *config.Config,
	log zerolog.Logger,
) *PollHandler {
	return &PollHandler{
		gameRepo: gameRepo,
		sender:   sender,
		cfg:      cfg,
		log:      log,
	}
}

// HandlePollClosed processes a closed Telegram poll: determines the winner,
// clears active_poll_id, and publishes the follow-up task.
// task_result is intentionally NOT saved here so that FinalizeRouter can later
// run TextFinalizer and send the task summary.
// Uses ClaimActivePoll for atomic idempotency to prevent double-publish when
// ForceClosed (scheduler) and HandlePollClosed (OnPoll event) race each other.
func (h *PollHandler) HandlePollClosed(ctx context.Context, poll *tele.Poll) error {
	game, err := h.gameRepo.GetByActivePollID(ctx, poll.ID)
	if err != nil {
		return fmt.Errorf("subtask/poll.HandlePollClosed: get game: %w", err)
	}
	if game == nil {
		h.log.Warn().Str("poll_id", poll.ID).Msg("poll closed: no active game found — already processed or poll_id not stored")
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

	// Atomic claim: only one of HandlePollClosed/ForceClosed publishes the follow-up.
	claimed, err := h.gameRepo.ClaimActivePoll(ctx, game.ID, poll.ID)
	if err != nil {
		return fmt.Errorf("subtask/poll.HandlePollClosed: claim poll: %w", err)
	}
	if !claimed {
		h.log.Debug().
			Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
			Uint64("game", game.ID).
			Msg("poll already claimed by scheduler, skipping OnPoll follow-up")
		return nil
	}

	h.log.Info().
		Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
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
		kbd := buildMemeVoiceoverKeyboard(task.ID)
		if _, err := h.sender.Send(chat, h.cfg.Messages.MemeVoiceoverAnnounce, formatter.ParseMode, kbd); err != nil {
			return fmt.Errorf("subtask/poll.publishFollowUp: send meme announce: %w", err)
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

// ForceClosed is called by the scheduler after PollDuration has elapsed.
// It explicitly stops the Telegram poll to obtain final vote counts, determines the
// winner, and publishes the follow-up task.
// Uses ClaimActivePoll to prevent double-publish when HandlePollClosed (triggered by
// the UpdatePoll event that StopPoll fires) races this method.
func (h *PollHandler) ForceClosed(ctx context.Context, game *entity.Game) error {
	pollMsg := &tele.Message{ID: int(game.PollMessageID), Chat: &tele.Chat{ID: game.ChatID}}
	closedPoll, err := h.sender.StopPoll(pollMsg)
	if err != nil {
		// Poll was already closed (e.g., by a concurrent HandlePollClosed call).
		// OnPoll handler will/did publish the follow-up.
		h.log.Info().
			Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
			Uint64("game", game.ID).
			Err(err).
			Msg("force-close skipped: poll already stopped (OnPoll handler handles follow-up)")
		return nil
	}

	task := h.cfg.TaskByOrder(game.CurrentTaskOrder)
	if task == nil || task.Poll == nil {
		return fmt.Errorf("subtask/poll.ForceClosed: task %d not found or has no poll config", game.CurrentTaskOrder)
	}

	winner := determineWinner(closedPoll.Options, task.Poll.Options)
	if winner == nil {
		return fmt.Errorf("subtask/poll.ForceClosed: no poll options configured for task %s", task.ID)
	}

	// Atomic claim: prevents double-publish if HandlePollClosed received the
	// UpdatePoll event (triggered by StopPoll above) at the same time.
	claimed, err := h.gameRepo.ClaimActivePoll(ctx, game.ID, game.ActivePollID)
	if err != nil {
		return fmt.Errorf("subtask/poll.ForceClosed: claim poll: %w", err)
	}
	if !claimed {
		h.log.Debug().
			Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
			Uint64("game", game.ID).
			Msg("poll claimed by OnPoll handler concurrently, skipping scheduler follow-up")
		return nil
	}

	h.log.Info().
		Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
		Uint64("game", game.ID).
		Str("task", task.ID).
		Str("winner", winner.ID).
		Msg("poll force-closed by scheduler, winner determined")

	return h.publishFollowUp(ctx, game, task, winner)
}

// buildPollTaskKeyboard constructs the inline keyboard for a poll follow-up task.
func buildPollTaskKeyboard(taskID string) *tele.ReplyMarkup {
	kbd := &tele.ReplyMarkup{}
	answer := kbd.Data("Хочу відповісти ✍️", "task_request", taskID)
	skip := kbd.Data("Пропустити ⏭️", "task_skip", taskID)
	kbd.Inline(kbd.Row(answer, skip))
	return kbd
}

// buildMemeVoiceoverKeyboard constructs the keyboard for starting the meme voiceover subtask.
// Uses the same button labels as regular tasks ("Хочу відповісти" / "Пропустити") for UX consistency.
// The "Хочу відповісти" button keeps the task10_meme_request unique so it routes to
// MemeVoiceoverHandler.HandleRequestAnswer rather than the generic RequestAnswerer.
func buildMemeVoiceoverKeyboard(taskID string) *tele.ReplyMarkup {
	kbd := &tele.ReplyMarkup{}
	answer := kbd.Data("Хочу відповісти ✍️", "task10_meme_request")
	skip := kbd.Data("Пропустити ⏭️", "task_skip", taskID)
	kbd.Inline(kbd.Row(answer, skip))
	return kbd
}
