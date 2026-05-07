package subtask

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/media"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/lock"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// MemeVoiceoverHandler handles the sequential meme voiceover subtask (task_10b).
// Only one player can voice memes at a time — enforced by the exclusive lock.
// Player's voiceover answers are NOT deleted — they stay in chat as public content.
type MemeVoiceoverHandler struct {
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

func NewMemeVoiceoverHandler(
	lockManager *lock.Manager,
	subtaskProgressRepo repository.SubtaskProgressRepository,
	taskResponseRepo repository.TaskResponseRepository,
	playerStateRepo repository.PlayerStateRepository,
	mediaStorage media.Storage,
	sender Sender,
	msgs *config.Messages,
	timings *config.Timings,
	log zerolog.Logger,
) *MemeVoiceoverHandler {
	return &MemeVoiceoverHandler{
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

// HandleRequestAnswer is called when a player presses the meme voiceover start button.
// Acquires the exclusive lock and sends the first meme GIF without accompanying text.
func (h *MemeVoiceoverHandler) HandleRequestAnswer(
	ctx context.Context,
	game *entity.Game,
	player *entity.Player,
	task *config.Task,
) error {
	chat := &tele.Chat{ID: game.ChatID}
	mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)

	existing, err := h.taskResponseRepo.GetByPlayerAndTask(ctx, game.ID, player.ID, task.ID)
	if err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleRequestAnswer: get response: %w", err)
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
		return fmt.Errorf("subtask.meme_voiceover.HandleRequestAnswer: acquire lock: %w", err)
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
		return fmt.Errorf("subtask.meme_voiceover.HandleRequestAnswer: get progress: %w", err)
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
			return fmt.Errorf("subtask.meme_voiceover.HandleRequestAnswer: create progress: %w", err)
		}
	}

	// TaskID with ":meme" suffix routes message.go to this handler for all incoming messages.
	state := &entity.PlayerState{
		GameID:   game.ID,
		PlayerID: player.ID,
		State:    entity.PlayerStateAwaitingAnswer,
		TaskID:   task.ID + ":meme",
	}
	if err := h.playerStateRepo.Upsert(ctx, state); err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleRequestAnswer: upsert state: %w", err)
	}

	memeFiles := memeFilesFromTask(task)
	if err := h.sendMeme(chat, memeFiles[0]); err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleRequestAnswer: send first meme: %w", err)
	}

	h.log.Info().
		Int64("chat", game.ChatID).
		Int64("user", player.TelegramUserID).
		Str("task", task.ID).
		Msg("meme_voiceover: lock acquired, first meme sent")

	return nil
}

// HandleAnswer processes the player's voiceover for the current meme.
// The voiceover message is NOT deleted — it stays in chat as public content.
// Any message type is accepted (text, voice, video, photo, etc.).
func (h *MemeVoiceoverHandler) HandleAnswer(
	ctx context.Context,
	game *entity.Game,
	player *entity.Player,
	task *config.Task,
	msg *tele.Message,
) error {
	chat := &tele.Chat{ID: game.ChatID}
	mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)

	lockHolder, err := h.lockManager.TryAcquire(ctx, game.ID, task.ID, player.ID)
	if err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleAnswer: check lock: %w", err)
	}
	if !lockHolder {
		return nil
	}

	progress, err := h.subtaskProgressRepo.Get(ctx, game.ID, player.ID, task.ID)
	if err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleAnswer: get progress: %w", err)
	}
	if progress == nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleAnswer: no progress for player %d", player.ID)
	}

	var answers map[string]string
	if err := json.Unmarshal(progress.AnswersData, &answers); err != nil {
		answers = make(map[string]string)
	}

	memeKey := fmt.Sprintf("meme_%d", progress.QuestionIndex+1)
	answers[memeKey] = msg.Text
	updated, _ := json.Marshal(answers)
	progress.AnswersData = updated
	progress.QuestionIndex++

	if err := h.subtaskProgressRepo.Upsert(ctx, progress); err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleAnswer: upsert progress: %w", err)
	}

	memeFiles := memeFilesFromTask(task)
	if progress.QuestionIndex < len(memeFiles) {
		if err := h.sendMeme(chat, memeFiles[progress.QuestionIndex]); err != nil {
			return fmt.Errorf("subtask.meme_voiceover.HandleAnswer: send next meme: %w", err)
		}
		return nil
	}

	// All memes voiced — finalize.
	responseData, _ := json.Marshal(answers)
	resp := &entity.TaskResponse{
		GameID:       game.ID,
		PlayerID:     player.ID,
		TaskID:       task.ID,
		Status:       entity.ResponseAnswered,
		ResponseData: responseData,
	}
	if err := h.taskResponseRepo.Create(ctx, resp); err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleAnswer: create response: %w", err)
	}

	if err := h.subtaskProgressRepo.Delete(ctx, game.ID, player.ID, task.ID); err != nil {
		h.log.Warn().Err(err).Msg("meme_voiceover: failed to delete progress")
	}

	if err := h.lockManager.Release(ctx, game.ID, task.ID); err != nil {
		h.log.Warn().Err(err).Msg("meme_voiceover: failed to release lock")
	}

	if err := h.playerStateRepo.SetIdle(ctx, game.ID, player.ID); err != nil {
		return fmt.Errorf("subtask.meme_voiceover.HandleAnswer: set idle: %w", err)
	}

	text, renderErr := formatter.RenderTemplate(config.Random(h.msgs.MemeVoiceoverDone), struct{ Mention string }{mention})
	if renderErr != nil {
		text = config.Random(h.msgs.MemeVoiceoverDone)
	}
	h.sender.Send(chat, text, formatter.ParseMode) //nolint:errcheck

	h.log.Info().
		Int64("chat", game.ChatID).
		Int64("user", player.TelegramUserID).
		Str("task", task.ID).
		Msg("meme_voiceover: all memes voiced")

	return nil
}

// sendMeme sends a single meme GIF to the chat without any text.
func (h *MemeVoiceoverHandler) sendMeme(chat *tele.Chat, memeFile string) error {
	anim, err := h.media.GetAnimation(memeFile)
	if err != nil {
		return fmt.Errorf("get animation %s: %w", memeFile, err)
	}
	h.sender.Send(chat, anim) //nolint:errcheck
	return nil
}

// memeFilesFromTask extracts meme files from the task's poll option with result_type=meme_voiceover.
func memeFilesFromTask(task *config.Task) []string {
	if task.Poll == nil {
		return nil
	}
	for _, opt := range task.Poll.Options {
		if opt.ResultType == "meme_voiceover" {
			return opt.MemeFiles
		}
	}
	return nil
}
