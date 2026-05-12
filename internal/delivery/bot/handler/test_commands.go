package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/repository"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/notification"
	taskuc "github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task/finalize"
	"github.com/ArtemHvozdov/bestieverse.git/pkg/logger"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// TestCommandsHandler handles test-only bot commands (TEST_MODE=true).
type TestCommandsHandler struct {
	gameRepo         repository.GameRepository
	playerRepo       repository.PlayerRepository
	playerStateRepo  repository.PlayerStateRepository
	taskResponseRepo repository.TaskResponseRepository
	publisher        *taskuc.Publisher
	finalizeRouter   *finalize.FinalizeRouter
	reminderSender   *notification.ReminderSender
	bot              *tele.Bot
	cfg              *config.Config
	log              zerolog.Logger
}

func NewTestCommandsHandler(
	gameRepo repository.GameRepository,
	playerRepo repository.PlayerRepository,
	playerStateRepo repository.PlayerStateRepository,
	taskResponseRepo repository.TaskResponseRepository,
	publisher *taskuc.Publisher,
	finalizeRouter *finalize.FinalizeRouter,
	reminderSender *notification.ReminderSender,
	bot *tele.Bot,
	cfg *config.Config,
	log zerolog.Logger,
) *TestCommandsHandler {
	return &TestCommandsHandler{
		gameRepo:         gameRepo,
		playerRepo:       playerRepo,
		playerStateRepo:  playerStateRepo,
		taskResponseRepo: taskResponseRepo,
		publisher:        publisher,
		finalizeRouter:   finalizeRouter,
		reminderSender:   reminderSender,
		bot:              bot,
		cfg:              cfg,
		log:              log,
	}
}

// OnTestTask handles /test_task_N — publishes task N immediately.
// The command text should be in the form "/test_task_5".
func (h *TestCommandsHandler) OnTestTask(c tele.Context) error {
	ctx := context.Background()
	n, err := extractTaskNumber(c.Message().Text)
	if err != nil {
		return c.Send(fmt.Sprintf("usage: /test_task_N (got: %s)", c.Message().Text))
	}

	game, err := h.gameRepo.GetByChatID(ctx, c.Chat().ID)
	if err != nil {
		return fmt.Errorf("test_task: get game: %w", err)
	}
	if game == nil {
		return c.Send("no game in this chat")
	}

	// Set current_task_order to N-1 so Publish picks up task N.
	if err := h.gameRepo.UpdateCurrentTask(ctx, game.ID, n-1, time.Now()); err != nil {
		return fmt.Errorf("test_task: reset order: %w", err)
	}
	game.CurrentTaskOrder = n - 1

	if err := h.publisher.Publish(ctx, game); err != nil {
		return fmt.Errorf("test_task: publish: %w", err)
	}
	h.log.Info().Str("chat", logger.ChatValue(c.Chat().ID, c.Chat().Title)).Int("task", n).Msg("test: task published")
	return nil
}

// OnTestFinalize handles /test_finalize_N — finalizes task N immediately.
func (h *TestCommandsHandler) OnTestFinalize(c tele.Context) error {
	ctx := context.Background()
	n, err := extractTaskNumber(c.Message().Text)
	if err != nil {
		return c.Send(fmt.Sprintf("usage: /test_finalize_N (got: %s)", c.Message().Text))
	}

	game, err := h.gameRepo.GetByChatID(ctx, c.Chat().ID)
	if err != nil {
		return fmt.Errorf("test_finalize: get game: %w", err)
	}
	if game == nil {
		return c.Send("no game in this chat")
	}

	task := h.cfg.TaskByOrder(n)
	if task == nil {
		return c.Send(fmt.Sprintf("task %d not found", n))
	}

	if err := h.finalizeRouter.Finalize(ctx, game, task); err != nil {
		return fmt.Errorf("test_finalize: %w", err)
	}
	h.log.Info().Str("chat", logger.ChatValue(c.Chat().ID, c.Chat().Title)).Int("task", n).Msg("test: task finalized")
	return nil
}

// OnTestNotify handles /test_notify — sends reminders to all unanswered players.
func (h *TestCommandsHandler) OnTestNotify(c tele.Context) error {
	if err := h.reminderSender.SendReminders(context.Background()); err != nil {
		return fmt.Errorf("test_notify: %w", err)
	}
	h.log.Info().Str("chat", logger.ChatValue(c.Chat().ID, c.Chat().Title)).Msg("test: reminders sent")
	return nil
}

// OnTestState handles /test_state — sends a JSON dump of the current game state.
func (h *TestCommandsHandler) OnTestState(c tele.Context) error {
	ctx := context.Background()
	chatID := c.Chat().ID

	game, err := h.gameRepo.GetByChatID(ctx, chatID)
	if err != nil {
		return fmt.Errorf("test_state: get game: %w", err)
	}
	if game == nil {
		return c.Send("no game in this chat")
	}

	players, err := h.playerRepo.GetAllByGame(ctx, game.ID)
	if err != nil {
		return fmt.Errorf("test_state: get players: %w", err)
	}

	// Collect player states.
	type playerStateEntry struct {
		PlayerID uint64 `json:"player_id"`
		Username string `json:"username"`
		State    string `json:"state"`
		TaskID   string `json:"task_id,omitempty"`
	}
	var playerStates []playerStateEntry
	for _, p := range players {
		ps, err := h.playerStateRepo.GetByPlayerAndGame(ctx, game.ID, p.ID)
		if err != nil {
			continue
		}
		entry := playerStateEntry{PlayerID: p.ID, Username: p.Username}
		if ps != nil {
			entry.State = string(ps.State)
			entry.TaskID = ps.TaskID
		}
		playerStates = append(playerStates, entry)
	}

	// Last responses for the current task.
	var recentResponses interface{}
	if game.CurrentTaskOrder > 0 {
		task := h.cfg.TaskByOrder(game.CurrentTaskOrder)
		if task != nil {
			responses, err := h.taskResponseRepo.GetAllByTask(ctx, game.ID, task.ID)
			if err == nil {
				// Limit to 5.
				if len(responses) > 5 {
					responses = responses[len(responses)-5:]
				}
				recentResponses = responses
			}
		}
	}

	state := map[string]interface{}{
		"game":             game,
		"players":          players,
		"player_states":    playerStates,
		"recent_responses": recentResponses,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("test_state: marshal: %w", err)
	}

	// Telegram messages have a 4096 char limit; truncate if needed.
	text := string(data)
	if len(text) > 4000 {
		text = text[:4000] + "\n...(truncated)"
	}

	return c.Send("<pre>" + text + "</pre>", tele.ModeHTML)
}

// OnTestReset handles /test_reset — deletes the game and all related data (CASCADE).
func (h *TestCommandsHandler) OnTestReset(c tele.Context) error {
	ctx := context.Background()
	game, err := h.gameRepo.GetByChatID(ctx, c.Chat().ID)
	if err != nil {
		return fmt.Errorf("test_reset: get game: %w", err)
	}
	if game == nil {
		return c.Send("no game in this chat")
	}
	if err := h.gameRepo.Delete(ctx, game.ID); err != nil {
		return fmt.Errorf("test_reset: delete: %w", err)
	}
	h.log.Warn().Uint64("game", game.ID).Str("chat", logger.ChatValue(c.Chat().ID, c.Chat().Title)).Msg("test: game reset")
	return c.Send("game reset ✓")
}

// extractTaskNumber parses the task number from "/test_task_5" or "/test_finalize_5".
func extractTaskNumber(text string) (int, error) {
	parts := strings.Split(strings.Fields(text)[0], "_")
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid command")
	}
	numStr := parts[len(parts)-1]
	n, err := strconv.Atoi(numStr)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid task number: %s", numStr)
	}
	return n, nil
}
