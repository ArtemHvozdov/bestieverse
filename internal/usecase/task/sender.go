package task

import (
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"
)

// Sender is the minimal bot interface required by task usecases.
// *tele.Bot satisfies this interface.
type Sender interface {
	Send(to tele.Recipient, what interface{}, opts ...interface{}) (*tele.Message, error)
	Delete(msg tele.Editable) error
}

// deleteAfter schedules msg for deletion after delay in a background goroutine.
func deleteAfter(s Sender, msg *tele.Message, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		_ = s.Delete(msg)
	}()
}

// taskOrderFromID extracts the numeric order from a task ID like "task_03" → 3.
// Returns 0 if the format is unexpected.
func taskOrderFromID(taskID string) int {
	parts := strings.SplitN(taskID, "_", 2)
	if len(parts) != 2 {
		return 0
	}
	n, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0
	}
	return n
}
