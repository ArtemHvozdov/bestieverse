package entity

import (
	"encoding/json"
	"time"
)

type ResponseStatus string

const (
	ResponseAnswered ResponseStatus = "answered"
	ResponseSkipped  ResponseStatus = "skipped"
)

type TaskResponse struct {
	ID           uint64
	GameID       uint64
	PlayerID     uint64
	TaskID       string
	Status       ResponseStatus
	ResponseData json.RawMessage
	CreatedAt    time.Time
}
