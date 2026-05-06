package entity

import (
	"encoding/json"
	"time"
)

type TaskResult struct {
	ID          uint64
	GameID      uint64
	TaskID      string
	ResultData  json.RawMessage
	FinalizedAt time.Time
}
