package handler

import (
	"context"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// MessageHandler routes incoming group messages to the appropriate task handler.
type MessageHandler struct {
	gameRepo        repository.GameRepository
	playerRepo      repository.PlayerRepository
	playerStateRepo repository.PlayerStateRepository
	answerer        *task.Answerer
	log             zerolog.Logger
}

func NewMessageHandler(
	gameRepo repository.GameRepository,
	playerRepo repository.PlayerRepository,
	playerStateRepo repository.PlayerStateRepository,
	answerer *task.Answerer,
	log zerolog.Logger,
) *MessageHandler {
	return &MessageHandler{
		gameRepo:        gameRepo,
		playerRepo:      playerRepo,
		playerStateRepo: playerStateRepo,
		answerer:        answerer,
		log:             log,
	}
}

// OnMessage handles any incoming group message.
// Ignores messages from chats without an active game, non-players, and players not awaiting an answer.
func (h *MessageHandler) OnMessage(c tele.Context) error {
	ctx := context.Background()

	game, err := h.gameRepo.GetByChatID(ctx, c.Chat().ID)
	if err != nil || game == nil || game.Status != entity.GameActive {
		return nil
	}

	player, err := h.playerRepo.GetByGameAndTelegramID(ctx, game.ID, c.Sender().ID)
	if err != nil || player == nil {
		return nil
	}

	state, err := h.playerStateRepo.GetByPlayerAndGame(ctx, game.ID, player.ID)
	if err != nil || state == nil || state.State != entity.PlayerStateAwaitingAnswer {
		return nil
	}

	return h.answerer.Answer(ctx, game, player, c.Message())
}
