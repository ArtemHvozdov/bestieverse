package middleware

import (
	"github.com/rs/zerolog"
	tele "gopkg.in/telebot.v3"
)

// Recover returns a middleware that catches panics and logs them as errors.
func Recover(log zerolog.Logger) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					log.Error().Interface("panic", r).Msg("panic recovered")
				}
			}()
			return next(c)
		}
	}
}
