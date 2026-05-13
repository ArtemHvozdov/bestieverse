package subtask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/media"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/logger"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/lock"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// VotingCollageHandler handles the multi-step voting subtask (task_02).
// Only one player can vote at a time — enforced by the exclusive lock.
type VotingCollageHandler struct {
	lockManager         *lock.Manager
	subtaskProgressRepo repository.SubtaskProgressRepository
	taskResponseRepo    repository.TaskResponseRepository
	playerStateRepo     repository.PlayerStateRepository
	media               media.Storage
	sender              Sender
	msgs                *config.Messages
	timings             *config.Timings
	log                 zerolog.Logger
}

func NewVotingCollageHandler(
	lockManager *lock.Manager,
	subtaskProgressRepo repository.SubtaskProgressRepository,
	taskResponseRepo repository.TaskResponseRepository,
	playerStateRepo repository.PlayerStateRepository,
	mediaStorage media.Storage,
	sender Sender,
	msgs *config.Messages,
	timings *config.Timings,
	log zerolog.Logger,
) *VotingCollageHandler {
	return &VotingCollageHandler{
		lockManager:         lockManager,
		subtaskProgressRepo: subtaskProgressRepo,
		taskResponseRepo:    taskResponseRepo,
		playerStateRepo:     playerStateRepo,
		media:               mediaStorage,
		sender:              sender,
		msgs:                msgs,
		timings:             timings,
		log:                 log,
	}
}

// HandleRequestAnswer is called when a player presses "Хочу відповісти" on task_02.
// Acquires the exclusive lock and sends the first voting category.
func (h *VotingCollageHandler) HandleRequestAnswer(
	ctx context.Context,
	game *entity.Game,
	player *entity.Player,
	task *config.Task,
) error {
	chat := &tele.Chat{ID: game.ChatID}
	mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)

	existing, err := h.taskResponseRepo.GetByPlayerAndTask(ctx, game.ID, player.ID, task.ID)
	if err != nil {
		return fmt.Errorf("subtask.voting_collage.HandleRequestAnswer: get response: %w", err)
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
		return fmt.Errorf("subtask.voting_collage.HandleRequestAnswer: acquire lock: %w", err)
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
		return fmt.Errorf("subtask.voting_collage.HandleRequestAnswer: get progress: %w", err)
	}
	if progress == nil {
		emptyAnswers, _ := json.Marshal(map[string]string{})
		progress = &entity.SubtaskProgress{
			GameID:        game.ID,
			PlayerID:      player.ID,
			TaskID:        task.ID,
			QuestionIndex: 0,
			AnswersData:   emptyAnswers,
		}
		if err := h.subtaskProgressRepo.Upsert(ctx, progress); err != nil {
			return fmt.Errorf("subtask.voting_collage.HandleRequestAnswer: create progress: %w", err)
		}
	}

	state := &entity.PlayerState{
		GameID:   game.ID,
		PlayerID: player.ID,
		State:    entity.PlayerStateAwaitingAnswer,
		TaskID:   task.ID,
	}
	if err := h.playerStateRepo.Upsert(ctx, state); err != nil {
		return fmt.Errorf("subtask.voting_collage.HandleRequestAnswer: upsert state: %w", err)
	}

	if err := h.sendCategory(chat, task, progress.QuestionIndex); err != nil {
		return fmt.Errorf("subtask.voting_collage.HandleRequestAnswer: send category: %w", err)
	}

	h.log.Info().
		Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
		Str("user", logger.UserValue(player.TelegramUserID, player.Username)).
		Str("task", task.ID).
		Msg("voting_collage: lock acquired, first category sent")

	return nil
}

// HandleCategoryChoice is called when a player selects an option in a voting category.
// categoryID and optionID are parsed from the callback data "categoryID:optionID".
// prevMsg is the message containing the category photo and buttons — deleted before sending the next category.
func (h *VotingCollageHandler) HandleCategoryChoice(
	ctx context.Context,
	game *entity.Game,
	player *entity.Player,
	task *config.Task,
	categoryID, optionID string,
	prevMsg *tele.Message,
) error {
	chat := &tele.Chat{ID: game.ChatID}
	mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)

	lockHolder, err := h.lockManager.TryAcquire(ctx, game.ID, task.ID, player.ID)
	if err != nil {
		return fmt.Errorf("subtask.voting_collage.HandleCategoryChoice: check lock: %w", err)
	}
	if !lockHolder {
		text, _ := formatter.RenderTemplate(h.msgs.SubtaskLocked, struct{ Mention string }{mention})
		msg, _ := h.sender.Send(chat, text, formatter.ParseMode)
		if msg != nil {
			deleteAfter(h.sender, msg, h.timings.DeleteMessageDelay)
		}
		return nil
	}

	if prevMsg != nil {
		_ = h.sender.Delete(prevMsg)
	}

	progress, err := h.subtaskProgressRepo.Get(ctx, game.ID, player.ID, task.ID)
	if err != nil {
		return fmt.Errorf("subtask.voting_collage.HandleCategoryChoice: get progress: %w", err)
	}
	if progress == nil {
		return fmt.Errorf("subtask.voting_collage.HandleCategoryChoice: no progress found for player %d", player.ID)
	}

	var answers map[string]string
	if err := json.Unmarshal(progress.AnswersData, &answers); err != nil {
		answers = make(map[string]string)
	}
	answers[categoryID] = optionID

	updated, _ := json.Marshal(answers)
	progress.AnswersData = updated
	progress.QuestionIndex++

	if err := h.subtaskProgressRepo.Upsert(ctx, progress); err != nil {
		return fmt.Errorf("subtask.voting_collage.HandleCategoryChoice: upsert progress: %w", err)
	}

	categories := task.Subtask.Categories
	if progress.QuestionIndex < len(categories) {
		if err := h.sendCategory(chat, task, progress.QuestionIndex); err != nil {
			return fmt.Errorf("subtask.voting_collage.HandleCategoryChoice: send next category: %w", err)
		}
		return nil
	}

	// All categories answered — finalize.
	responseData, _ := json.Marshal(answers)
	resp := &entity.TaskResponse{
		GameID:       game.ID,
		PlayerID:     player.ID,
		TaskID:       task.ID,
		Status:       entity.ResponseAnswered,
		ResponseData: responseData,
	}
	if err := h.taskResponseRepo.Create(ctx, resp); err != nil {
		return fmt.Errorf("subtask.voting_collage.HandleCategoryChoice: create response: %w", err)
	}

	if err := h.subtaskProgressRepo.Delete(ctx, game.ID, player.ID, task.ID); err != nil {
		h.log.Warn().Err(err).Msg("voting_collage: failed to delete progress")
	}

	if err := h.lockManager.Release(ctx, game.ID, task.ID); err != nil {
		h.log.Warn().Err(err).Msg("voting_collage: failed to release lock")
	}

	if err := h.playerStateRepo.SetIdle(ctx, game.ID, player.ID); err != nil {
		return fmt.Errorf("subtask.voting_collage.HandleCategoryChoice: set idle: %w", err)
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
		Msg("voting_collage: all categories answered")

	return nil
}

// sendCategory sends the voting category photo and inline keyboard at the given index.
func (h *VotingCollageHandler) sendCategory(chat *tele.Chat, task *config.Task, idx int) error {
	cat := task.Subtask.Categories[idx]
	kbd := buildCategoryKeyboard(task, idx)

	if photo, err := h.media.GetPhoto(cat.MediaFile); err == nil && photo != nil {
		photo.Caption = cat.HeaderText
		h.sender.Send(chat, photo, formatter.ParseMode, kbd) //nolint:errcheck
	} else {
		h.sender.Send(chat, cat.HeaderText, formatter.ParseMode, kbd) //nolint:errcheck
	}
	return nil
}

// buildCategoryKeyboard constructs the inline keyboard for a voting category.
// Callback data format: "categoryID:optionID" — routed via "\ftask02:choice".
func buildCategoryKeyboard(task *config.Task, catIdx int) *tele.ReplyMarkup {
	cat := task.Subtask.Categories[catIdx]
	kbd := &tele.ReplyMarkup{}
	buttons := make([]tele.Row, 0, len(cat.Options))
	for _, opt := range cat.Options {
		payload := cat.ID + ":" + opt.ID
		btn := kbd.Data(opt.Label, "task02_choice", payload)
		buttons = append(buttons, kbd.Row(btn))
	}
	kbd.Inline(buttons...)
	return kbd
}

// ParseCategoryChoice splits callback data "categoryID:optionID" into its parts.
func ParseCategoryChoice(data string) (categoryID, optionID string, err error) {
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid category choice data: %q", data)
	}
	return parts[0], parts[1], nil
}
