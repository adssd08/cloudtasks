package handlers

import (
	"context"
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const attachmentURLTTL = 5 * time.Minute

func (h *AttachmentHandler) PresignUpload(c fiber.Ctx) error {
	id := uuid.NewString()
	key := "attachments/" + id

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	request, err := h.presigner.PresignUpload(ctx, key, attachmentURLTTL)
	if err != nil {
		log.Printf("presign attachment upload: %v", err)

		return fiber.NewError(
			fiber.StatusInternalServerError,
			"could not generate upload URL",
		)
	}

	c.Set("Cache-Control", "no-store")

	return c.JSON(fiber.Map{
		"id":                 id,
		"key":                key,
		"expires_in_seconds": int(attachmentURLTTL.Seconds()),
		"request":            request,
	})
}

func (h *AttachmentHandler) PresignDownload(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))

	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "attachment ID must be UUID")
	}

	key := "attachments/" + id.String()

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	request, err := h.presigner.PresignDownload(ctx, key, attachmentURLTTL)
	if err != nil {
		log.Printf("presign attachment download: %v", err)

		return fiber.NewError(fiber.StatusInternalServerError, "could not generate download URL")
	}

	c.Set("Cache-Control", "no-store")

	return c.JSON(fiber.Map{
		"id":                 id.String(),
		"key":                key,
		"expires_in_seconds": int(attachmentURLTTL.Seconds()),
		"request":            request,
	})
}
