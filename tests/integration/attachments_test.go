//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"example.com/cloudtasks/internal/services"
	"example.com/cloudtasks/internal/storage"
	"github.com/google/uuid"
)

func TestAttachmentIntegration(t *testing.T) {
	env := newTestEnvironment(t)

	t.Run("authentication required", func(t *testing.T) {
		env.apiRequest(t, http.MethodPost, "/attachments/presign-upload", "", http.StatusUnauthorized)
	})

	t.Run("unknown token reject", func(t *testing.T) {
		env.apiRequest(t, http.MethodPost, "/attachments/presign-upload", "unknown-token", http.StatusUnauthorized)
	})

	t.Run("new attachment is pending", func(t *testing.T) {
		grant := env.createGrant(t)

		env.apiRequest(t, http.MethodGet, "/attachments/"+grant.ID+"/presign-download", aliceToken, http.StatusConflict)
	})

	t.Run("upload confirm download and delete", func(t *testing.T) {
		grant := env.createGrant(t)

		payload, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jWZkAAAAASUVORK5CYII=")

		if err != nil {
			t.Fatal("could not decode PNG fixture")
		}

		performSignedRequest(t, grant.Request, payload, http.StatusOK)

		confirmation := env.apiRequest(t, http.MethodPost, "/attachments/"+grant.ID+"/confirm", aliceToken, http.StatusOK)

		var result struct {
			Status string `json:"status"`
		}

		if err := json.Unmarshal(confirmation, &result); err != nil {
			t.Fatal("could not decode confirmation")
		}

		if result.Status != "READY" {
			t.Fatalf("expected READY, got %q", result.Status)
		}

		env.assertState(t, grant.ID, "READY")

		env.apiRequest(t, http.MethodPost, "/attachments/"+grant.ID+"/confirm", aliceToken, http.StatusOK)

		performSignedRequest(t, grant.Request, []byte("attempted replacement"), http.StatusPreconditionFailed)

		body := env.apiRequest(t, http.MethodGet, "/attachments/"+grant.ID+"/presign-download", aliceToken, http.StatusOK)

		var download struct {
			Request storage.PresignedRequest `json:"request"`
		}

		if err := json.Unmarshal(body, &download); err != nil {
			t.Fatal("could not decode download grant")
		}

		actual := performSignedRequest(t, download.Request, nil, http.StatusOK)

		if !bytes.Equal(payload, actual) {
			t.Fatal("downloaded bytes differ from the uploaded fixtures")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var size int64
		var contentType string

		err = env.db.QueryRow(ctx, `
			SELECT size_bytes, content_type
			FROM attachments
			WHERE id = $1
		`, grant.ID).Scan(&size, &contentType)

		if err != nil {
			t.Fatal("could not read confirmed metadata")
		}

		if size != int64(len(payload)) || contentType != "image/png" {
			t.Fatalf("unexpected metadata: size: %d content_type=%q", size, contentType)
		}

		if err := env.store.Delete(ctx, grant.Key); err != nil {
			t.Fatal("could not delete the test object")
		}

		objectBody, err := env.store.Get(ctx, grant.Key)
		if objectBody != nil {
			objectBody.Close()
		}

		if !errors.Is(err, storage.ErrObjectNotFound) {
			t.Fatal("expected object-not-found after deletion")
		}
	})

	t.Run("another owner cannot confirm or download", func(t *testing.T) {
		grant := env.createGrant(t)

		performSignedRequest(t, grant.Request, []byte("Alice owns this attachment\n"), http.StatusOK)

		env.apiRequest(t, http.MethodPost, "/attachments/"+grant.ID+"/confirm", aliceToken, http.StatusOK)

		env.apiRequest(t, http.MethodPost, "/attachments/"+grant.ID+"/confirm", bobToken, http.StatusNotFound)

		env.apiRequest(t, http.MethodGet, "/attachments/"+grant.ID+"/presign-download", bobToken, http.StatusNotFound)

		env.assertState(t, grant.ID, "READY")
	})

	t.Run("malformed attachment ID", func(t *testing.T) {
		env.apiRequest(t, http.MethodPost, "/attachments/not-a-uuid/confirm", aliceToken, http.StatusBadRequest)

		env.apiRequest(t, http.MethodGet, "/attachments/not-a-uuid/presign-download", aliceToken, http.StatusBadRequest)
	})

	t.Run("unknown attachment record", func(t *testing.T) {
		id := uuid.NewString()

		env.apiRequest(t, http.MethodPost, "/attachments/"+id+"/confirm", aliceToken, http.StatusNotFound)

		env.apiRequest(t, http.MethodGet, "/attachments/"+id+"/presign-download", aliceToken, http.StatusNotFound)
	})

	t.Run("confirmation requires and uploaded object", func(t *testing.T) {
		grant := env.createGrant(t)

		env.apiRequest(t, http.MethodPost, "/attachments/"+grant.ID+"/confirm", aliceToken, http.StatusConflict)

		env.assertState(t, grant.ID, "PENDING")
	})

	t.Run("invalid files remain pending", func(t *testing.T) {
		cases := []struct {
			name    string
			payload []byte
		}{
			{
				name:    "empty",
				payload: []byte{},
			},
			{
				name:    "unsupported binary",
				payload: []byte{1, 2, 127, 128, 255},
			},
			{
				name:    "oversized",
				payload: bytes.Repeat([]byte("x"), int(services.MaxAttachmentBytes+1)),
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				grant := env.createGrant(t)

				performSignedRequest(t, grant.Request, tc.payload, http.StatusOK)

				env.apiRequest(t, http.MethodPost, "/attachments/"+grant.ID+"/confirm", aliceToken, http.StatusUnprocessableEntity)

				env.assertState(t, grant.ID, "PENDING")

				env.apiRequest(t, http.MethodGet, "/attachments/"+grant.ID+"/presign-download", aliceToken, http.StatusConflict)
			})
		}
	})

	t.Run("expired download URL is rejected", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		key := "expiry/" + uuid.NewString()
		env.keys = append(env.keys, key)

		payload := []byte("temporary downlaod\n")

		if err := env.store.Put(ctx, key, bytes.NewReader(payload), "text/plain"); err != nil {
			t.Fatal("could not create expiry fixture")
		}

		request, err := env.presigner.PresignDownload(ctx, key, time.Second)

		if err != nil {
			t.Fatal("could not sign short-lived download")
		}

		actual := performSignedRequest(t, request, nil, http.StatusOK)

		if !bytes.Equal(payload, actual) {
			t.Fatal("fresh URL did not return the expected fixture")
		}

		time.Sleep(2 * time.Second)

		performSignedRequest(t, request, nil, http.StatusForbidden)
	})
}
