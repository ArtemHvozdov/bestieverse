package task

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

// Publisher publishes the next task for a game.
// Implements game.TaskPublisher.
type Publisher struct {
	gameRepo repository.GameRepository
	media    media.Storage
	sender   Sender
	cfg      *config.Config
	log      zerolog.Logger
}

func NewPublisher(
	gameRepo repository.GameRepository,
	mediaStorage media.Storage,
	sender Sender,
	cfg *config.Config,
	log zerolog.Logger,
) *Publisher {
	return &Publisher{
		gameRepo: gameRepo,
		media:    mediaStorage,
		sender:   sender,
		cfg:      cfg,
		log:      log,
	}
}

// Publish publishes the next task in sequence for the given game.
// When all tasks are exhausted it is a no-op (game finish is handled by Stage 6).
func (p *Publisher) Publish(ctx context.Context, game *entity.Game) error {
	nextOrder := game.CurrentTaskOrder + 1
	task := p.cfg.TaskByOrder(nextOrder)
	if task == nil {
		p.log.Info().Uint64("game", game.ID).Msg("all tasks published")
		return nil
	}

	now := time.Now()
	if err := p.gameRepo.UpdateCurrentTask(ctx, game.ID, task.Order, now); err != nil {
		return fmt.Errorf("task.Publish: update current task: %w", err)
	}

	chat := &tele.Chat{ID: game.ChatID}

	switch task.Type {
	case "question_answer":
		kbd := buildTaskKeyboard(task.ID)
		if anim, err := p.media.GetAnimation(task.MediaFile); err == nil {
			anim.Caption = task.Text
			p.sender.Send(chat, anim, formatter.ParseMode, kbd) //nolint:errcheck
		} else {
			p.sender.Send(chat, task.Text, formatter.ParseMode, kbd) //nolint:errcheck
		}
	}

	p.log.Info().
		Int64("chat", game.ChatID).
		Uint64("game", game.ID).
		Str("task", task.ID).
		Msg("task published")

	return nil
}

// buildTaskKeyboard constructs the inline keyboard attached to a published task.
func buildTaskKeyboard(taskID string) *tele.ReplyMarkup {
	kbd := &tele.ReplyMarkup{}
	answer := kbd.Data("Хочу відповісти ✍️", "task:request", taskID)
	skip := kbd.Data("Пропустити ⏭️", "task:skip", taskID)
	kbd.Inline(kbd.Row(answer, skip))
	return kbd
}
