package finalize_test

import (
	"context"
	"testing"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task/finalize"
)

func TestTextFinalizer_SupportedSummaryType(t *testing.T) {
	f := finalize.NewTextFinalizer(&mockSender{})
	if f.SupportedSummaryType() != finalize.SummaryTypeText {
		t.Errorf("expected %q, got %q", finalize.SummaryTypeText, f.SupportedSummaryType())
	}
}

func TestTextFinalizer_SendsSummaryText(t *testing.T) {
	sender := &mockSender{}
	f := finalize.NewTextFinalizer(sender)

	game := &entity.Game{ID: 1, ChatID: 100}
	task := &config.Task{
		ID:      "task_01",
		Summary: config.TaskSummary{Type: "text", Text: "The result text"},
	}

	err := f.Finalize(context.Background(), game, task, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sender.sent))
	}
	if sender.sent[0] != "The result text" {
		t.Errorf("expected %q, got %q", "The result text", sender.sent[0])
	}
}
