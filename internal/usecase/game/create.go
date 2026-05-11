package game

import (
	"context"
	"fmt"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/logger"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// Creator handles game creation when the bot is added to a chat.
type Creator struct {
	gameRepo        repository.GameRepository
	playerRepo      repository.PlayerRepository
	playerStateRepo repository.PlayerStateRepository
	log             zerolog.Logger
}

func NewCreator(
	gameRepo repository.GameRepository,
	playerRepo repository.PlayerRepository,
	playerStateRepo repository.PlayerStateRepository,
	log zerolog.Logger,
) *Creator {
	return &Creator{
		gameRepo:        gameRepo,
		playerRepo:      playerRepo,
		playerStateRepo: playerStateRepo,
		log:             log,
	}
}

// Create initialises a new game for the given chat. Idempotent: returns nil, nil
// if a game for this chat already exists.
func (c *Creator) Create(ctx context.Context, chatID int64, chatName string, adminUser tele.User) (*entity.Game, error) {
	existing, err := c.gameRepo.GetByChatID(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("game.Create: get by chat: %w", err)
	}
	if existing != nil {
		return nil, nil
	}

	game := &entity.Game{
		ChatID:        chatID,
		ChatName:      chatName,
		AdminUserID:   adminUser.ID,
		AdminUsername: adminUser.Username,
		Status:        entity.GamePending,
	}
	game, err = c.gameRepo.Create(ctx, game)
	if err != nil {
		return nil, fmt.Errorf("game.Create: %w", err)
	}

	player := &entity.Player{
		GameID:         game.ID,
		TelegramUserID: adminUser.ID,
		Username:       adminUser.Username,
		FirstName:      adminUser.FirstName,
	}
	player, err = c.playerRepo.Create(ctx, player)
	if err != nil {
		return nil, fmt.Errorf("game.Create: create admin player: %w", err)
	}

	state := &entity.PlayerState{
		GameID:   game.ID,
		PlayerID: player.ID,
		State:    entity.PlayerStateIdle,
	}
	if err := c.playerStateRepo.Upsert(ctx, state); err != nil {
		return nil, fmt.Errorf("game.Create: upsert player state: %w", err)
	}

	c.log.Info().
		Int64("chat", chatID).
		Str("admin", logger.UserValue(adminUser.ID, adminUser.Username)).
		Msg("game created")

	return game, nil
}
