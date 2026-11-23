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

	// Find winners per question: every player sharing the max vote count wins.
	// Winners keep player insertion order.
	results := make(map[string][]int64, len(task.Questions))
	for _, q := range task.Questions {
		maxVotes := 0
		var winners []int64
		for _, p := range allPlayers {
			n := votes[q.ID][p.TelegramUserID]
			if n == 0 {
				continue
			}
			if n > maxVotes {
				maxVotes = n
				winners = winners[:0]
			}
			if n == maxVotes {
				winners = append(winners, p.TelegramUserID)
			}
		}
		if len(winners) > 0 {
			results[q.ID] = winners
		}
	}

	chat := &tele.Chat{ID: game.ChatID}

	// Build result lines: "question text @mention1, @mention2", separated by blank lines.
	var lines []string
	for _, q := range task.Questions {
		var mentions []string
		for _, uid := range results[q.ID] {
			p := playerByUID[uid]
			if p == nil {
				continue
			}
			mentions = append(mentions, formatter.Mention(p.TelegramUserID, p.Username, p.FirstName))
		}
		if len(mentions) == 0 {
			continue
		}
		lines = append(lines, q.Text+" "+strings.Join(mentions, ", "))
	}

	// Combine header and results into a single message separated by a blank line.
	msg := task.Summary.HeaderText
	if len(lines) > 0 {
		msg += "\n" + strings.Join(lines, "\n\n")
	}
	f.sender.Send(chat, msg, formatter.ParseMode) //nolint:errcheck

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
