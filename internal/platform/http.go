package platform

import (
	"example.com/cloudtasks/internal/handlers"
	"example.com/cloudtasks/internal/storage"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
)

func NewHTTPApp(store storage.ObjectStore) *fiber.App {
	app := fiber.New(fiber.Config{
		BodyLimit: 8 * 1024 * 1024,
	})

	attachments := handlers.NewAttachmentHandler(store)

	app.Use(logger.New())
	app.Get("/health", handlers.Health)
	app.Post("/attachments", attachments.Upload)

	return app

}
