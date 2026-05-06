package handler

import (
	"context"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/delivery/bot/keyboard"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/game"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// ChatMemberHandler handles events when the bot's membership in a chat changes.
type ChatMemberHandler struct {
	creator *game.Creator
	bot     *tele.Bot
	cfg     *config.Config
	log     zerolog.Logger
}

func NewChatMemberHandler(creator *game.Creator, bot *tele.Bot, cfg *config.Config, log zerolog.Logger) *ChatMemberHandler {
	return &ChatMemberHandler{creator: creator, bot: bot, cfg: cfg, log: log}
}

// OnMyChatMember is called when the bot's own membership status changes.
func (h *ChatMemberHandler) OnMyChatMember(c tele.Context) error {
	cm := c.ChatMember()
	if cm == nil {
		return nil
	}

	// Only handle "bot was added to a chat"
	isAdded := (cm.OldChatMember.Role == tele.Left || cm.OldChatMember.Role == tele.Kicked) &&
		(cm.NewChatMember.Role == tele.Member || cm.NewChatMember.Role == tele.Administrator)
	if !isAdded {
		return nil
	}

	adminUser := *cm.Sender
	chatName := c.Chat().Title
	ctx := context.Background()

	if _, err := h.creator.Create(ctx, c.Chat().ID, chatName, adminUser); err != nil {
		h.log.Error().Err(err).Int64("chat", c.Chat().ID).Msg("chat_member: create game")
		return nil
	}

	// Send join invite + join keyboard
	h.bot.Send(c.Chat(), h.cfg.Messages.JoinInvite, keyboard.JoinKeyboard(h.cfg.Support.TelegramURL), formatter.ParseMode) //nolint:errcheck

	// After JoinMessageDelay — send start invite + start keyboard
	go func() {
		time.Sleep(h.cfg.Timings.JoinMessageDelay)
		h.bot.Send(c.Chat(), h.cfg.Messages.StartInvite, keyboard.StartKeyboard(), formatter.ParseMode) //nolint:errcheck
	}()

	return nil
}
