package entity

import "time"

type TaskLock struct {
	ID         uint64
	GameID     uint64
	TaskID     string
	PlayerID   uint64
	AcquiredAt time.Time
	ExpiresAt  time.Time
}
