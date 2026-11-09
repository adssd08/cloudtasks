package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"

	"github.com/gofiber/fiber/v3"
)

func decodeTaskJSON(c fiber.Ctx, destination any) error {
	contentType, _, err := mime.ParseMediaType(c.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		return fiber.NewError(fiber.StatusUnsupportedMediaType, "Content-Type must be application/json")
	}

	body := bytes.TrimSpace(c.Body())
	if len(body) == 0 || body[0] != '{' {
		return fiber.NewError(fiber.StatusBadRequest, "body must be a JSON object")
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body or unsupported field")
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fiber.NewError(fiber.StatusBadRequest, "body must contain exactly one JSON object")
	}

	return nil
}
