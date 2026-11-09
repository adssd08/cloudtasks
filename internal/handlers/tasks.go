package handlers

import (
	"context"
	"errors"
	"log"
	"strconv"
	"time"

	"example.com/cloudtasks/internal/identity"
	"example.com/cloudtasks/internal/services"
	"example.com/cloudtasks/internal/storage"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type TaskHandler struct {
	service *services.TaskService
}

func NewTaskHandler(service *services.TaskService) *TaskHandler {
	return &TaskHandler{service: service}
}

func (h *TaskHandler) Create(c fiber.Ctx) error {
	ownerID, ok := identity.OwnerID(c)

	if !ok {
		return fiber.ErrUnauthorized
	}

	var input services.CreateTaskInput
	if err := decodeTaskJSON(c, &input); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	task, err := h.service.Create(ctx, ownerID, input)
	if err != nil {
		return taskHTTPError(err)
	}

	c.Set("Location", "/tasks/"+task.ID)
	return c.Status(fiber.StatusCreated).JSON(task)
}

func (h *TaskHandler) Get(c fiber.Ctx) error {
	ownerID, ok := identity.OwnerID(c)
	if !ok {
		return fiber.ErrUnauthorized
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "task ID must be a UUID")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	task, err := h.service.Get(ctx, id.String(), ownerID)
	if err != nil {
		return taskHTTPError(err)
	}

	return c.JSON(task)
}

func (h *TaskHandler) List(c fiber.Ctx) error {
	ownerID, ok := identity.OwnerID(c)

	if !ok {
		return fiber.ErrUnauthorized
	}

	limit, err := strconv.Atoi(c.Query("limit", "20"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "limit must be an integer")
	}

	offset, err := strconv.Atoi(c.Query("offset", "0"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "offset must be an integer")
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	tasks, err := h.service.List(ctx, ownerID, limit, offset)
	if err != nil {
		return taskHTTPError(err)
	}

	return c.JSON(fiber.Map{
		"tasks":  tasks,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *TaskHandler) Patch(c fiber.Ctx) error {
	ownerID, ok := identity.OwnerID(c)

	if !ok {
		return fiber.ErrUnauthorized
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "task ID must be a UUID")
	}

	var input services.PatchTaskInput
	if err := decodeTaskJSON(c, &input); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	task, err := h.service.Patch(ctx, id.String(), ownerID, input)
	if err != nil {
		return taskHTTPError(err)
	}

	return c.JSON(task)

}

func taskHTTPError(err error) error {
	switch {
	case errors.Is(err, storage.ErrTaskNotFound):
		return fiber.NewError(fiber.StatusNotFound, "task not found")
	case errors.Is(err, services.ErrInvalidTaskInput):
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		log.Printf("task operation timed out: %v", err)
		return fiber.NewError(fiber.StatusGatewayTimeout, "task operation timed out")

	default:
		log.Printf("task operation failed: %v", err)
		return fiber.NewError(fiber.StatusInternalServerError, "task operation failed")
	}
}

func TaskErrorResponse(c fiber.Ctx) error {
	err := c.Next()

	if err == nil {
		return nil
	}

	code := fiber.StatusInternalServerError
	message := "task operation failed"

	var fiberErr *fiber.Error

	if errors.As(err, &fiberErr) {
		code = fiberErr.Code
		message = fiberErr.Message
	} else {
		log.Printf("unhandled task request failure: %v", err)
	}

	return c.Status(code).JSON(fiber.Map{
		"error": message,
	})
}
