package utils

import (
	"errors"

	"github.com/gin-gonic/gin"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"github.com/glennprays/whatsapp-gateway/internal/contextkeys"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
)

func MustGetPhoneNumber(c *gin.Context) (string, bool) {
	raw, ok := c.Get(string(contextkeys.PhoneNumber))
	if !ok {
		err := errDomain.NewError(errDomain.ErrUnauthorized, errors.New("phone number not found in context"))
		httpErr := httperror.FromError(err)
		c.JSON(httpErr.Status, httpErr)
		c.Abort()
		return "", false
	}

	phoneNumber, ok := raw.(string)
	if !ok {
		err := errDomain.NewError(errDomain.ErrUnauthorized, errors.New("invalid phone number format"))
		httpErr := httperror.FromError(err)
		c.JSON(httpErr.Status, httpErr)
		c.Abort()
		return "", false
	}

	return phoneNumber, true
}
