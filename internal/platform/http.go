package platform

import (
	"example.com/cloudtasks/internal/handlers"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
)

func NewHTTPApp() *fiber.App {
	app := fiber.New()

	app.Use(logger.New())
	app.Get("/health", handlers.Health)

	return app

}
