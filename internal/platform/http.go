package platform

import (
	"example.com/cloudtasks/internal/handlers"
	"example.com/cloudtasks/internal/services"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
)

func NewHTTPApp(attachmentService *services.AttachmentService, taskService *services.TaskService, auth fiber.Handler) *fiber.App {
	app := fiber.New(fiber.Config{
		BodyLimit: 8 * 1024 * 1024,
	})

	attachments := handlers.NewAttachmentHandler(attachmentService)
	tasks := handlers.NewTaskHandler(taskService)

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

	taskRoutes := app.Group(
		"/tasks", func(c fiber.Ctx) error {
			c.Set("Cache-Control", "no-store")
			return c.Next()
		},
		handlers.TaskErrorResponse,
		auth,
	)

	taskRoutes.Post("/", tasks.Create)
	taskRoutes.Get("/", tasks.List)
	taskRoutes.Get("/:id", tasks.Get)
	taskRoutes.Patch("/:id", tasks.Patch)

	return app

}
