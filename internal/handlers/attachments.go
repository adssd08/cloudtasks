package handlers

import (
	"context"
	"errors"
	"log"
	"time"

	"example.com/cloudtasks/internal/identity"
	"example.com/cloudtasks/internal/services"
	"example.com/cloudtasks/internal/storage"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type AttachmentHandler struct {
	service *services.AttachmentService
}

func NewAttachmentHandler(service *services.AttachmentService) *AttachmentHandler {
	return &AttachmentHandler{service: service}
}

func (h *AttachmentHandler) Confirm(c fiber.Ctx) error {
	ownerID, ok := identity.OwnerID(c)

	if !ok {
		return fiber.ErrUnauthorized
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "attachment ID must be a UUID")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	attachment, err := h.service.Confirm(ctx, id.String(), ownerID)
	if err != nil {
		return attachmentHTTPError(err)
	}

	return c.JSON(fiber.Map{
		"id":     attachment.ID,
		"status": attachment.Status,
	})
}

func attachmentHTTPError(err error) error {
	switch {
	case errors.Is(err, storage.ErrAttachmentNotFound):
		return fiber.NewError(fiber.StatusNotFound, "attachment not found")

	case errors.Is(err, storage.ErrObjectNotFound):
		return fiber.NewError(fiber.StatusConflict, "attachment has not been uploaded")

	case errors.Is(err, services.ErrAttachmentNotReady):
		return fiber.NewError(fiber.StatusConflict, "attachment is not ready")

	case errors.Is(err, services.ErrInvalidAttachment):
		return fiber.NewError(fiber.StatusUnprocessableEntity, "attachment must be 1 byte to 8 MiB and an allowed file type")

	case errors.Is(err, context.DeadlineExceeded):
		log.Printf("attachment operation timed out: %v", err)
		return fiber.NewError(fiber.StatusGatewayTimeout, "attachment operation timed out")

	default:
		log.Printf("attachment operation failed: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "attachment operation failed")
	}

}
