package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"example.com/cloudtasks/internal/storage"
	"github.com/google/uuid"
)

var ErrInvalidTaskInput = errors.New("invalid task input")

type CreateTaskInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type PatchTaskInput struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

type TaskService struct {
	repository *storage.TaskRepository
}

func NewTaskService(repository *storage.TaskRepository) *TaskService {
	return &TaskService{repository: repository}
}

func (s *TaskService) Create(ctx context.Context, ownerID string, input CreateTaskInput) (storage.Task, error) {
	title, err := normalizeTaskTitle(input.Title)
	if err != nil {
		return storage.Task{}, err
	}

	if err := validateTaskDescription(input.Description); err != nil {
		return storage.Task{}, err
	}

	return s.repository.Create(ctx, uuid.NewString(), ownerID, title, input.Description)
}

func (s *TaskService) Get(ctx context.Context, id string, ownerID string) (storage.Task, error) {
	return s.repository.GetOwned(ctx, id, ownerID)
}

func (s *TaskService) List(ctx context.Context, ownerID string, limit int, offset int) ([]storage.Task, error) {
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("%w: limit must be between 1 and 100", ErrInvalidTaskInput)
	}

	if offset < 0 {
		return nil, fmt.Errorf("%w: offset must not be negative", ErrInvalidTaskInput)
	}

	return s.repository.ListOwned(ctx, ownerID, limit, offset)
}

func (s *TaskService) Patch(ctx context.Context, id string, ownerID string, input PatchTaskInput) (storage.Task, error) {
	if input.Title == nil && input.Description == nil && input.Status == nil {
		return storage.Task{}, fmt.Errorf("%w: supply at least one field", ErrInvalidTaskInput)
	}

	patch := storage.TaskPatch{Description: input.Description, Status: input.Status}

	if input.Title != nil {
		title, err := normalizeTaskTitle(*input.Title)
		if err != nil {
			return storage.Task{}, err
		}
		patch.Title = &title
	}

	if input.Description != nil {
		if err := validateTaskDescription(*input.Description); err != nil {
			return storage.Task{}, err
		}
	}

	if input.Status != nil {
		switch *input.Status {
		case storage.TaskStatusOpen, storage.TaskStatusInProgress, storage.TaskStatusCompleted:
		default:
			return storage.Task{}, fmt.Errorf("%w: unsupported status", ErrInvalidTaskInput)
		}
	}

	return s.repository.PatchOwned(ctx, id, ownerID, patch)
}

func normalizeTaskTitle(title string) (string, error) {
	if !utf8.ValidString(title) {
		return "", fmt.Errorf("%w: title must be valid UTF-8", ErrInvalidTaskInput)
	}

	title = strings.TrimSpace(title)
	length := utf8.RuneCountInString(title)

	if length < 1 || length > 200 {
		return "", fmt.Errorf("%w: title must contain 1 to 200 characters", ErrInvalidTaskInput)
	}

	return title, nil
}

func validateTaskDescription(description string) error {
	if !utf8.ValidString(description) {
		return fmt.Errorf("%w: description must be valid UTF-8", ErrInvalidTaskInput)
	}

	if utf8.RuneCountInString(description) > 5000 {
		return fmt.Errorf("%w: description must not exceed 5000 characters", ErrInvalidTaskInput)
	}

	return nil
}
