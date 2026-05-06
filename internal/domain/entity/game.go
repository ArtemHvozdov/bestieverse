package entity

import "time"

type GameStatus string

const (
	GamePending  GameStatus = "pending"
	GameActive   GameStatus = "active"
	GameFinished GameStatus = "finished"
)

type Game struct {
	ID                     uint64
	ChatID                 int64
	ChatName               string
	AdminUserID            int64
	AdminUsername          string
	Status                 GameStatus
	CurrentTaskOrder       int
	CurrentTaskPublishedAt *time.Time
	ActivePollID           string
	CreatedAt              time.Time
	StartedAt              *time.Time
	FinishedAt             *time.Time
}
