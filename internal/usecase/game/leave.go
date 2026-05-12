package game

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

// Leaver handles a player leaving the game.
type Leaver struct {
	playerRepo repository.PlayerRepository
	sender     Sender
	msgs       *config.Messages
	timings    *config.Timings
	log        zerolog.Logger
}

func NewLeaver(
	playerRepo repository.PlayerRepository,
	sender Sender,
	msgs *config.Messages,
	timings *config.Timings,
	log zerolog.Logger,
) *Leaver {
	return &Leaver{
		playerRepo: playerRepo,
		sender:     sender,
		msgs:       msgs,
		timings:    timings,
		log:        log,
	}
}

// InitiateLeave shows a confirmation prompt. kb is the LeaveConfirmKeyboard
// provided by the delivery layer keyboard factory.
func (l *Leaver) InitiateLeave(ctx context.Context, game *entity.Game, player *entity.Player, kb *tele.ReplyMarkup) error {
	chat := &tele.Chat{ID: game.ChatID}
	mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)

	if player.TelegramUserID == game.AdminUserID {
		text, _ := formatter.RenderTemplate(l.msgs.LeaveAdminBlocked, struct{ Mention string }{Mention: mention})
		msg, _ := l.sender.Send(chat, text, formatter.ParseMode)
		if msg != nil {
			deleteAfter(l.sender, msg, l.timings.DeleteMessageDelay)
		}
		return nil
	}

	text, _ := formatter.RenderTemplate(l.msgs.LeaveConfirm, struct{ Mention string }{Mention: mention})
	l.sender.Send(chat, text, kb, formatter.ParseMode) //nolint:errcheck
	return nil
}

// ConfirmLeave removes the player from the game after they confirmed leaving.
func (l *Leaver) ConfirmLeave(ctx context.Context, game *entity.Game, player *entity.Player, confirmMsg *tele.Message) error {
	if err := l.playerRepo.Delete(ctx, player.ID); err != nil {
		return fmt.Errorf("game.ConfirmLeave: %w", err)
	}

	chat := &tele.Chat{ID: game.ChatID}
	mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)
	text, _ := formatter.RenderTemplate(l.msgs.LeaveSuccess, struct{ Mention string }{Mention: mention})
	l.sender.Send(chat, text, formatter.ParseMode) //nolint:errcheck

	l.log.Info().Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).Str("user", logger.UserValue(player.TelegramUserID, player.Username)).Msg("player left")
	return nil
}

// CancelLeave removes the confirmation prompt and notifies the player they stayed.
func (l *Leaver) CancelLeave(ctx context.Context, game *entity.Game, player *entity.Player, confirmMsg *tele.Message) error {
	l.sender.Delete(confirmMsg) //nolint:errcheck

	chat := &tele.Chat{ID: game.ChatID}
	mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)
	text, _ := formatter.RenderTemplate(l.msgs.CancelLeave, struct{ Mention string }{Mention: mention})
	msg, _ := l.sender.Send(chat, text, formatter.ParseMode)
	if msg != nil {
		deleteAfter(l.sender, msg, l.timings.DeleteMessageDelay)
	}
	return nil
}
