package finalize

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/formatter"
	tele "gopkg.in/telebot.v3"
)

type WhoIsWhoFinalizer struct {
	playerRepo     repository.PlayerRepository
	taskResultRepo repository.TaskResultRepository
	sender         Sender
}

func NewWhoIsWhoFinalizer(
	playerRepo repository.PlayerRepository,
	taskResultRepo repository.TaskResultRepository,
	sender Sender,
) *WhoIsWhoFinalizer {
	return &WhoIsWhoFinalizer{
		playerRepo:     playerRepo,
		taskResultRepo: taskResultRepo,
		sender:         sender,
	}
}

func (f *WhoIsWhoFinalizer) SupportedSummaryType() string { return SummaryTypeWhoIsWho }

func (f *WhoIsWhoFinalizer) Finalize(
	ctx context.Context,
	game *entity.Game,
	task *config.Task,
	responses []*entity.TaskResponse,
) error {
	allPlayers, err := f.playerRepo.GetAllByGame(ctx, game.ID)
	if err != nil {
		return fmt.Errorf("who_is_who.Finalize: get players: %w", err)
	}

	// Build player lookup by TelegramUserID.
	playerByUID := make(map[int64]*entity.Player, len(allPlayers))
	for _, p := range allPlayers {
		playerByUID[p.TelegramUserID] = p
	}

	// Tally votes: questionID → telegramUserID → count.
	votes := make(map[string]map[int64]int)
	for _, resp := range responses {
		var answers map[string]int64
		if err := json.Unmarshal(resp.ResponseData, &answers); err != nil {
			continue
		}
		for qID, uid := range answers {
			if votes[qID] == nil {
				votes[qID] = make(map[int64]int)
			}
			votes[qID][uid]++
		}
	}

	// Find winner per question, respecting player insertion order for tie-breaking.
	results := make(map[string]int64, len(task.Questions))
	for _, q := range task.Questions {
		maxVotes := 0
		var winner *entity.Player
		for _, p := range allPlayers {
			if votes[q.ID][p.TelegramUserID] > maxVotes {
				maxVotes = votes[q.ID][p.TelegramUserID]
				winner = p
			}
		}
		if winner != nil {
			results[q.ID] = winner.TelegramUserID
		}
	}

	chat := &tele.Chat{ID: game.ChatID}

	f.sender.Send(chat, task.Summary.HeaderText, formatter.ParseMode) //nolint:errcheck

	// Build result lines: "question text → @mention of winner".
	var lines []string
	for _, q := range task.Questions {
		uid, ok := results[q.ID]
		if !ok {
			continue
		}
		p := playerByUID[uid]
		if p == nil {
			continue
		}
		mention := formatter.Mention(p.TelegramUserID, p.Username, p.FirstName)
		lines = append(lines, q.Text+" → "+mention)
	}
	if len(lines) > 0 {
		f.sender.Send(chat, strings.Join(lines, "\n"), formatter.ParseMode) //nolint:errcheck
	}

	resultData, _ := json.Marshal(results)
	if err := f.taskResultRepo.Create(ctx, &entity.TaskResult{
		GameID:      game.ID,
		TaskID:      task.ID,
		ResultData:  resultData,
		FinalizedAt: time.Now(),
	}); err != nil {
		return fmt.Errorf("who_is_who.Finalize: save result: %w", err)
	}

	return nil
}
