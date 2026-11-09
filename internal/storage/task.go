package storage

import "time"

const (
	TaskStatusOpen       = "OPEN"
	TaskStatusInProgress = "IN_PROGRESS"
	TaskStatusCompleted  = "COMPLETED"
)

type Task struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"-"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TaskPatch struct {
	Title       *string
	Description *string
	Status      *string
}
