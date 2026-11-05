package platform

import (
	"example.com/cloudtasks/internal/handlers"
	"example.com/cloudtasks/internal/storage"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
)

func NewHTTPApp(store storage.ObjectStore, presigner storage.ObjectPresigner) *fiber.App {
	app := fiber.New(fiber.Config{
		BodyLimit: 8 * 1024 * 1024,
	})

	attachments := handlers.NewAttachmentHandler(store, presigner)

	app.Use(logger.New())
	app.Get("/health", handlers.Health)
	app.Post("/attachments", attachments.Upload)
	app.Get("/attachments/:id/presign-download", attachments.PresignDownload)
	app.Post("/attachments/presign-upload", attachments.PresignUpload)

	return app

}
