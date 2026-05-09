package game

import (
	"context"
	"fmt"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// Joiner handles a player joining the game.
type Joiner struct {
	gameRepo        repository.GameRepository
	playerRepo      repository.PlayerRepository
	playerStateRepo repository.PlayerStateRepository
	sender          Sender
	msgs            *config.Messages
	timings         *config.Timings
	log             zerolog.Logger
}

func NewJoiner(
	gameRepo repository.GameRepository,
	playerRepo repository.PlayerRepository,
	playerStateRepo repository.PlayerStateRepository,
	sender Sender,
	msgs *config.Messages,
	timings *config.Timings,
	log zerolog.Logger,
) *Joiner {
	return &Joiner{
		gameRepo:        gameRepo,
		playerRepo:      playerRepo,
		playerStateRepo: playerStateRepo,
		sender:          sender,
		msgs:            msgs,
		timings:         timings,
		log:             log,
	}
}

// Join registers the user as a player in the game for this chat.
func (j *Joiner) Join(ctx context.Context, chatID int64, user tele.User) error {
	game, err := j.gameRepo.GetByChatID(ctx, chatID)
	if err != nil {
		return fmt.Errorf("game.Join: get game: %w", err)
	}
	if game == nil || game.Status == entity.GameFinished {
		return nil
	}

	chat := &tele.Chat{ID: chatID}
	mention := formatter.Mention(user.ID, user.Username, user.FirstName)

	if game.AdminUserID == user.ID {
		text, _ := formatter.RenderTemplate(j.msgs.JoinAdminAlready, struct{ Mention string }{Mention: mention})
		msg, _ := j.sender.Send(chat, text, formatter.ParseMode)
		if msg != nil {
			deleteAfter(j.sender, msg, j.timings.DeleteMessageDelay)
		}
		return nil
	}

	existing, err := j.playerRepo.GetByGameAndTelegramID(ctx, game.ID, user.ID)
	if err != nil {
		return fmt.Errorf("game.Join: get player: %w", err)
	}
	if existing != nil {
		text, _ := formatter.RenderTemplate(j.msgs.JoinAlreadyMember, struct{ Mention string }{Mention: mention})
		msg, _ := j.sender.Send(chat, text, formatter.ParseMode)
		if msg != nil {
			deleteAfter(j.sender, msg, j.timings.DeleteMessageDelay)
		}
		return nil
	}

	player := &entity.Player{
		GameID:         game.ID,
		TelegramUserID: user.ID,
		Username:       user.Username,
		FirstName:      user.FirstName,
	}
	player, err = j.playerRepo.Create(ctx, player)
	if err != nil {
		return fmt.Errorf("game.Join: create player: %w", err)
	}

	state := &entity.PlayerState{
		GameID:   game.ID,
		PlayerID: player.ID,
		State:    entity.PlayerStateIdle,
	}
	if err := j.playerStateRepo.Upsert(ctx, state); err != nil {
		return fmt.Errorf("game.Join: upsert state: %w", err)
	}

	text, _ := formatter.RenderTemplate(config.Random(j.msgs.JoinWelcome), struct{ Mention string }{Mention: mention})
	j.sender.Send(chat, text, formatter.ParseMode) //nolint:errcheck

	j.log.Info().Int64("chat", chatID).Int64("user", user.ID).Str("username", user.Username).Msg("player joined")
	return nil
}
