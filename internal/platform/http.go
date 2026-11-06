package platform

import (
	"example.com/cloudtasks/internal/handlers"
	"example.com/cloudtasks/internal/services"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
)

func NewHTTPApp(service *services.AttachmentService, auth fiber.Handler) *fiber.App {
	app := fiber.New(fiber.Config{
		BodyLimit: 8 * 1024 * 1024,
	})

	attachments := handlers.NewAttachmentHandler(service)

	app.Use(logger.New())
	app.Get("/health", handlers.Health)

	protected := app.Group(
		"/attachments", func(c fiber.Ctx) error {
			c.Set("Cache-Control", "no-store")
			return c.Next()
		},
		auth,
	)

	protected.Post("/presign-upload", attachments.PresignUpload)
	protected.Post("/:id/confirm", attachments.Confirm)
	protected.Get("/:id/presign-download", attachments.PresignDownload)

	return app

}
