package finalize

import (
	"context"
	"fmt"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/logger"
	tele "gopkg.in/telebot.v3"
)

const gameFinalMediaFile = "game/final.gif"

func (r *FinalizeRouter) finishGame(ctx context.Context, game *entity.Game) error {
	if err := r.gameRepo.SetFinished(ctx, game.ID); err != nil {
		return fmt.Errorf("finalize.finishGame: set finished: %w", err)
	}

	chat := &tele.Chat{ID: game.ChatID}

	if anim, err := r.media.GetAnimation(gameFinalMediaFile); err == nil {
		anim.Caption = r.cfg.Game.FinalMessage1
		r.sender.Send(chat, anim, formatter.ParseMode) //nolint:errcheck
	} else {
		r.sender.Send(chat, r.cfg.Game.FinalMessage1, formatter.ParseMode) //nolint:errcheck
	}

	time.Sleep(r.cfg.Timings.TaskInfoInterval)

	if r.cfg.Game.FinalMessage2ButtonText != "" && r.cfg.Game.FinalMessage2ButtonURL != "" {
		kb := &tele.ReplyMarkup{}
		kb.Inline(kb.Row(kb.URL(r.cfg.Game.FinalMessage2ButtonText, r.cfg.Game.FinalMessage2ButtonURL)))
		r.sender.Send(chat, r.cfg.Game.FinalMessage2, formatter.ParseMode, kb) //nolint:errcheck
	} else {
		r.sender.Send(chat, r.cfg.Game.FinalMessage2, formatter.ParseMode) //nolint:errcheck
	}

	r.log.Info().Str("chat", logger.ChatValue(game.ChatID, game.ChatName)).Uint64("game", game.ID).Msg("game finished")

	return nil
}
