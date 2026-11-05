package handlers

import (
	"bytes"
	"context"
	"log"
	"time"

	"example.com/cloudtasks/internal/storage"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type AttachmentHandler struct {
	store     storage.ObjectStore
	presigner storage.ObjectPresigner
}

func NewAttachmentHandler(store storage.ObjectStore, presigner storage.ObjectPresigner) *AttachmentHandler {
	return &AttachmentHandler{store: store, presigner: presigner}
}

func (h *AttachmentHandler) Upload(c fiber.Ctx) error {
	body := c.Body()
	if len(body) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "attachment body must not be empty")
	}

	key := "attachments/" + uuid.NewString()
	contentType := c.Get("Content-Type", "application/octet-stream")

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	err := h.store.Put(ctx, key, bytes.NewReader(body), contentType)
	if err != nil {
		log.Printf("upload attachment: %v", err)

		return fiber.NewError(fiber.StatusBadGateway, "attachment storage request failed")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"key":  key,
		"size": len(body),
	})
}
