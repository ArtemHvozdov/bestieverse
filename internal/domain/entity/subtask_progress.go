package entity

import (
	"encoding/json"
	"time"
)

type SubtaskProgress struct {
	ID            uint64
	GameID        uint64
	PlayerID      uint64
	TaskID        string
	QuestionIndex int
	AnswersData   json.RawMessage
	UpdatedAt     time.Time
}
