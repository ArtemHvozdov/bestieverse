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

// Answerer handles an incoming message that constitutes a task answer.
type Answerer struct {
	taskResponseRepo repository.TaskResponseRepository
	playerStateRepo  repository.PlayerStateRepository
	sender           Sender
	msgs             *config.Messages
	timings          *config.Timings
	log              zerolog.Logger
}

func NewAnswerer(
	taskResponseRepo repository.TaskResponseRepository,
	playerStateRepo repository.PlayerStateRepository,
	sender Sender,
	msgs *config.Messages,
	timings *config.Timings,
	log zerolog.Logger,
) *Answerer {
	return &Answerer{
		taskResponseRepo: taskResponseRepo,
		playerStateRepo:  playerStateRepo,
		sender:           sender,
		msgs:             msgs,
		timings:          timings,
		log:              log,
	}
}

// Answer records the player's answer and transitions them back to idle.
// Returns nil without action if the player is not in awaiting_answer state.
func (a *Answerer) Answer(ctx context.Context, game *entity.Game, player *entity.Player, _ *tele.Message) error {
	state, err := a.playerStateRepo.GetByPlayerAndGame(ctx, game.ID, player.ID)
	if err != nil {
		return fmt.Errorf("task.Answer: get state: %w", err)
	}
	if state == nil || state.State != entity.PlayerStateAwaitingAnswer {
		return nil
	}

	response := &entity.TaskResponse{
		GameID:   game.ID,
		PlayerID: player.ID,
		TaskID:   state.TaskID,
		Status:   entity.ResponseAnswered,
	}
	if err := a.taskResponseRepo.Create(ctx, response); err != nil {
		return fmt.Errorf("task.Answer: create response: %w", err)
	}

	if err := a.playerStateRepo.SetIdle(ctx, game.ID, player.ID); err != nil {
		return fmt.Errorf("task.Answer: set idle: %w", err)
	}

	chat := &tele.Chat{ID: game.ChatID}
	mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)
	text, _ := formatter.RenderTemplate(a.msgs.AnswerAccepted, struct{ Mention string }{Mention: mention})
	resp, _ := a.sender.Send(chat, text, formatter.ParseMode)
	if resp != nil {
		deleteAfter(a.sender, resp, a.timings.DeleteMessageDelay)
	}

	a.log.Info().
		Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).
		Uint64("game", game.ID).
		Str("user", logger.UserValue(player.TelegramUserID, player.Username)).
		Str("task", state.TaskID).
		Msg("task answered")

	return nil
}
