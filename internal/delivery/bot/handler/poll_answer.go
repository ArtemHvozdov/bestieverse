package handler

import (
	"context"

	"github.com/ArtemHvozdov/bestieverse.git/internal/usecase/task/subtask"
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// PollAnswerHandler processes Telegram poll update events.
type PollAnswerHandler struct {
	pollHandler *subtask.PollHandler
	log         zerolog.Logger
}

func NewPollAnswerHandler(pollHandler *subtask.PollHandler, log zerolog.Logger) *PollAnswerHandler {
	return &PollAnswerHandler{pollHandler: pollHandler, log: log}
}

// OnPoll is called by telebot on tele.OnPoll events.
// Ignores open polls; processes only closed ones.
func (h *PollAnswerHandler) OnPoll(c tele.Context) error {
	poll := c.Poll()
	if poll == nil || !poll.Closed {
		return nil
	}
	h.log.Info().Str("poll_id", poll.ID).Msg("processing closed poll")
	return h.pollHandler.HandlePollClosed(context.Background(), poll)
}
