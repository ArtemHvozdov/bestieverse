package entity

import "time"

type NotificationLog struct {
	ID       uint64
	GameID   uint64
	PlayerID uint64
	TaskID   string
	SentAt   time.Time
}
