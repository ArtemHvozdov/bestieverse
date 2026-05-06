package finalize

import (
	"context"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	tele "gopkg.in/telebot.v3"
)

// TextFinalizer handles summary.type == "text".
// It simply sends task.Summary.Text to the chat.
type TextFinalizer struct {
	sender Sender
}

func NewTextFinalizer(sender Sender) *TextFinalizer {
	return &TextFinalizer{sender: sender}
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
	return nil
}
