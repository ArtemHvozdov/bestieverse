package handler

import (
	"context"

	"github.com/ArtemHvozdov/bestieverse.git/internal/config"
	"github.com/ArtemHvozdov/bestieverse.git/internal/delivery/bot/keyboard"
	"github.com/ArtemHvozdov/bestieverse.git/internal/domain/entity"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/game"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task"
	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task/subtask"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// CallbackHandler routes inline button callbacks to the appropriate usecases.
type CallbackHandler struct {
	joiner               *game.Joiner
	leaver               *game.Leaver
	starter              *game.Starter
	requestAnswerer      *task.RequestAnswerer
	skipper              *task.Skipper
	votingCollageHandler *subtask.VotingCollageHandler
	cfg                  *config.Config
	log                  zerolog.Logger
}

func NewCallbackHandler(
	joiner *game.Joiner,
	leaver *game.Leaver,
	starter *game.Starter,
	requestAnswerer *task.RequestAnswerer,
	skipper *task.Skipper,
	votingCollageHandler *subtask.VotingCollageHandler,
	cfg *config.Config,
	log zerolog.Logger,
) *CallbackHandler {
	return &CallbackHandler{
		joiner:               joiner,
		leaver:               leaver,
		starter:              starter,
		requestAnswerer:      requestAnswerer,
		skipper:              skipper,
		votingCollageHandler: votingCollageHandler,
		cfg:                  cfg,
		log:                  log,
	}
}

func (h *CallbackHandler) OnJoin(c tele.Context) error {
	return h.joiner.Join(context.Background(), c.Chat().ID, *c.Sender())
}

func (h *CallbackHandler) OnLeave(c tele.Context) error {
	g := c.Get("game").(*entity.Game)
	p := c.Get("player").(*entity.Player)
	return h.leaver.InitiateLeave(context.Background(), g, p, keyboard.LeaveConfirmKeyboard())
}

func (h *CallbackHandler) OnLeaveConfirm(c tele.Context) error {
	g := c.Get("game").(*entity.Game)
	p := c.Get("player").(*entity.Player)
	return h.leaver.ConfirmLeave(context.Background(), g, p, c.Message())
}

func (h *CallbackHandler) OnLeaveCancel(c tele.Context) error {
	g := c.Get("game").(*entity.Game)
	p := c.Get("player").(*entity.Player)
	return h.leaver.CancelLeave(context.Background(), g, p, c.Message())
}

func (h *CallbackHandler) OnStart(c tele.Context) error {
	g := c.Get("game").(*entity.Game)
	p := c.Get("player").(*entity.Player)
	return h.starter.Start(context.Background(), g, p, c.Message())
}

// OnTaskRequestAnswer handles the "Хочу відповісти" button press.
// c.Data() contains the taskID.
// Routes to the appropriate handler depending on the task's subtask type.
func (h *CallbackHandler) OnTaskRequestAnswer(c tele.Context) error {
	taskID := c.Data()
	g := c.Get("game").(*entity.Game)
	p := c.Get("player").(*entity.Player)
	t := h.cfg.TaskByID(taskID)
	if t == nil {
		h.log.Warn().Str("task_id", taskID).Msg("unknown task in request_answer callback")
		return nil
	}
	if t.Subtask != nil && t.Subtask.Type == "voting_collage" {
		return h.votingCollageHandler.HandleRequestAnswer(context.Background(), g, p, t)
	}
	return h.requestAnswerer.RequestAnswer(context.Background(), g, p, t)
}

// OnTaskSkip handles the "Пропустити" button press.
// c.Data() contains the taskID.
func (h *CallbackHandler) OnTaskSkip(c tele.Context) error {
	taskID := c.Data()
	g := c.Get("game").(*entity.Game)
	p := c.Get("player").(*entity.Player)
	return h.skipper.Skip(context.Background(), g, p, taskID)
}

// OnTask02Choice handles a category option selection in the voting_collage subtask.
// c.Data() contains "categoryID:optionID".
func (h *CallbackHandler) OnTask02Choice(c tele.Context) error {
	g := c.Get("game").(*entity.Game)
	p := c.Get("player").(*entity.Player)
	t := h.cfg.TaskByID("task_02")
	if t == nil {
		h.log.Warn().Msg("task02:choice callback but task_02 not found in config")
		return nil
	}
	categoryID, optionID, err := subtask.ParseCategoryChoice(c.Data())
	if err != nil {
		h.log.Warn().Err(err).Str("data", c.Data()).Msg("task02:choice: invalid data")
		return nil
	}
	return h.votingCollageHandler.HandleCategoryChoice(context.Background(), g, p, t, categoryID, optionID)
}
