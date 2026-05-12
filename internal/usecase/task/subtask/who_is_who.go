package subtask

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/logger"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/lock"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// WhoIsWhoHandler handles the multi-step player-selection subtask (task_04).
// Only one player can answer at a time — enforced by the exclusive lock.
type WhoIsWhoHandler struct {
	lockManager         *lock.Manager
	subtaskProgressRepo repository.SubtaskProgressRepository
	taskResponseRepo    repository.TaskResponseRepository
	playerStateRepo     repository.PlayerStateRepository
	playerRepo          repository.PlayerRepository
	sender              Sender
	msgs                *config.Messages
	timings             *config.Timings
	log                 zerolog.Logger
}

func NewWhoIsWhoHandler(
	lockManager *lock.Manager,
	subtaskProgressRepo repository.SubtaskProgressRepository,
	taskResponseRepo repository.TaskResponseRepository,
	playerStateRepo repository.PlayerStateRepository,
	playerRepo repository.PlayerRepository,
	sender Sender,
	msgs *config.Messages,
	timings *config.Timings,
	log zerolog.Logger,
) *WhoIsWhoHandler {
	return &WhoIsWhoHandler{
		lockManager:         lockManager,
		subtaskProgressRepo: subtaskProgressRepo,
		taskResponseRepo:    taskResponseRepo,
		playerStateRepo:     playerStateRepo,
		playerRepo:          playerRepo,
		sender:              sender,
		msgs:                msgs,
		timings:             timings,
		log:                 log,
	}
}

// HandleRequestAnswer is called when a player presses "Хочу відповісти" on task_04.
// Acquires the exclusive lock and sends the first question with player selection buttons.
func (h *WhoIsWhoHandler) HandleRequestAnswer(
	ctx context.Context,
	game *entity.Game,
	player *entity.Player,
	task *config.Task,
) error {
	chat := &tele.Chat{ID: game.ChatID}
	mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)

	existing, err := h.taskResponseRepo.GetByPlayerAndTask(ctx, game.ID, player.ID, task.ID)
	if err != nil {
		return fmt.Errorf("subtask.who_is_who.HandleRequestAnswer: get response: %w", err)
	}
	if existing != nil {
		text, _ := formatter.RenderTemplate(config.Random(h.msgs.AlreadyAnswered), struct{ Mention string }{mention})
		msg, _ := h.sender.Send(chat, text, formatter.ParseMode)
		if msg != nil {
			deleteAfter(h.sender, msg, h.timings.DeleteMessageDelay)
		}
		return nil
	}

	acquired, err := h.lockManager.TryAcquire(ctx, game.ID, task.ID, player.ID)
	if err != nil {
		return fmt.Errorf("subtask.who_is_who.HandleRequestAnswer: acquire lock: %w", err)
	}
	if !acquired {
		text, _ := formatter.RenderTemplate(h.msgs.SubtaskLocked, struct{ Mention string }{mention})
		msg, _ := h.sender.Send(chat, text, formatter.ParseMode)
		if msg != nil {
			deleteAfter(h.sender, msg, h.timings.DeleteMessageDelay)
		}
		return nil
	}

	progress, err := h.subtaskProgressRepo.Get(ctx, game.ID, player.ID, task.ID)
	if err != nil {
		return fmt.Errorf("subtask.who_is_who.HandleRequestAnswer: get progress: %w", err)
	}
	if progress == nil {
		emptyAnswers, _ := json.Marshal(map[string]int64{})
		progress = &entity.SubtaskProgress{
			GameID:        game.ID,
			PlayerID:      player.ID,
			TaskID:        task.ID,
			QuestionIndex: 0,
			AnswersData:   emptyAnswers,
		}
		if err := h.subtaskProgressRepo.Upsert(ctx, progress); err != nil {
			return fmt.Errorf("subtask.who_is_who.HandleRequestAnswer: create progress: %w", err)
		}
	}

	state := &entity.PlayerState{
		GameID:   game.ID,
		PlayerID: player.ID,
		State:    entity.PlayerStateAwaitingAnswer,
		TaskID:   task.ID,
	}
	if err := h.playerStateRepo.Upsert(ctx, state); err != nil {
		return fmt.Errorf("subtask.who_is_who.HandleRequestAnswer: upsert state: %w", err)
	}

	players, err := h.playerRepo.GetAllByGame(ctx, game.ID)
	if err != nil {
		return fmt.Errorf("subtask.who_is_who.HandleRequestAnswer: get players: %w", err)
	}

	if err := h.sendQuestion(chat, task, players, 0); err != nil {
		return fmt.Errorf("subtask.who_is_who.HandleRequestAnswer: send question: %w", err)
	}

	h.log.Info().
		Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
		Str("user", logger.UserValue(player.TelegramUserID, player.Username)).
		Str("task", task.ID).
		Msg("who_is_who: lock acquired, first question sent")

	return nil
}

// HandlePlayerChoice is called when a player selects a participant for the current question.
// questionID and chosenTelegramUserID are parsed from the callback data "questionID:telegramUserID".
func (h *WhoIsWhoHandler) HandlePlayerChoice(
	ctx context.Context,
	game *entity.Game,
	player *entity.Player,
	task *config.Task,
	questionID string,
	chosenTelegramUserID int64,
) error {
	chat := &tele.Chat{ID: game.ChatID}
	mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)

	lockHolder, err := h.lockManager.TryAcquire(ctx, game.ID, task.ID, player.ID)
	if err != nil {
		return fmt.Errorf("subtask.who_is_who.HandlePlayerChoice: check lock: %w", err)
	}
	if !lockHolder {
		text, _ := formatter.RenderTemplate(h.msgs.SubtaskLocked, struct{ Mention string }{mention})
		msg, _ := h.sender.Send(chat, text, formatter.ParseMode)
		if msg != nil {
			deleteAfter(h.sender, msg, h.timings.DeleteMessageDelay)
		}
		return nil
	}

	progress, err := h.subtaskProgressRepo.Get(ctx, game.ID, player.ID, task.ID)
	if err != nil {
		return fmt.Errorf("subtask.who_is_who.HandlePlayerChoice: get progress: %w", err)
	}
	if progress == nil {
		return fmt.Errorf("subtask.who_is_who.HandlePlayerChoice: no progress for player %d", player.ID)
	}

	var answers map[string]int64
	if err := json.Unmarshal(progress.AnswersData, &answers); err != nil {
		answers = make(map[string]int64)
	}
	answers[questionID] = chosenTelegramUserID

	updated, _ := json.Marshal(answers)
	progress.AnswersData = updated
	progress.QuestionIndex++

	if err := h.subtaskProgressRepo.Upsert(ctx, progress); err != nil {
		return fmt.Errorf("subtask.who_is_who.HandlePlayerChoice: upsert progress: %w", err)
	}

	if progress.QuestionIndex < len(task.Questions) {
		players, err := h.playerRepo.GetAllByGame(ctx, game.ID)
		if err != nil {
			return fmt.Errorf("subtask.who_is_who.HandlePlayerChoice: get players: %w", err)
		}
		if err := h.sendQuestion(chat, task, players, progress.QuestionIndex); err != nil {
			return fmt.Errorf("subtask.who_is_who.HandlePlayerChoice: send next question: %w", err)
		}
		return nil
	}

	// All questions answered — save response, release lock, set idle.
	responseData, _ := json.Marshal(answers)
	resp := &entity.TaskResponse{
		GameID:       game.ID,
		PlayerID:     player.ID,
		TaskID:       task.ID,
		Status:       entity.ResponseAnswered,
		ResponseData: responseData,
	}
	if err := h.taskResponseRepo.Create(ctx, resp); err != nil {
		return fmt.Errorf("subtask.who_is_who.HandlePlayerChoice: create response: %w", err)
	}

	if err := h.subtaskProgressRepo.Delete(ctx, game.ID, player.ID, task.ID); err != nil {
		h.log.Warn().Err(err).Msg("who_is_who: failed to delete progress")
	}

	if err := h.lockManager.Release(ctx, game.ID, task.ID); err != nil {
		h.log.Warn().Err(err).Msg("who_is_who: failed to release lock")
	}

	if err := h.playerStateRepo.SetIdle(ctx, game.ID, player.ID); err != nil {
		return fmt.Errorf("subtask.who_is_who.HandlePlayerChoice: set idle: %w", err)
	}

	followup := config.Random(task.Followup)
	if followup != "" {
		text, renderErr := formatter.RenderTemplate(followup, struct{ Mention string }{mention})
		if renderErr != nil {
			text = followup
		}
		h.sender.Send(chat, text, formatter.ParseMode) //nolint:errcheck
	}

	h.log.Info().
		Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
		Str("user", logger.UserValue(player.TelegramUserID, player.Username)).
		Str("task", task.ID).
		Msg("who_is_who: all questions answered")

	return nil
}

// sendQuestion sends the question text and inline keyboard with all players at the given index.
func (h *WhoIsWhoHandler) sendQuestion(chat *tele.Chat, task *config.Task, players []*entity.Player, idx int) error {
	question := task.Questions[idx]
	kbd := buildPlayerSelectionKeyboard(players, question.ID)
	h.sender.Send(chat, question.Text, formatter.ParseMode, kbd) //nolint:errcheck
	return nil
}

// buildPlayerSelectionKeyboard constructs the inline keyboard for player selection.
// Callback data format: "questionID:telegramUserID" — routed via "\ftask04:player".
func buildPlayerSelectionKeyboard(players []*entity.Player, questionID string) *tele.ReplyMarkup {
	kbd := &tele.ReplyMarkup{}
	buttons := make([]tele.Row, 0, len(players))
	for _, p := range players {
		label := p.FirstName
		if p.Username != "" {
			label = "@" + p.Username
		}
		payload := questionID + ":" + strconv.FormatInt(p.TelegramUserID, 10)
		btn := kbd.Data(label, "task04_player", payload)
		buttons = append(buttons, kbd.Row(btn))
	}
	kbd.Inline(buttons...)
	return kbd
}

// ParsePlayerChoice splits callback data "questionID:telegramUserID" into its parts.
func ParsePlayerChoice(data string) (questionID string, chosenTelegramUserID int64, err error) {
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid player choice data: %q", data)
	}
	uid, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid telegram user id in choice data %q: %w", data, err)
	}
	return parts[0], uid, nil
}
