package entity

import "time"

type PlayerStateType string

const (
	PlayerStateIdle            PlayerStateType = "idle"
	PlayerStateAwaitingAnswer  PlayerStateType = "awaiting_answer"
)

type PlayerState struct {
	ID        uint64
	GameID    uint64
	PlayerID  uint64
	State     PlayerStateType
	TaskID    string
	UpdatedAt time.Time
}
