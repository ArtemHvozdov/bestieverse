package task

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

// RequestAnswerer handles the "Хочу відповісти" button press.
type RequestAnswerer struct {
	taskResponseRepo repository.TaskResponseRepository
	playerStateRepo  repository.PlayerStateRepository
	sender           Sender
	msgs             *config.Messages
	timings          *config.Timings
	log              zerolog.Logger
}

func NewRequestAnswerer(
	taskResponseRepo repository.TaskResponseRepository,
	playerStateRepo repository.PlayerStateRepository,
	sender Sender,
	msgs *config.Messages,
	timings *config.Timings,
	log zerolog.Logger,
) *RequestAnswerer {
	return &RequestAnswerer{
		taskResponseRepo: taskResponseRepo,
		playerStateRepo:  playerStateRepo,
		sender:           sender,
		msgs:             msgs,
		timings:          timings,
		log:              log,
	}
}

// RequestAnswer transitions the player to awaiting_answer state for the given task.
// If the player has already answered or skipped, it sends an error message instead.
func (r *RequestAnswerer) RequestAnswer(ctx context.Context, game *entity.Game, player *entity.Player, task *config.Task) error {
	chat := &tele.Chat{ID: game.ChatID}

	mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)
	mentionData := struct{ Mention string }{Mention: mention}

	existing, err := r.taskResponseRepo.GetByPlayerAndTask(ctx, game.ID, player.ID, task.ID)
	if err != nil {
		return fmt.Errorf("task.RequestAnswer: get response: %w", err)
	}
	if existing != nil {
		text, _ := formatter.RenderTemplate(config.Random(r.msgs.AlreadyAnswered), mentionData)
		msg, _ := r.sender.Send(chat, text, formatter.ParseMode)
		if msg != nil {
			deleteAfter(r.sender, msg, r.timings.DeleteMessageDelay)
		}
		return nil
	}

	state := &entity.PlayerState{
		GameID:   game.ID,
		PlayerID: player.ID,
		State:    entity.PlayerStateAwaitingAnswer,
		TaskID:   task.ID,
	}
	if err := r.playerStateRepo.Upsert(ctx, state); err != nil {
		return fmt.Errorf("task.RequestAnswer: upsert state: %w", err)
	}

	text, _ := formatter.RenderTemplate(config.Random(r.msgs.AwaitingAnswer), mentionData)
	msg, _ := r.sender.Send(chat, text, formatter.ParseMode)
	if msg != nil {
		deleteAfter(r.sender, msg, r.timings.DeleteMessageDelay)
	}

	r.log.Info().
		Int64("chat", game.ChatID).
		Str("user", logger.UserValue(player.TelegramUserID, player.Username)).
		Str("task", task.ID).
		Msg("awaiting answer")

	return nil
}
