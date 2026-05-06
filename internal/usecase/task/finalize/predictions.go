package finalize

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	tele "gopkg.in/telebot.v3"
)

// PredictionsFinalizer handles summary.type == "predictions".
// It sends a header, then a personal prediction for each responding player.
type PredictionsFinalizer struct {
	playerRepo     repository.PlayerRepository
	taskResultRepo repository.TaskResultRepository
	sender         Sender
}

func NewPredictionsFinalizer(
	playerRepo repository.PlayerRepository,
	taskResultRepo repository.TaskResultRepository,
	sender Sender,
) *PredictionsFinalizer {
	return &PredictionsFinalizer{
		playerRepo:     playerRepo,
		taskResultRepo: taskResultRepo,
		sender:         sender,
	}
}

func (f *PredictionsFinalizer) SupportedSummaryType() string { return SummaryTypePredictions }

func (f *PredictionsFinalizer) Finalize(
	ctx context.Context,
	game *entity.Game,
	task *config.Task,
	responses []*entity.TaskResponse,
) error {
	// Build player map for quick lookup.
	allPlayers, err := f.playerRepo.GetAllByGame(ctx, game.ID)
	if err != nil {
		return fmt.Errorf("predictions.Finalize: get players: %w", err)
	}
	playerByID := make(map[uint64]*entity.Player, len(allPlayers))
	for _, p := range allPlayers {
		playerByID[p.ID] = p
	}

	chat := &tele.Chat{ID: game.ChatID}

	f.sender.Send(chat, task.Summary.HeaderText, formatter.ParseMode) //nolint:errcheck

	for _, resp := range responses {
		player, ok := playerByID[resp.PlayerID]
		if !ok {
			continue
		}
		mention := formatter.Mention(player.TelegramUserID, player.Username, player.FirstName)
		prediction := config.Random(task.Summary.Predictions)
		text, err := formatter.RenderTemplate(prediction, struct{ Mention string }{Mention: mention})
		if err != nil {
			text = prediction
		}
		f.sender.Send(chat, text, formatter.ParseMode) //nolint:errcheck
	}

	resultData, _ := json.Marshal(map[string]string{"type": "predictions"})
	if err := f.taskResultRepo.Create(ctx, &entity.TaskResult{
		GameID:      game.ID,
		TaskID:      task.ID,
		ResultData:  resultData,
		FinalizedAt: time.Now(),
	}); err != nil {
		return fmt.Errorf("predictions.Finalize: save result: %w", err)
	}

	return nil
}
