package middleware

import (
	"context"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/telegram"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// PlayerCheck returns middleware that resolves game+player for the current chat/sender
// and stores them in the context. Requests from unknown players are rejected.
func PlayerCheck(
	gameRepo repository.GameRepository,
	playerRepo repository.PlayerRepository,
	bot *tele.Bot,
	msgs *config.Messages,
	timings *config.Timings,
	log zerolog.Logger,
) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			ctx := context.Background()

			game, err := gameRepo.GetByChatID(ctx, c.Chat().ID)
			if err != nil {
				log.Error().Err(err).Int64("chat", c.Chat().ID).Msg("player_check: get game")
				return nil
			}
			if game == nil {
				return nil
			}

			player, err := playerRepo.GetByGameAndTelegramID(ctx, game.ID, c.Sender().ID)
			if err != nil {
				log.Error().Err(err).Msg("player_check: get player")
				return nil
			}
			if player == nil {
				mention := formatter.Mention(c.Sender().ID, c.Sender().Username, c.Sender().FirstName)
				text, _ := formatter.RenderTemplate(msgs.NotInGame, struct{ Mention string }{Mention: mention})
				msg, _ := bot.Send(c.Chat(), text, formatter.ParseMode)
				if msg != nil {
					telegram.DeleteAfter(bot, msg, timings.DeleteMessageDelay)
				}
				return nil
			}

			c.Set("game", game)
			c.Set("player", player)
			return next(c)
		}
	}
}

// PlayerCheckForStart is like PlayerCheck but never blocks on missing player.
// If the sender is not in the game, a minimal Player with just the sender's
// Telegram identity is placed in the context so that Starter.Start can still
// perform the admin check and send the correct "only admin" message.
func PlayerCheckForStart(
	gameRepo repository.GameRepository,
	playerRepo repository.PlayerRepository,
	log zerolog.Logger,
) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			ctx := context.Background()

			game, err := gameRepo.GetByChatID(ctx, c.Chat().ID)
			if err != nil {
				log.Error().Err(err).Int64("chat", c.Chat().ID).Msg("player_check_start: get game")
				return nil
			}
			if game == nil {
				return nil
			}

			player, err := playerRepo.GetByGameAndTelegramID(ctx, game.ID, c.Sender().ID)
			if err != nil {
				log.Error().Err(err).Msg("player_check_start: get player")
				return nil
			}

			c.Set("game", game)
			if player != nil {
				c.Set("player", player)
			} else {
				// Sender is not a player yet; pass a minimal stub so handlers
				// can still check TelegramUserID against game.AdminUserID.
				c.Set("player", &entity.Player{
					TelegramUserID: c.Sender().ID,
					Username:       c.Sender().Username,
					FirstName:      c.Sender().FirstName,
				})
			}
			return next(c)
		}
	}
}
