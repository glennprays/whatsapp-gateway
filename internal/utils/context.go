package utils

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"github.com/glennprays/whatsapp-gateway/internal/contextkeys"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
)

func MustGetPhoneNumber(c *fiber.Ctx) (string, bool) {
	raw := c.Locals(string(contextkeys.PhoneNumber))
	if raw == nil {
		err := errDomain.NewError(errDomain.ErrUnauthorized, errors.New("phone number not found in context"))
		httpErr := httperror.FromError(err)
		c.Status(httpErr.Status).JSON(httpErr)
		return "", false
	}

	phoneNumber, ok := raw.(string)
	if !ok {
		err := errDomain.NewError(errDomain.ErrUnauthorized, errors.New("invalid phone number format"))
		httpErr := httperror.FromError(err)
		c.Status(httpErr.Status).JSON(httpErr)
		return "", false
	}

	return phoneNumber, true
}
