package handler

import (
	"context"
	"strings"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task/subtask"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// MessageHandler routes incoming group messages to the appropriate task handler.
type MessageHandler struct {
	gameRepo             repository.GameRepository
	playerRepo           repository.PlayerRepository
	playerStateRepo      repository.PlayerStateRepository
	answerer             *task.Answerer
	memeVoiceoverHandler *subtask.MemeVoiceoverHandler
	adminOnlyHandler     *subtask.AdminOnlyHandler
	cfg                  *config.Config
	log                  zerolog.Logger
}

func NewMessageHandler(
	gameRepo repository.GameRepository,
	playerRepo repository.PlayerRepository,
	playerStateRepo repository.PlayerStateRepository,
	answerer *task.Answerer,
	memeVoiceoverHandler *subtask.MemeVoiceoverHandler,
	adminOnlyHandler *subtask.AdminOnlyHandler,
	cfg *config.Config,
	log zerolog.Logger,
) *MessageHandler {
	return &MessageHandler{
		gameRepo:             gameRepo,
		playerRepo:           playerRepo,
		playerStateRepo:      playerStateRepo,
		answerer:             answerer,
		memeVoiceoverHandler: memeVoiceoverHandler,
		adminOnlyHandler:     adminOnlyHandler,
		cfg:                  cfg,
		log:                  log,
	}
}

// OnMessage handles any incoming group message.
// Ignores messages from chats without an active game, non-players, and players not awaiting an answer.
// Routes messages with a ":meme" suffix in state.TaskID to the meme voiceover handler.
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

	if strings.HasSuffix(state.TaskID, ":meme") {
		baseTaskID := strings.TrimSuffix(state.TaskID, ":meme")
		t := h.cfg.TaskByID(baseTaskID)
		if t == nil {
			h.log.Warn().Str("task_id", baseTaskID).Msg("meme_voiceover: task not found in config")
			return nil
		}
		return h.memeVoiceoverHandler.HandleAnswer(ctx, game, player, t, c.Message())
	}

	if strings.HasSuffix(state.TaskID, ":admin") {
		baseTaskID := strings.TrimSuffix(state.TaskID, ":admin")
		t := h.cfg.TaskByID(baseTaskID)
		if t == nil {
			h.log.Warn().Str("task_id", baseTaskID).Msg("admin_only: task not found in config")
			return nil
		}
		return h.adminOnlyHandler.HandleAnswer(ctx, game, player, t, c.Message())
	}

	return h.answerer.Answer(ctx, game, player, c.Message())
}
