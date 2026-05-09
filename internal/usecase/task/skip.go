package task

import (
	"context"
	"fmt"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// Skipper handles the "Пропустити" button press.
type Skipper struct {
	taskResponseRepo repository.TaskResponseRepository
	playerRepo       repository.PlayerRepository
	sender           Sender
	msgs             *config.Messages
	timings          *config.Timings
	log              zerolog.Logger
}

func NewSkipper(
	taskResponseRepo repository.TaskResponseRepository,
	playerRepo repository.PlayerRepository,
	sender Sender,
	msgs *config.Messages,
	timings *config.Timings,
	log zerolog.Logger,
) *Skipper {
	return &Skipper{
		taskResponseRepo: taskResponseRepo,
		playerRepo:       playerRepo,
		sender:           sender,
		msgs:             msgs,
		timings:          timings,
		log:              log,
	}
}

// Skip records a skip for the player on the given task.
// Enforces the 3-skip-per-game limit. Sends the appropriate remaining-skips message.
func (s *Skipper) Skip(ctx context.Context, game *entity.Game, player *entity.Player, taskID string) error {
	chat := &tele.Chat{ID: game.ChatID}
	mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)
	mentionData := struct{ Mention string }{Mention: mention}

	existing, err := s.taskResponseRepo.GetByPlayerAndTask(ctx, game.ID, player.ID, taskID)
	if err != nil {
		return fmt.Errorf("task.Skip: get response: %w", err)
	}
	if existing != nil {
		var rawText string
		if existing.Status == entity.ResponseAnswered {
			rawText = config.Random(s.msgs.AlreadyAnswered)
		} else {
			rawText = s.msgs.AlreadySkipped
		}
		text, _ := formatter.RenderTemplate(rawText, mentionData)
		msg, _ := s.sender.Send(chat, text, formatter.ParseMode)
		if msg != nil {
			deleteAfter(s.sender, msg, s.timings.DeleteMessageDelay)
		}
		return nil
	}

	if player.SkipCount >= 3 {
		text, _ := formatter.RenderTemplate(s.msgs.SkipNoRemaining, mentionData)
		msg, _ := s.sender.Send(chat, text, formatter.ParseMode)
		if msg != nil {
			deleteAfter(s.sender, msg, s.timings.DeleteMessageDelay)
		}
		return nil
	}

	if err := s.playerRepo.IncrementSkipCount(ctx, player.ID); err != nil {
		return fmt.Errorf("task.Skip: increment skip count: %w", err)
	}

	response := &entity.TaskResponse{
		GameID:   game.ID,
		PlayerID: player.ID,
		TaskID:   taskID,
		Status:   entity.ResponseSkipped,
	}
	if err := s.taskResponseRepo.Create(ctx, response); err != nil {
		return fmt.Errorf("task.Skip: create response: %w", err)
	}

	newSkipCount := player.SkipCount + 1
	var rawText string
	switch newSkipCount {
	case 1:
		rawText = s.msgs.SkipWithRemaining2
	case 2:
		rawText = s.msgs.SkipWithRemaining1
	default:
		rawText = s.msgs.SkipLast
	}
	text, _ := formatter.RenderTemplate(rawText, mentionData)

	msg, _ := s.sender.Send(chat, text, formatter.ParseMode)
	if msg != nil {
		deleteAfter(s.sender, msg, s.timings.DeleteMessageDelay)
	}

	s.log.Info().
		Int64("chat", game.ChatID).
		Int64("user", player.TelegramUserID).
		Str("task", taskID).
		Int("skip_count", newSkipCount).
		Msg("task skipped")

	return nil
}
