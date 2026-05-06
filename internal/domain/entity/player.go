package entity

import "time"

type Player struct {
	ID             uint64
	GameID         uint64
	TelegramUserID int64
	Username       string
	FirstName      string
	SkipCount      int
	JoinedAt       time.Time
}
