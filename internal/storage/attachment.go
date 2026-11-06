package storage

import "time"

type Attachment struct {
	ID        string
	OwnerID   string
	ObjectKey string
	Status    string
	CreatedAt time.Time
}
