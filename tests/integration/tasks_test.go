//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"example.com/cloudtasks/internal/identity"
	"example.com/cloudtasks/internal/storage"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func (env *testEnvironment) taskRequest(
	t *testing.T,
	method string,
	path string,
	token string,
	body string,
	expectedStatus int,
) []byte {
	t.Helper()

	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := env.app.Test(request, fiber.TestConfig{
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal("task request failed")
	}
	defer response.Body.Close()

	var raw json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		t.Fatal("could not decode task response")
	}

	if response.StatusCode == http.StatusCreated {
		var task storage.Task
		if err := json.Unmarshal(raw, &task); err != nil || task.ID == "" {
			t.Fatal("created task response is incomplete")
		}

		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(
				context.Background(),
				5*time.Second,
			)
			defer cancel()

			_, err := env.db.Exec(ctx, `
				DELETE FROM tasks WHERE id = $1
			`, task.ID)
			if err != nil {
				t.Error("could not clean task fixture")
			}
		})
	}

	if response.StatusCode != expectedStatus {
		t.Fatalf(
			"%s %s: expected %d, got %d",
			method,
			path,
			expectedStatus,
			response.StatusCode,
		)
	}

	if response.Header.Get("Cache-Control") != "no-store" {
		t.Error("task response must use Cache-Control: no-store")
	}

	return raw
}

func decodeTask(t *testing.T, body []byte) storage.Task {
	t.Helper()

	var task storage.Task
	if err := json.Unmarshal(body, &task); err != nil {
		t.Fatal("could not decode task")
	}

	return task
}

func (env *testEnvironment) createTaskFixture(t *testing.T, token, title string) storage.Task {
	t.Helper()
	body, err := json.Marshal(map[string]string{
		"title": title, "description": "Preserve this description",
	})
	if err != nil {
		t.Fatal(err)
	}
	return decodeTask(t, env.taskRequest(t, http.MethodPost, "/tasks", token, string(body), http.StatusCreated))
}

func TestTaskIntegration(t *testing.T) {
	env := newTestEnvironment(t)

	t.Run("create read and partial update", func(t *testing.T) {
		body := env.taskRequest(
			t,
			http.MethodPost,
			"/tasks",
			aliceToken,
			`{"title":"  First task  ","description":"Keep this"}`,
			http.StatusCreated,
		)

		task := decodeTask(t, body)

		if task.Title != "First task" || task.Status != storage.TaskStatusOpen {
			t.Fatal("unexpected initial task fields")
		}
		if task.CreatedAt.IsZero() || task.UpdatedAt.IsZero() {
			t.Fatal("task timestamps must be populated")
		}

		body = env.taskRequest(
			t,
			http.MethodGet,
			"/tasks/"+task.ID,
			aliceToken,
			"",
			http.StatusOK,
		)
		if decodeTask(t, body).ID != task.ID {
			t.Fatal("read returned a different task")
		}

		body = env.taskRequest(
			t,
			http.MethodPatch,
			"/tasks/"+task.ID,
			aliceToken,
			`{"title":"Updated"}`,
			http.StatusOK,
		)
		updated := decodeTask(t, body)

		if updated.Title != "Updated" ||
			updated.Description != "Keep this" ||
			updated.Status != storage.TaskStatusOpen {
			t.Fatal("title patch changed unrelated fields")
		}

		body = env.taskRequest(
			t,
			http.MethodPatch,
			"/tasks/"+task.ID,
			aliceToken,
			`{"description":"","status":"COMPLETED"}`,
			http.StatusOK,
		)
		updated = decodeTask(t, body)

		if updated.Description != "" ||
			updated.Status != storage.TaskStatusCompleted ||
			updated.Title != "Updated" {
			t.Fatal("description/status patch produced unexpected fields")
		}

		env.taskRequest(
			t,
			http.MethodGet,
			"/tasks/"+task.ID,
			bobToken,
			"",
			http.StatusNotFound,
		)

		env.taskRequest(
			t,
			http.MethodPatch,
			"/tasks/"+task.ID,
			bobToken,
			`{"title":"Unauthorized"}`,
			http.StatusNotFound,
		)

		body = env.taskRequest(
			t,
			http.MethodGet,
			"/tasks/"+task.ID,
			aliceToken,
			"",
			http.StatusOK,
		)
		if decodeTask(t, body).Title != "Updated" {
			t.Fatal("another owner's request changed the task")
		}
	})

	t.Run("invalid create requests", func(t *testing.T) {
		cases := []string{
			`{}`,
			`{"title":"   "}`,
			`{"title":123}`,
			`{"title":"Task","owner_id":"another-owner"}`,
			`{"title":"Task","status":"COMPLETED"}`,
			`{"title":"Task"} {"title":"Another"}`,
			`null`,
			`[]`,
		}

		for _, body := range cases {
			env.taskRequest(
				t,
				http.MethodPost,
				"/tasks",
				aliceToken,
				body,
				http.StatusBadRequest,
			)
		}
	})

	t.Run("invalid pagination", func(t *testing.T) {
		paths := []string{
			"/tasks?limit=0",
			"/tasks?limit=101",
			"/tasks?limit=abc",
			"/tasks?offset=-1",
			"/tasks?offset=abc",
		}

		for _, path := range paths {
			env.taskRequest(
				t,
				http.MethodGet,
				path,
				aliceToken,
				"",
				http.StatusBadRequest,
			)
		}
	})

	t.Run("task routes require authentication", func(t *testing.T) {
		id := uuid.NewString()
		for _, tc := range []struct{ method, path, body string }{
			{http.MethodGet, "/tasks", ""},
			{http.MethodPost, "/tasks", `{"title":"Unauthorized"}`},
			{http.MethodGet, "/tasks/" + id, ""},
			{http.MethodPatch, "/tasks/" + id, `{"title":"Unauthorized"}`},
		} {
			t.Run(tc.method+" "+tc.path, func(t *testing.T) {
				env.taskRequest(t, tc.method, tc.path, "", tc.body, http.StatusUnauthorized)
			})
		}
	})

	t.Run("malformed and unknown task IDs", func(t *testing.T) {
		for _, tc := range []struct {
			name, id string
			status   int
		}{
			{"malformed", "not-a-uuid", http.StatusBadRequest},
			{"unknown", uuid.NewString(), http.StatusNotFound},
		} {
			t.Run(tc.name, func(t *testing.T) {
				env.taskRequest(t, http.MethodGet, "/tasks/"+tc.id, aliceToken, "", tc.status)
				env.taskRequest(t, http.MethodPatch, "/tasks/"+tc.id, aliceToken, `{"title":"Updated"}`, tc.status)
			})
		}
	})

	t.Run("invalid patches leave the task unchanged", func(t *testing.T) {
		task := env.createTaskFixture(t, aliceToken, "Keep this title")
		for _, tc := range []struct{ name, body string }{
			{"empty", `{}`},
			{"null title only", `{"title":null}`},
			{"empty title", `{"title":""}`},
			{"whitespace title", `{"title":" \t "}`},
			{"unsupported status", `{"status":"completed"}`},
			{"unknown field", `{"owner_id":"another-owner"}`},
			{"wrong field type", `{"description":123}`},
			{"long title", `{"title":"` + strings.Repeat("界", 201) + `"}`},
			{"long description", `{"description":"` + strings.Repeat("界", 5001) + `"}`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				env.taskRequest(t, http.MethodPatch, "/tasks/"+task.ID, aliceToken, tc.body, http.StatusBadRequest)
				actual := decodeTask(t, env.taskRequest(t, http.MethodGet, "/tasks/"+task.ID, aliceToken, "", http.StatusOK))
				if actual.Title != task.Title || actual.Description != task.Description ||
					actual.Status != task.Status || !actual.UpdatedAt.Equal(task.UpdatedAt) {
					t.Fatal("rejected patch changed the persisted task")
				}
			})
		}
	})

	t.Run("description patches preserve omitted and null fields", func(t *testing.T) {
		task := env.createTaskFixture(t, aliceToken, "Preserved title")
		for _, body := range []string{
			`{"description":"Description only"}`,
			`{"title":null,"description":"Null title preserves the old value"}`,
			`{"description":""}`,
		} {
			var input struct{ Description string }
			if err := json.Unmarshal([]byte(body), &input); err != nil {
				t.Fatal(err)
			}
			updated := decodeTask(t, env.taskRequest(t, http.MethodPatch, "/tasks/"+task.ID, aliceToken, body, http.StatusOK))
			if updated.Title != task.Title || updated.Status != task.Status || updated.Description != input.Description {
				t.Fatal("description patch did not preserve the other fields")
			}
			persisted := decodeTask(t, env.taskRequest(t, http.MethodGet, "/tasks/"+task.ID, aliceToken, "", http.StatusOK))
			if persisted.Description != input.Description || persisted.Title != task.Title || persisted.Status != task.Status {
				t.Fatal("persisted description patch differs from the response")
			}
		}
	})

	t.Run("listing respects owners and pagination order", func(t *testing.T) {
		env.createTaskFixture(t, aliceToken, "Alice first")
		env.createTaskFixture(t, aliceToken, "Alice second")
		env.createTaskFixture(t, bobToken, "Bob task")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		rows, err := env.db.Query(ctx, `
			SELECT id::text FROM tasks WHERE owner_id = $1
			ORDER BY created_at DESC, id DESC LIMIT 2
		`, identity.AliceID)
		if err != nil {
			t.Fatal(err)
		}
		var expected []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			expected = append(expected, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if len(expected) != 2 {
			t.Fatal("expected at least two Alice fixtures")
		}

		type taskPage struct {
			Tasks  []storage.Task `json:"tasks"`
			Limit  int            `json:"limit"`
			Offset int            `json:"offset"`
		}
		for offset, path := range []string{"/tasks?limit=1&offset=0", "/tasks?limit=1&offset=1"} {
			var page taskPage
			if err := json.Unmarshal(env.taskRequest(t, http.MethodGet, path, aliceToken, "", http.StatusOK), &page); err != nil {
				t.Fatal(err)
			}
			if len(page.Tasks) != 1 || page.Limit != 1 || page.Offset != offset || page.Tasks[0].ID != expected[offset] {
				t.Fatal("pagination did not match the owner's database ordering")
			}
		}
		for _, owner := range []struct{ token, id string }{
			{aliceToken, identity.AliceID}, {bobToken, identity.BobID},
		} {
			var page taskPage
			if err := json.Unmarshal(env.taskRequest(t, http.MethodGet, "/tasks?limit=100", owner.token, "", http.StatusOK), &page); err != nil {
				t.Fatal(err)
			}
			if len(page.Tasks) == 0 {
				t.Fatal("owned fixtures are missing from the list")
			}
			for _, task := range page.Tasks {
				var actualOwner string
				if err := env.db.QueryRow(ctx, `SELECT owner_id::text FROM tasks WHERE id = $1`, task.ID).Scan(&actualOwner); err != nil {
					t.Fatal(err)
				}
				if actualOwner != owner.id {
					t.Fatal("list exposed another owner's task")
				}
			}
		}
	})

	t.Run("unsupported content type", func(t *testing.T) {
		task := env.createTaskFixture(t, aliceToken, "Content type fixture")
		for _, tc := range []struct{ method, path string }{
			{http.MethodPost, "/tasks"},
			{http.MethodPatch, "/tasks/" + task.ID},
		} {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"title":"Rejected"}`))
			request.Header.Set("Authorization", "Bearer "+aliceToken)
			request.Header.Set("Content-Type", "text/plain")
			response, err := env.app.Test(request, fiber.TestConfig{Timeout: 10 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Error string `json:"error"`
			}
			decodeErr := json.NewDecoder(response.Body).Decode(&result)
			response.Body.Close()
			if decodeErr != nil || response.StatusCode != http.StatusUnsupportedMediaType || result.Error == "" {
				t.Fatal("unsupported content type must return a JSON 415 response")
			}
		}
	})
}
