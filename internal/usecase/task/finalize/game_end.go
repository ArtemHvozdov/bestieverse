package finalize

import (
	"context"
	"fmt"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
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

	r.sender.Send(chat, r.cfg.Game.FinalMessage2, formatter.ParseMode) //nolint:errcheck

	r.log.Info().Int64("chat", game.ChatID).Uint64("game", game.ID).Msg("game finished")

	return nil
}
