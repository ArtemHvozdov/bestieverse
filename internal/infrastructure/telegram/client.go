package telegram

import (
	"time"

	tele "gopkg.in/telebot.v3"
)

// NewBot creates and returns a new telebot Bot instance.
func NewBot(token string, settings tele.Settings) (*tele.Bot, error) {
	settings.Token = token
	return tele.NewBot(settings)
}

// DeleteAfter schedules deletion of msg after the given delay.
// It runs in a separate goroutine and silently ignores delete errors.
func DeleteAfter(bot *tele.Bot, msg *tele.Message, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		_ = bot.Delete(msg)
	}()
}
