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

	h.log.Info().
		Int64("chat_id", c.Chat().ID).
		Int64("sender_id", c.Sender().ID).
		Str("msg_type", messageType(c.Message())).
		Msg("OnMessage: received")

	game, err := h.gameRepo.GetByChatID(ctx, c.Chat().ID)
	if err != nil {
		h.log.Warn().Err(err).Int64("chat_id", c.Chat().ID).Msg("OnMessage: get game error")
		return nil
	}
	if game == nil {
		h.log.Warn().Int64("chat_id", c.Chat().ID).Msg("OnMessage: no game for chat, ignoring")
		return nil
	}
	if game.Status != entity.GameActive {
		h.log.Warn().
			Int64("chat_id", c.Chat().ID).
			Str("game_status", string(game.Status)).
			Uint64("game_id", game.ID).
			Msg("OnMessage: game not active, ignoring")
		return nil
	}

	player, err := h.playerRepo.GetByGameAndTelegramID(ctx, game.ID, c.Sender().ID)
	if err != nil {
		h.log.Warn().Err(err).Int64("sender_id", c.Sender().ID).Msg("OnMessage: get player error")
		return nil
	}
	if player == nil {
		h.log.Warn().Int64("sender_id", c.Sender().ID).Uint64("game_id", game.ID).Msg("OnMessage: sender not a player, ignoring")
		return nil
	}

	state, err := h.playerStateRepo.GetByPlayerAndGame(ctx, game.ID, player.ID)
	if err != nil {
		h.log.Warn().Err(err).Uint64("player_id", player.ID).Msg("OnMessage: get player state error")
		return nil
	}
	if state == nil || state.State != entity.PlayerStateAwaitingAnswer {
		h.log.Warn().
			Uint64("player_id", player.ID).
			Str("state", func() string {
				if state == nil {
					return "nil"
				}
				return string(state.State)
			}()).
			Str("task_id", func() string {
				if state == nil {
					return ""
				}
				return state.TaskID
			}()).
			Msg("OnMessage: player not awaiting answer, ignoring")
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

// messageType returns a short diagnostic string identifying the media type of m.
func messageType(m *tele.Message) string {
	switch {
	case m == nil:
		return "nil"
	case m.Text != "":
		return "text"
	case m.Photo != nil:
		return "photo"
	case m.Voice != nil:
		return "voice"
	case m.Audio != nil:
		return "audio"
	case m.Animation != nil:
		return "animation"
	case m.Document != nil:
		return "document"
	case m.Sticker != nil:
		return "sticker"
	case m.Video != nil:
		return "video"
	case m.VideoNote != nil:
		return "video_note"
	default:
		return "unknown"
	}
}
