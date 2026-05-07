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

	case "poll_then_task":
		if task.Poll == nil {
			return fmt.Errorf("task.Publish: task %s has no poll config", task.ID)
		}
		// Send animation with task description
		if anim, err := p.media.GetAnimation(task.MediaFile); err == nil {
			anim.Caption = task.Text
			p.sender.Send(chat, anim, formatter.ParseMode) //nolint:errcheck
		} else {
			p.sender.Send(chat, task.Text, formatter.ParseMode) //nolint:errcheck
		}
		// Build and send the Telegram poll
		options := make([]tele.PollOption, len(task.Poll.Options))
		for i, opt := range task.Poll.Options {
			options[i] = tele.PollOption{Text: opt.Label}
		}
		poll := &tele.Poll{
			Question:      task.Poll.Title,
			Options:       options,
			Anonymous:     true,
			CloseUnixdate: now.Add(p.cfg.Timings.PollDuration).Unix(),
		}
		msg, err := p.sender.Send(chat, poll)
		if err != nil {
			return fmt.Errorf("task.Publish: send poll: %w", err)
		}
		if msg != nil && msg.Poll != nil {
			if err := p.gameRepo.SetActivePollID(ctx, game.ID, msg.Poll.ID); err != nil {
				return fmt.Errorf("task.Publish: set active poll id: %w", err)
			}
		}

	case "admin_only":
		if len(task.Messages) == 0 {
			return fmt.Errorf("task.Publish: task %s has no messages", task.ID)
		}
		// First message: animation/photo with text
		msg0 := task.Messages[0]
		if msg0.MediaFile != "" {
			if anim, err := p.media.GetAnimation(msg0.MediaFile); err == nil {
				anim.Caption = msg0.Text
				p.sender.Send(chat, anim, formatter.ParseMode) //nolint:errcheck
			} else {
				p.sender.Send(chat, msg0.Text, formatter.ParseMode) //nolint:errcheck
			}
		} else {
			p.sender.Send(chat, msg0.Text, formatter.ParseMode) //nolint:errcheck
		}
		// Second message with task keyboard after TaskInfoInterval
		if len(task.Messages) > 1 {
			time.Sleep(p.cfg.Timings.TaskInfoInterval)
			msg1 := task.Messages[1]
			kbd := buildTaskKeyboard(task.ID)
			p.sender.Send(chat, msg1.Text, formatter.ParseMode, kbd) //nolint:errcheck
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
