package identity

import (
	"crypto/subtle"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
)

type ownerContextKey struct{}

const (
	AliceID = "11111111-1111-4111-8111-111111111111"
	BobID   = "22222222-2222-4222-8222-222222222222"
)

func NewDevelopmentAuth(appEnv string, aliceToken string, bobToken string) (fiber.Handler, error) {
	if appEnv != "local" {
		return nil, fmt.Errorf("development authentication requires APP_ENV=local")
	}

	if len(aliceToken) < 32 || len(bobToken) < 32 {
		return nil, fmt.Errorf("development tokens must contain at least 32 characters")
	}

	if aliceToken == bobToken {
		return nil, fmt.Errorf("development tokens must be different")
	}

	return func(c fiber.Ctx) error {
		parts := strings.Fields(c.Get("Authorization"))

		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return fiber.ErrUnauthorized
		}

		token := []byte(parts[1])

		aliceMatch := subtle.ConstantTimeCompare(token, []byte(aliceToken))
		bobMatch := subtle.ConstantTimeCompare(token, []byte(bobToken))

		var ownerID string

		switch {
		case aliceMatch == 1:
			ownerID = AliceID
		case bobMatch == 1:
			ownerID = BobID
		default:
			return fiber.ErrUnauthorized
		}

		c.Locals(ownerContextKey{}, ownerID)
		return c.Next()
	}, nil
}

func OwnerID(c fiber.Ctx) (string, bool) {
	ownerID, ok := c.Locals(ownerContextKey{}).(string)
	return ownerID, ok && ownerID != ""
}
