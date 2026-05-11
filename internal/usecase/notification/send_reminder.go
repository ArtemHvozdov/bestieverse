package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/logger"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// Sender is the minimal bot interface required by the notifier.
type Sender interface {
	Send(to tele.Recipient, what interface{}, opts ...interface{}) (*tele.Message, error)
}

// ReminderSender sends reminders to players who have not yet answered the current task.
type ReminderSender struct {
	gameRepo         repository.GameRepository
	notificationRepo repository.NotificationRepository
	sender           Sender
	cfg              *config.Config
	log              zerolog.Logger
}

func NewReminderSender(
	gameRepo repository.GameRepository,
	notificationRepo repository.NotificationRepository,
	sender Sender,
	cfg *config.Config,
	log zerolog.Logger,
) *ReminderSender {
	return &ReminderSender{
		gameRepo:         gameRepo,
		notificationRepo: notificationRepo,
		sender:           sender,
		cfg:              cfg,
		log:              log,
	}
}

// SendReminders iterates all active games and notifies players who have not responded.
func (rs *ReminderSender) SendReminders(ctx context.Context) error {
	games, err := rs.gameRepo.GetAllActive(ctx)
	if err != nil {
		return fmt.Errorf("notification.SendReminders: get active games: %w", err)
	}

	for _, game := range games {
		if err := rs.remindGame(ctx, game); err != nil {
			rs.log.Error().Err(err).Uint64("game", game.ID).Msg("send reminders: game error")
		}
	}
	return nil
}

func (rs *ReminderSender) remindGame(ctx context.Context, game *entity.Game) error {
	if game.CurrentTaskPublishedAt == nil {
		return nil
	}
	if time.Since(*game.CurrentTaskPublishedAt) < rs.cfg.Timings.ReminderDelay {
		return nil
	}

	task := rs.cfg.TaskByOrder(game.CurrentTaskOrder)
	if task == nil {
		return nil
	}

	players, err := rs.notificationRepo.GetUnnotifiedPlayers(ctx, game.ID, task.ID)
	if err != nil {
		return fmt.Errorf("notification.SendReminders: get unnotified players: %w", err)
	}

	chat := &tele.Chat{ID: game.ChatID}

	for _, player := range players {
		mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)
		tmpl := config.Random(rs.cfg.Messages.Reminder)
		text, err := formatter.RenderTemplate(tmpl, struct{ Mention string }{Mention: mention})
		if err != nil {
			text = tmpl
		}

		rs.sender.Send(chat, text, formatter.ParseMode) //nolint:errcheck

		if err := rs.notificationRepo.Create(ctx, &entity.NotificationLog{
			GameID:   game.ID,
			PlayerID: player.ID,
			TaskID:   task.ID,
			SentAt:   time.Now(),
		}); err != nil {
			rs.log.Error().Err(err).
				Uint64("game", game.ID).
				Uint64("player", player.ID).
				Msg("send reminder: save log")
		}

		rs.log.Info().
			Int64("chat", game.ChatID).
			Uint64("game", game.ID).
			Str("user", logger.UserValue(player.TelegramUserID, player.Username)).
			Msg("reminder sent")
	}

	return nil
}
