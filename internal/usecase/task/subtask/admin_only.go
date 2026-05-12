package subtask

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/logger"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// ImageGenerator generates AI images from a text prompt.
// *openai.Client satisfies this interface.
type ImageGenerator interface {
	GenerateCollage(ctx context.Context, prompt string) ([]byte, error)
}

// adminProgressData holds persisted state between questions in task_12.
type adminProgressData struct {
	Answers       map[string]string `json:"answers"`
	QuestionMsgID int               `json:"q_msg_id"`
}

// AdminOnlyHandler manages the admin-only sequential questions subtask (task_12).
// Only the game admin can answer; produces an OpenAI-generated collage upon completion.
type AdminOnlyHandler struct {
	subtaskProgressRepo repository.SubtaskProgressRepository
	taskResponseRepo    repository.TaskResponseRepository
	taskResultRepo      repository.TaskResultRepository
	playerStateRepo     repository.PlayerStateRepository
	openai              ImageGenerator
	sender              Sender
	msgs                *config.Messages
	timings             *config.Timings
	log                 zerolog.Logger
}

func NewAdminOnlyHandler(
	subtaskProgressRepo repository.SubtaskProgressRepository,
	taskResponseRepo repository.TaskResponseRepository,
	taskResultRepo repository.TaskResultRepository,
	playerStateRepo repository.PlayerStateRepository,
	openai ImageGenerator,
	sender Sender,
	msgs *config.Messages,
	timings *config.Timings,
	log zerolog.Logger,
) *AdminOnlyHandler {
	return &AdminOnlyHandler{
		subtaskProgressRepo: subtaskProgressRepo,
		taskResponseRepo:    taskResponseRepo,
		taskResultRepo:      taskResultRepo,
		playerStateRepo:     playerStateRepo,
		openai:              openai,
		sender:              sender,
		msgs:                msgs,
		timings:             timings,
		log:                 log,
	}
}

// HandleRequestAnswer is called when a player presses "Хочу відповісти" on task_12.
// Non-admin players receive a dismissal message and the flow stops.
func (h *AdminOnlyHandler) HandleRequestAnswer(
	ctx context.Context,
	game *entity.Game,
	player *entity.Player,
	task *config.Task,
) error {
	chat := &tele.Chat{ID: game.ChatID}
	mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)

	if player.TelegramUserID != game.AdminUserID {
		text, _ := formatter.RenderTemplate(h.msgs.Task12OnlyAdmin, struct{ Mention string }{mention})
		msg, _ := h.sender.Send(chat, text, formatter.ParseMode)
		if msg != nil {
			deleteAfter(h.sender, msg, h.timings.DeleteMessageDelay)
		}
		return nil
	}

	existing, err := h.taskResponseRepo.GetByPlayerAndTask(ctx, game.ID, player.ID, task.ID)
	if err != nil {
		return fmt.Errorf("subtask.admin_only.HandleRequestAnswer: get response: %w", err)
	}
	if existing != nil {
		text, _ := formatter.RenderTemplate(config.Random(h.msgs.AlreadyAnswered), struct{ Mention string }{mention})
		msg, _ := h.sender.Send(chat, text, formatter.ParseMode)
		if msg != nil {
			deleteAfter(h.sender, msg, h.timings.DeleteMessageDelay)
		}
		return nil
	}

	progress, err := h.subtaskProgressRepo.Get(ctx, game.ID, player.ID, task.ID)
	if err != nil {
		return fmt.Errorf("subtask.admin_only.HandleRequestAnswer: get progress: %w", err)
	}
	if progress == nil {
		emptyData, _ := json.Marshal(adminProgressData{Answers: make(map[string]string)})
		progress = &entity.SubtaskProgress{
			GameID:        game.ID,
			PlayerID:      player.ID,
			TaskID:        task.ID,
			QuestionIndex: 0,
			AnswersData:   emptyData,
		}
	}

	state := &entity.PlayerState{
		GameID:   game.ID,
		PlayerID: player.ID,
		State:    entity.PlayerStateAwaitingAnswer,
		TaskID:   task.ID + ":admin",
	}
	if err := h.playerStateRepo.Upsert(ctx, state); err != nil {
		return fmt.Errorf("subtask.admin_only.HandleRequestAnswer: upsert state: %w", err)
	}

	msgID := h.sendQuestionMsg(chat, task, 0)
	pd := adminProgressData{Answers: make(map[string]string), QuestionMsgID: msgID}
	data, _ := json.Marshal(pd)
	progress.AnswersData = data
	progress.QuestionIndex = 0
	if err := h.subtaskProgressRepo.Upsert(ctx, progress); err != nil {
		return fmt.Errorf("subtask.admin_only.HandleRequestAnswer: save progress: %w", err)
	}

	h.log.Info().
		Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
		Str("user", logger.UserValue(player.TelegramUserID, player.Username)).
		Str("task", task.ID).
		Msg("admin_only: first question sent")

	return nil
}

// HandleButtonPress is called when the admin clicks the question button (button_label).
// Sends a "write your answer" prompt; does not delete it so the admin can read it.
func (h *AdminOnlyHandler) HandleButtonPress(
	_ context.Context,
	game *entity.Game,
	_ *entity.Player,
	_ *config.Task,
	_ string,
) error {
	chat := &tele.Chat{ID: game.ChatID}
	text := config.Random(h.msgs.Task12AwaitingAnswer)
	h.sender.Send(chat, text, formatter.ParseMode) //nolint:errcheck
	return nil
}

// HandleAnswer processes the admin's text response to the current question.
func (h *AdminOnlyHandler) HandleAnswer(
	ctx context.Context,
	game *entity.Game,
	player *entity.Player,
	task *config.Task,
	msg *tele.Message,
) error {
	chat := &tele.Chat{ID: game.ChatID}

	progress, err := h.subtaskProgressRepo.Get(ctx, game.ID, player.ID, task.ID)
	if err != nil {
		return fmt.Errorf("subtask.admin_only.HandleAnswer: get progress: %w", err)
	}
	if progress == nil {
		return fmt.Errorf("subtask.admin_only.HandleAnswer: no progress for player %d", player.ID)
	}

	var pd adminProgressData
	if err := json.Unmarshal(progress.AnswersData, &pd); err != nil {
		pd = adminProgressData{Answers: make(map[string]string)}
	}
	if pd.Answers == nil {
		pd.Answers = make(map[string]string)
	}

	if progress.QuestionIndex >= len(task.Questions) {
		return fmt.Errorf("subtask.admin_only.HandleAnswer: question index %d out of bounds", progress.QuestionIndex)
	}

	currentQ := task.Questions[progress.QuestionIndex]
	pd.Answers[currentQ.ID] = msg.Text

	// Delete the question message (admin-only content, not the answer)
	if pd.QuestionMsgID != 0 {
		h.sender.Delete(&tele.Message{ID: pd.QuestionMsgID, Chat: &tele.Chat{ID: game.ChatID}}) //nolint:errcheck
	}

	// Reply acknowledging the answer (keep in chat)
	replyText := config.Random(h.msgs.Task12Reply)
	h.sender.Send(chat, replyText, formatter.ParseMode) //nolint:errcheck

	progress.QuestionIndex++

	if progress.QuestionIndex < len(task.Questions) {
		nextMsgID := h.sendQuestionMsg(chat, task, progress.QuestionIndex)
		pd.QuestionMsgID = nextMsgID
		updatedData, _ := json.Marshal(pd)
		progress.AnswersData = updatedData
		if err := h.subtaskProgressRepo.Upsert(ctx, progress); err != nil {
			return fmt.Errorf("subtask.admin_only.HandleAnswer: save progress: %w", err)
		}
		return nil
	}

	return h.completeAdminTask(ctx, game, player, task, pd.Answers)
}

// sendQuestionMsg sends the question text with an inline button and returns the message ID.
func (h *AdminOnlyHandler) sendQuestionMsg(chat *tele.Chat, task *config.Task, idx int) int {
	q := task.Questions[idx]
	kbd := buildTask12QuestionKeyboard(q)
	sentMsg, _ := h.sender.Send(chat, q.Text, formatter.ParseMode, kbd)
	if sentMsg != nil {
		return sentMsg.ID
	}
	return 0
}

// completeAdminTask generates the OpenAI collage and finalises task_12.
func (h *AdminOnlyHandler) completeAdminTask(
	ctx context.Context,
	game *entity.Game,
	player *entity.Player,
	task *config.Task,
	answers map[string]string,
) error {
	chat := &tele.Chat{ID: game.ChatID}

	// Render OpenAI prompt
	var prompt string
	if task.OpenAI != nil {
		rendered, err := formatter.RenderTemplate(task.OpenAI.PromptTemplate, struct {
			Answers map[string]string
		}{Answers: answers})
		if err != nil {
			h.log.Warn().Err(err).Msg("admin_only: failed to render prompt template, using raw template")
			prompt = task.OpenAI.PromptTemplate
		} else {
			prompt = rendered
		}
	}

	h.sender.Send(chat, task.Summary.SendingText, formatter.ParseMode) //nolint:errcheck

	imageBytes, err := h.openai.GenerateCollage(ctx, prompt)
	if err != nil {
		return fmt.Errorf("subtask.admin_only.completeAdminTask: generate collage: %w", err)
	}

	tmpPath, err := h.saveToTempFile(imageBytes)
	if err != nil {
		return fmt.Errorf("subtask.admin_only.completeAdminTask: save image: %w", err)
	}

	photo := &tele.Photo{
		File:    tele.FromDisk(tmpPath),
		Caption: task.Summary.ReadyText,
	}
	h.sender.Send(chat, photo, formatter.ParseMode) //nolint:errcheck

	go func() {
		time.Sleep(5 * time.Second)
		os.Remove(tmpPath) //nolint:errcheck
	}()

	// Save task_response
	responseData, _ := json.Marshal(map[string]interface{}{"answers": answers})
	resp := &entity.TaskResponse{
		GameID:       game.ID,
		PlayerID:     player.ID,
		TaskID:       task.ID,
		Status:       entity.ResponseAnswered,
		ResponseData: responseData,
	}
	if err := h.taskResponseRepo.Create(ctx, resp); err != nil {
		return fmt.Errorf("subtask.admin_only.completeAdminTask: create response: %w", err)
	}

	// Save task_result so OpenAICollageFinalizer can verify it
	resultData, _ := json.Marshal(map[string]interface{}{"answers": answers, "image_generated": true})
	taskResult := &entity.TaskResult{
		GameID:      game.ID,
		TaskID:      task.ID,
		ResultData:  resultData,
		FinalizedAt: time.Now(),
	}
	if err := h.taskResultRepo.Create(ctx, taskResult); err != nil {
		h.log.Warn().Err(err).Msg("admin_only: failed to save task result")
	}

	if err := h.subtaskProgressRepo.Delete(ctx, game.ID, player.ID, task.ID); err != nil {
		h.log.Warn().Err(err).Msg("admin_only: failed to delete progress")
	}

	if err := h.playerStateRepo.SetIdle(ctx, game.ID, player.ID); err != nil {
		return fmt.Errorf("subtask.admin_only.completeAdminTask: set idle: %w", err)
	}

	h.log.Info().
		Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
		Str("user", logger.UserValue(player.TelegramUserID, player.Username)).
		Str("task", task.ID).
		Msg("admin_only: task completed, collage generated")

	return nil
}

func (h *AdminOnlyHandler) saveToTempFile(data []byte) (string, error) {
	tmp, err := os.CreateTemp(os.TempDir(), "openai_collage_*.png")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmp.Close()
	if _, err := tmp.Write(data); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("write temp file: %w", err)
	}
	return tmp.Name(), nil
}

// buildTask12QuestionKeyboard creates the inline keyboard with the single question button.
func buildTask12QuestionKeyboard(q config.TaskQuestion) *tele.ReplyMarkup {
	kbd := &tele.ReplyMarkup{}
	btn := kbd.Data(q.ButtonLabel, "task12_question", q.ID)
	kbd.Inline(kbd.Row(btn))
	return kbd
}
