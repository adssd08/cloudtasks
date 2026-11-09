package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrTaskNotFound = errors.New("task not found")

type TaskRepository struct {
	db *pgxpool.Pool
}

func NewTaskRepository(db *pgxpool.Pool) *TaskRepository {
	return &TaskRepository{db: db}
}

func scanTask(row pgx.Row) (Task, error) {
	var task Task

	err := row.Scan(&task.ID, &task.OwnerID, &task.Title, &task.Description, &task.Status, &task.CreatedAt, &task.UpdatedAt)

	if err != nil {
		return Task{}, err
	}

	return task, nil
}

func (r *TaskRepository) Create(ctx context.Context, id string, ownerID string, title string, description string) (Task, error) {
	task, err := scanTask(r.db.QueryRow(ctx, `
		INSERT INTO tasks (
			id,
			owner_id,
			title,
			description
		)
		VALUES ($1, $2, $3, $4)
		RETURNING
			id::text,
			owner_id::text,
			title,
			description,
			status,
			created_at,
			updated_at
	`, id, ownerID, title, description))

	if err != nil {
		return Task{}, fmt.Errorf("create task: %w", err)
	}

	return task, nil
}

func (r *TaskRepository) GetOwned(ctx context.Context, id string, ownerID string) (Task, error) {
	task, err := scanTask(r.db.QueryRow(ctx, `
		SELECT
			id::text,
			owner_id::text,
			title,
			description,
			status,
			created_at,
			updated_at
		FROM tasks
		WHERE id = $1 AND owner_id = $2
	`, id, ownerID))

	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}

	if err != nil {
		return Task{}, fmt.Errorf("get owned task: %w", err)
	}

	return task, nil
}

func (r *TaskRepository) ListOwned(ctx context.Context, ownerID string, limit int, offset int) ([]Task, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			id::text,
			owner_id::text,
			title,
			description,
			status,
			created_at,
			updated_at
		FROM tasks
		WHERE owner_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, ownerID, limit, offset)

	if err != nil {
		return nil, fmt.Errorf("list owned tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]Task, 0)

	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}

	return tasks, nil

}

func (r *TaskRepository) PatchOwned(ctx context.Context, id string, ownerID string, patch TaskPatch) (Task, error) {
	task, err := scanTask(r.db.QueryRow(ctx, `
		UPDATE tasks
		SET
			title = COALESCE($3::text, title),
			description = COALESCE($4::text, description),
			status = COALESCE($5::text, status),
			updated_at = now()
		WHERE id = $1 AND owner_id = $2
		RETURNING
			id::text,
			owner_id::text,
			title,
			description,
			status,
			created_at,
			updated_at
	`, id, ownerID, patch.Title, patch.Description, patch.Status))

	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}

	if err != nil {
		return Task{}, fmt.Errorf("update owned tasks: %w", err)
	}

	return task, nil
}
