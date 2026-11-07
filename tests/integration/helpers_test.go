//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"example.com/cloudtasks/internal/identity"
	"example.com/cloudtasks/internal/platform"
	"example.com/cloudtasks/internal/services"
	"example.com/cloudtasks/internal/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	aliceToken = "integration-alice-token-0000000000000000"
	bobToken   = "integration-bob-token-000000000000000000"
)

type uploadGrant struct {
	ID      string                   `json:"id"`
	Key     string                   `json:"key"`
	Status  string                   `json:"status"`
	Request storage.PresignedRequest `json:"request"`
}

type testEnvironment struct {
	app       *fiber.App
	db        *pgxpool.Pool
	s3Client  *s3.Client
	store     *storage.S3Store
	presigner *storage.S3Presigner
	bucket    string
	keys      []string
}

func newTestEnvironment(t *testing.T) *testEnvironment {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL must be configured")
	}

	dbConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}

	if dbConfig.ConnConfig.Database != "cloudtasks_test" {
		t.Fatal("integration tests required the cloudtasks_test database")
	}

	if dbConfig.ConnConfig.Host != "localhost" && dbConfig.ConnConfig.Host != "127.0.0.1" {
		t.Fatal("integration tests require a local PostgreSQL host")
	}

	endpoint := os.Getenv("AWS_ENDPOINT_URL")
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil ||
		parsedEndpoint.Scheme != "http" ||
		(parsedEndpoint.Hostname() != "localhost" &&
			parsedEndpoint.Hostname() != "127.0.0.1") {
		t.Fatal("AWS_ENDPOINT_URL must be an explicit local HTTP endpoint")
	}

	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_SECURITY_TOKEN", "")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_DEFAULT_PROFILE", "")
	t.Setenv("AWS_CONFIG_FILE", "/dev/null")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/dev/null")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := platform.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatal("could not connect to the test database")
	}
	t.Cleanup(db.Close)

	clients, err := platform.NewAWSClients(ctx)
	if err != nil {
		t.Fatal("could not configure AWS clients")
	}

	bucket := "cloudtasks-test-" + uuid.NewString()

	_, err = clients.S3.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})

	if err != nil {
		t.Fatal("could not create the integration bucket")
	}

	env := &testEnvironment{
		db:        db,
		s3Client:  clients.S3,
		store:     storage.NewS3Store(clients.S3, bucket),
		presigner: storage.NewS3Presigner(clients.S3, bucket),
		bucket:    bucket,
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		for _, key := range env.keys {

			if err := env.store.Delete(cleanupCtx, key); err != nil {
				t.Errorf("could not clean test object %q", key)
				continue
			}

			_, err := db.Exec(cleanupCtx, `
				DELETE FROM attachments WHERE object_key = $1
			`, key)

			if err != nil {
				t.Errorf("could not clean attachment record for %q", key)
			}
		}

		_, err := clients.S3.DeleteBucket(cleanupCtx, &s3.DeleteBucketInput{
			Bucket: aws.String(bucket),
		})

		if err != nil {
			t.Errorf("could not remove test bucket %q", bucket)
		}

	})

	auth, err := identity.NewDevelopmentAuth("local", aliceToken, bobToken)

	if err != nil {
		t.Fatal(err)
	}

	repository := storage.NewAttachmentRepository(db)
	service := services.NewAttachmentService(repository, env.store, env.presigner)
	env.app = platform.NewHTTPApp(service, auth)

	return env
}

func (env *testEnvironment) apiRequest(t *testing.T, method string, path string, token string, expectedStatus int) []byte {
	t.Helper()

	request := httptest.NewRequest(method, path, nil)

	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := env.app.Test(request, fiber.TestConfig{
		Timeout: 10 * time.Second,
	})

	if err != nil {
		t.Fatal("application request failed")
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal("could not read application response")
	}

	if response.StatusCode != expectedStatus {
		t.Fatalf("%s %s: expected status %d, got %d", method, path, expectedStatus, response.StatusCode)
	}

	if strings.HasPrefix(path, "/attachments") && response.Header.Get("Cache-Control") != "no-store" {
		t.Error("attachment response must use Cache-control: no-store")
	}

	return body

}

func (env *testEnvironment) createGrant(t *testing.T) uploadGrant {
	t.Helper()

	body := env.apiRequest(t, http.MethodPost, "/attachments/presign-upload", aliceToken, http.StatusOK)

	var grant uploadGrant

	if err := json.Unmarshal(body, &grant); err != nil {
		t.Fatal("could not decode upload grant")
	}

	if grant.Key != "" {
		env.keys = append(env.keys, grant.Key)
	}

	if grant.ID == "" || grant.Key == "" || grant.Request.URL == "" {
		t.Fatal("upload grant is incomplete")
	}

	if grant.Status != "PENDING" {
		t.Fatalf("expected PENDING, got %q", grant.Status)
	}

	return grant
}

func (env *testEnvironment) assertState(t *testing.T, id string, expected string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var actual string

	err := env.db.QueryRow(ctx, `
		SELECT status
		FROM attachments
		WHERE id = $1
	`, id).Scan(&actual)
	if err != nil {
		t.Fatal("could not read attachment state")
	}

	if actual != expected {
		t.Fatalf("expected state %q, got %q", expected, actual)
	}
}

func performSignedRequest(t *testing.T, request storage.PresignedRequest, payload []byte, expectedStatus int) []byte {
	t.Helper()

	httpRequest, err := http.NewRequest(request.Method, request.URL, bytes.NewReader(payload))

	if err != nil {
		t.Fatal("could not construct signed request")
	}

	for name, values := range request.Headers {
		if strings.EqualFold(name, "Host") {
			if len(values) > 0 {
				httpRequest.Host = values[0]
			}
			continue
		}

		for _, value := range values {
			httpRequest.Header.Add(name, value)
		}
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	response, err := client.Do(httpRequest)

	if err != nil {
		t.Fatal("signed storage request failed")
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal("could not read storage response")
	}

	if response.StatusCode != expectedStatus {
		t.Fatalf("signed %s: expected staus %d, got %d", request.Method, expectedStatus, response.StatusCode)
	}

	return body
}
