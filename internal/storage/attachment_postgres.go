package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrAttachmentNotFound = errors.New("attachment not found")

type AttachmentRepository struct {
	db *pgxpool.Pool
}

func NewAttachmentRepository(db *pgxpool.Pool) *AttachmentRepository {
	return &AttachmentRepository{db: db}
}

func (r *AttachmentRepository) CreatePending(ctx context.Context, id string, ownerID string, objectKey string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO attachments (id, owner_id, object_key)
		VALUES ($1, $2, $3)
	`, id, ownerID, objectKey)

	if err != nil {
		return fmt.Errorf("create pending attachment: %w", err)
	}

	return nil
}

func (r *AttachmentRepository) GetOwned(ctx context.Context, id string, ownerID string) (Attachment, error) {
	var attachment Attachment

	err := r.db.QueryRow(ctx, `
		SELECT
			id::text,
			owner_id::text,
			object_key,
			status,
			created_at
		FROM attachments
		WHERE id = $1 AND owner_id = $2
	`, id, ownerID).Scan(&attachment.ID, &attachment.OwnerID, &attachment.ObjectKey, &attachment.Status, &attachment.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return Attachment{}, ErrAttachmentNotFound
	}

	if err != nil {
		return Attachment{}, fmt.Errorf("get owned attachment: %w", err)
	}

	return attachment, nil
}

func (r *AttachmentRepository) MarkReady(ctx context.Context, id string, ownerID string, sizeBytes int64, contentType string) (bool, error) {
	result, err := r.db.Exec(ctx, `
		UPDATE attachments
		SET
			status = 'READY',
			size_bytes = $3,
			content_type = $4,
			confirmed_at = now()
		WHERE
			id = $1
			AND owner_id = $2
			AND status = 'PENDING'
	`, id, ownerID, sizeBytes, contentType)

	if err != nil {
		return false, fmt.Errorf("confirm attachment: %w", err)
	}

	return result.RowsAffected() == 1, nil
}
