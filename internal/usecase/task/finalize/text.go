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

// TextFinalizer handles summary.type == "text".
// It simply sends task.Summary.Text to the chat.
type TextFinalizer struct {
	taskResultRepo repository.TaskResultRepository
	sender         Sender
}

func NewTextFinalizer(taskResultRepo repository.TaskResultRepository, sender Sender) *TextFinalizer {
	return &TextFinalizer{taskResultRepo: taskResultRepo, sender: sender}
}

func (f *TextFinalizer) SupportedSummaryType() string { return SummaryTypeText }

func (f *TextFinalizer) Finalize(
	ctx context.Context,
	game *entity.Game,
	task *config.Task,
	responses []*entity.TaskResponse,
) error {
	chat := &tele.Chat{ID: game.ChatID}
	f.sender.Send(chat, task.Summary.Text, formatter.ParseMode) //nolint:errcheck

	resultData, _ := json.Marshal(map[string]string{"type": "text"})
	if err := f.taskResultRepo.Create(ctx, &entity.TaskResult{
		GameID:      game.ID,
		TaskID:      task.ID,
		ResultData:  resultData,
		FinalizedAt: time.Now(),
	}); err != nil {
		return fmt.Errorf("text.Finalize: save result: %w", err)
	}

	return nil
}
