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

	existing, err := r.taskResponseRepo.GetByPlayerAndTask(ctx, game.ID, player.ID, task.ID)
	if err != nil {
		return fmt.Errorf("task.RequestAnswer: get response: %w", err)
	}
	if existing != nil {
		msg, _ := r.sender.Send(chat, config.Random(r.msgs.AlreadyAnswered), formatter.ParseMode)
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

	msg, _ := r.sender.Send(chat, config.Random(r.msgs.AwaitingAnswer), formatter.ParseMode)
	if msg != nil {
		deleteAfter(r.sender, msg, r.timings.DeleteMessageDelay)
	}

	r.log.Info().
		Int64("chat", game.ChatID).
		Int64("user", player.TelegramUserID).
		Str("task", task.ID).
		Msg("awaiting answer")

	return nil
}
