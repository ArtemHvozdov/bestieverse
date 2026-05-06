package game

import (
	"context"
	"fmt"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/media"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

const gameStartMediaFile = "game/start.gif"

// TaskPublisher publishes the next task for a game. Implemented by usecase/task.Publisher.
type TaskPublisher interface {
	Publish(ctx context.Context, game *entity.Game) error
}

// Starter handles the "start game" action triggered by the admin.
type Starter struct {
	gameRepo  repository.GameRepository
	media     media.Storage
	sender    Sender
	publisher TaskPublisher
	cfg       *config.Config
	log       zerolog.Logger
}

func NewStarter(
	gameRepo repository.GameRepository,
	mediaStorage media.Storage,
	sender Sender,
	publisher TaskPublisher,
	cfg *config.Config,
	log zerolog.Logger,
) *Starter {
	return &Starter{
		gameRepo:  gameRepo,
		media:     mediaStorage,
		sender:    sender,
		publisher: publisher,
		cfg:       cfg,
		log:       log,
	}
}

// Start transitions the game to active and sends the intro messages.
// startMsg is the message containing the "Start" button — it is deleted immediately.
func (s *Starter) Start(ctx context.Context, game *entity.Game, player *entity.Player, startMsg *tele.Message) error {
	chat := &tele.Chat{ID: game.ChatID}
	mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)

	if player.TelegramUserID != game.AdminUserID {
		text, _ := formatter.RenderTemplate(s.cfg.Messages.StartOnlyAdmin, struct{ Mention string }{Mention: mention})
		msg, _ := s.sender.Send(chat, text, formatter.ParseMode)
		if msg != nil {
			deleteAfter(s.sender, msg, s.cfg.Timings.DeleteMessageDelay)
		}
		return nil
	}

	s.sender.Delete(startMsg) //nolint:errcheck

	if err := s.gameRepo.UpdateStatus(ctx, game.ID, entity.GameActive); err != nil {
		return fmt.Errorf("game.Start: update status: %w", err)
	}

	// Message 1: intro animation with caption, or text if file missing
	if anim, err := s.media.GetAnimation(gameStartMediaFile); err == nil {
		anim.Caption = s.cfg.Game.StartMessage1
		s.sender.Send(chat, anim, formatter.ParseMode) //nolint:errcheck
	} else {
		s.sender.Send(chat, s.cfg.Game.StartMessage1, formatter.ParseMode) //nolint:errcheck
	}

	time.Sleep(s.cfg.Timings.TaskInfoInterval)

	// Message 2: schedule and rules
	s.sender.Send(chat, s.cfg.Game.StartMessage2, formatter.ParseMode) //nolint:errcheck

	s.log.Info().Int64("chat", game.ChatID).Uint64("game", game.ID).Msg("game started")

	if s.publisher != nil {
		return s.publisher.Publish(ctx, game)
	}
	return nil
}
