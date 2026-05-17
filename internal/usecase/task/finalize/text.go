package finalize

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/infrastructure/media"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	tele "gopkg.in/telebot.v3"
)

// TextFinalizer handles summary.type == "text".
// It sends task.Summary.Text and, if configured, an attachment file.
type TextFinalizer struct {
	taskResultRepo repository.TaskResultRepository
	storage        media.Storage
	sender         Sender
}

func NewTextFinalizer(
	taskResultRepo repository.TaskResultRepository,
	storage media.Storage,
	sender Sender,
) *TextFinalizer {
	return &TextFinalizer{taskResultRepo: taskResultRepo, storage: storage, sender: sender}
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

	if task.Summary.AttachmentFile != "" {
		doc, err := f.storage.GetFile(task.Summary.AttachmentFile)
		if err != nil {
			return fmt.Errorf("text.Finalize: get attachment: %w", err)
		}
		if task.Summary.AttachmentName != "" {
			doc.FileName = task.Summary.AttachmentName
		}
		f.sender.Send(chat, doc) //nolint:errcheck
	}

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
