package handlers

import (
	"context"
	"time"

	"example.com/cloudtasks/internal/identity"
	"example.com/cloudtasks/internal/services"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

const attachmentURLTTL = 5 * time.Minute

func (h *AttachmentHandler) PresignUpload(c fiber.Ctx) error {
	ownerID, ok := identity.OwnerID(c)
	if !ok {
		return fiber.ErrUnauthorized
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	grant, err := h.service.CreateUpload(ctx, ownerID)

	if err != nil {
		return attachmentHTTPError(err)
	}

	return c.JSON(fiber.Map{
		"id":                 grant.ID,
		"key":                grant.Key,
		"status":             "PENDING",
		"expires_in_seconds": grant.ExpiresInSeconds,
		"request":            grant.Request,
	})
}

func (h *AttachmentHandler) PresignDownload(c fiber.Ctx) error {
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

	request, err := h.service.DownloadURL(ctx, id.String(), ownerID)

	if err != nil {
		return attachmentHTTPError(err)
	}

	return c.JSON(fiber.Map{
		"id":                 id.String(),
		"expires_in_seconds": int(services.AttachmentURLTTL.Seconds()),
		"request":            request,
	})
}
