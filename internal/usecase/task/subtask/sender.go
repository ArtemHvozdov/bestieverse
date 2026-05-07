package subtask

import (
	"time"

	tele "gopkg.in/telebot.v3"
)

// Sender is the minimal bot interface required by subtask usecases.
// *tele.Bot satisfies this interface.
type Sender interface {
	Send(to tele.Recipient, what interface{}, opts ...interface{}) (*tele.Message, error)
	Delete(msg tele.Editable) error
}

func deleteAfter(s Sender, msg *tele.Message, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		_ = s.Delete(msg)
	}()
}
