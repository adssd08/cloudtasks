package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"time"

	"example.com/cloudtasks/internal/storage"
	"github.com/google/uuid"
)

const (
	MaxAttachmentBytes int64 = 8 * 1024 * 1024
	AttachmentURLTTL         = 5 * time.Minute
)

var (
	ErrAttachmentNotReady = errors.New("attachment is not ready")
	ErrInvalidAttachment  = errors.New("invalid attachment")
)

type UploadGrant struct {
	ID               string
	Key              string
	ExpiresInSeconds int
	Request          storage.PresignedRequest
}

type AttachmentService struct {
	repository *storage.AttachmentRepository
	store      storage.ObjectStore
	presigner  storage.ObjectPresigner
}

func NewAttachmentService(repository *storage.AttachmentRepository, store storage.ObjectStore, presigner storage.ObjectPresigner) *AttachmentService {
	return &AttachmentService{repository: repository, store: store, presigner: presigner}
}

func (s *AttachmentService) CreateUpload(ctx context.Context, ownerID string) (UploadGrant, error) {
	id := uuid.NewString()
	key := "attachments/" + id

	if err := s.repository.CreatePending(ctx, id, ownerID, key); err != nil {
		return UploadGrant{}, err
	}

	request, err := s.presigner.PresignUpload(ctx, key, AttachmentURLTTL)

	if err != nil {
		return UploadGrant{}, err
	}

	return UploadGrant{
		ID:               id,
		Key:              key,
		ExpiresInSeconds: int(AttachmentURLTTL.Seconds()),
		Request:          request,
	}, nil
}

func (s *AttachmentService) Confirm(ctx context.Context, id string, ownerID string) (storage.Attachment, error) {
	attachment, err := s.repository.GetOwned(ctx, id, ownerID)
	if err != nil {
		return storage.Attachment{}, err
	}

	if attachment.Status == "READY" {
		return attachment, nil
	}

	size, contentType, err := s.validateObject(ctx, attachment.ObjectKey)

	if err != nil {
		return storage.Attachment{}, err
	}

	updated, err := s.repository.MarkReady(ctx, id, ownerID, size, contentType)

	if err != nil {
		return storage.Attachment{}, err
	}

	if !updated {
		attachment, err = s.repository.GetOwned(ctx, id, ownerID)
		if err != nil {
			return storage.Attachment{}, err
		}

		if attachment.Status != "READY" {
			return storage.Attachment{}, ErrAttachmentNotReady
		}

		return attachment, nil
	}

	attachment.Status = "READY"
	return attachment, nil
}

func (s *AttachmentService) DownloadURL(ctx context.Context, id string, ownerID string) (storage.PresignedRequest, error) {
	attachment, err := s.repository.GetOwned(ctx, id, ownerID)

	if err != nil {
		return storage.PresignedRequest{}, err
	}

	if attachment.Status != "READY" {
		return storage.PresignedRequest{}, ErrAttachmentNotReady
	}

	return s.presigner.PresignDownload(ctx, attachment.ObjectKey, AttachmentURLTTL)
}

func (s *AttachmentService) validateObject(ctx context.Context, key string) (int64, string, error) {
	body, err := s.store.Get(ctx, key)

	if err != nil {
		return 0, "", err
	}

	data, readErr := io.ReadAll(
		io.LimitReader(body, MaxAttachmentBytes+1),
	)
	closeErr := body.Close()

	if readErr != nil {
		return 0, "", fmt.Errorf("read attachment for validation: %w", readErr)
	}

	if closeErr != nil {
		return 0, "", fmt.Errorf("close attachment body: %w", closeErr)
	}

	size := int64(len(data))

	if size == 0 || size > MaxAttachmentBytes {
		return 0, "", fmt.Errorf("%w: size must be between 1 byte and 8MiB", ErrInvalidAttachment)
	}

	detected := http.DetectContentType(data)

	contentType, _, err := mime.ParseMediaType(detected)
	if err != nil {
		return 0, "", fmt.Errorf("parse detected content type: %w", err)
	}

	switch contentType {
	case "image/png", "image/jpeg", "application/pdf", "text/plain":
		return size, contentType, nil
	default:
		return 0, "", fmt.Errorf("%w: unsupported content type %q", ErrInvalidAttachment, contentType)
	}

}
